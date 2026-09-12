//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeBatchMovie(t *testing.T, root, name string) string {
	t.Helper()
	movie := filepath.Join(root, name)
	videoTS := filepath.Join(movie, "VIDEO_TS")
	if err := os.MkdirAll(videoTS, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(videoTS, "VIDEO_TS.IFO"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	return movie
}

func TestDiscoverBatchMoviesFindsImmediateMovieFolders(t *testing.T) {
	root := t.TempDir()
	makeBatchMovie(t, root, "Movie B")
	makeBatchMovie(t, root, "Movie A")
	if err := os.Mkdir(filepath.Join(root, "Not a DVD"), 0755); err != nil {
		t.Fatal(err)
	}
	movies, err := discoverBatchMovies(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 2 {
		t.Fatalf("movies = %d; want 2", len(movies))
	}
	if movies[0].Name != "Movie A" || movies[1].Name != "Movie B" {
		t.Fatalf("unexpected order: %#v", movies)
	}
}

func TestRunBatchSelectsLongestAndDefaultsOutputToMovieFolder(t *testing.T) {
	root := t.TempDir()
	movieA := makeBatchMovie(t, root, "Movie A")
	movieB := makeBatchMovie(t, root, "Movie B")
	var logBuf bytes.Buffer
	var remuxed []string
	deps := batchDeps{
		ensureTools: func(context.Context, batchProgressFunc) (toolPaths, error) {
			return toolPaths{ffmpeg: "ffmpeg", ffprobe: "ffprobe"}, nil
		},
		scanTitles: func(_ context.Context, src string, _ toolPaths, _ batchProgressFunc) ([]titleInfo, error) {
			return []titleInfo{{Number: 1, Duration: time.Minute}, {Number: 7, Duration: 2 * time.Hour}}, nil
		},
		remuxTitle: func(_ context.Context, src string, title titleInfo, outDir string, _ toolPaths, _ batchProgressFunc) (string, error) {
			if title.Number != 7 {
				t.Fatalf("selected title = %d; want longest title 7", title.Number)
			}
			if outDir != src {
				t.Fatalf("output dir = %q; want movie folder %q", outDir, src)
			}
			// Simulate the ordinary DVD remux naming rule for title > 1. BATCH
			// must normalize this to the movie-folder name after the remux.
			out := filepath.Join(outDir, filepath.Base(src)+"-title-07.mkv")
			if err := os.WriteFile(out, []byte("mkv"), 0644); err != nil {
				t.Fatal(err)
			}
			remuxed = append(remuxed, out)
			return out, nil
		},
	}
	result, err := runBatchWithDeps(context.Background(), batchOptions{InputRoot: root, Log: &logBuf}, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || result.Completed != 2 || len(result.Outputs) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(remuxed) != 2 || !strings.Contains(logBuf.String(), "selected longest title 7") {
		t.Fatalf("batch did not remux/log as expected: remuxed=%v log=%q", remuxed, logBuf.String())
	}
	if result.Outputs[0] != filepath.Join(movieA, "Movie A.mkv") || result.Outputs[1] != filepath.Join(movieB, "Movie B.mkv") {
		t.Fatalf("outputs = %#v", result.Outputs)
	}
	for _, output := range result.Outputs {
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("normalized output missing: %s: %v", output, err)
		}
	}
}

func TestRunBatchUsesChosenOutputRoot(t *testing.T) {
	root := t.TempDir()
	makeBatchMovie(t, root, "Movie")
	outRoot := filepath.Join(t.TempDir(), "finished")
	if err := os.MkdirAll(outRoot, 0755); err != nil {
		t.Fatal(err)
	}
	deps := batchDeps{
		ensureTools: func(context.Context, batchProgressFunc) (toolPaths, error) { return toolPaths{}, nil },
		scanTitles: func(context.Context, string, toolPaths, batchProgressFunc) ([]titleInfo, error) {
			return []titleInfo{{Number: 2, Duration: time.Hour}}, nil
		},
		remuxTitle: func(_ context.Context, _ string, _ titleInfo, outDir string, _ toolPaths, _ batchProgressFunc) (string, error) {
			if outDir != outRoot {
				t.Fatalf("output dir = %q; want %q", outDir, outRoot)
			}
			out := filepath.Join(outDir, "Movie-title-02.mkv")
			if err := os.WriteFile(out, []byte("mkv"), 0644); err != nil {
				t.Fatal(err)
			}
			return out, nil
		},
	}
	result, err := runBatchWithDeps(context.Background(), batchOptions{InputRoot: root, OutputRoot: outRoot}, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 {
		t.Fatalf("completed = %d; want 1", result.Completed)
	}
	want := filepath.Join(outRoot, "Movie.mkv")
	if len(result.Outputs) != 1 || result.Outputs[0] != want {
		t.Fatalf("outputs = %#v; want %q", result.Outputs, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("normalized common-root output missing: %v", err)
	}
}
