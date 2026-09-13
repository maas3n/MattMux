//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

type movie struct {
	name   string
	dir    string
	source string
}

type title struct {
	number   int
	duration time.Duration
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("MattMux CLI %s\n", version)
		return
	}
	if len(os.Args) < 2 || os.Args[1] != "--batch" {
		usage()
		if len(os.Args) > 1 {
			os.Exit(2)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fs := flag.NewFlagSet("--batch", flag.ExitOnError)
	logPath := fs.String("log", "", "optional batch log file")
	fs.Usage = usage
	_ = fs.Parse(os.Args[2:])
	if fs.NArg() < 1 || fs.NArg() > 2 {
		usage()
		os.Exit(2)
	}
	inputRoot := fs.Arg(0)
	outputRoot := ""
	if fs.NArg() == 2 {
		outputRoot = fs.Arg(1)
	}
	var writer io.Writer = io.Discard
	var logFile *os.File
	if strings.TrimSpace(*logPath) != "" {
		abs, err := filepath.Abs(strings.TrimSpace(*logPath))
		fatal(err)
		fatal(os.MkdirAll(filepath.Dir(abs), 0755))
		logFile, err = os.OpenFile(abs, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		fatal(err)
		defer logFile.Close()
		writer = logFile
		fmt.Fprintln(os.Stderr, "Batch log:", abs)
	}
	fatal(runBatch(ctx, inputRoot, outputRoot, writer))
}

func usage() {
	fmt.Fprintf(os.Stderr, `MattMux CLI %s

Usage:
  mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]
  mattmux-cli --version

Batch mode expects MOVIES_ROOT / Movie Title / VIDEO_TS / VIDEO_TS.IFO.
It scans titles through FFmpeg dvdvideo/libdvdread/libdvdnav, chooses the longest
title, includes all streams and chapters, and remuxes with:
  -analyzeduration 100M -probesize 100M -fflags +genpts
When OUTPUT_ROOT is omitted, each MKV is placed in its movie title folder.
`, version)
}

func runBatch(ctx context.Context, root, outputRoot string, logWriter io.Writer) error {
	movies, err := discoverMovies(root)
	if err != nil {
		return err
	}
	ffmpeg, ffprobe, err := findTools(ctx)
	if err != nil {
		return err
	}
	if outputRoot != "" {
		outputRoot, err = filepath.Abs(outputRoot)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(outputRoot, 0755); err != nil {
			return err
		}
	}
	logger := log.New(logWriter, "", log.LstdFlags)
	logger.Printf("MattMux Windows batch start: input=%q output=%q movies=%d", root, outputRoot, len(movies))
	var failures []string
	for i, m := range movies {
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[%d/%d] Scanning %s through dvdvideo/libdvdread/libdvdnav...\n", i+1, len(movies), m.name)
		logger.Printf("[%d/%d] scanning %s", i+1, len(movies), m.name)
		titles := discoverDVDTitlesViaDVDVideo(ctx, ffprobe, m.source)
		if len(titles) == 0 {
			msg := m.name + ": no readable DVD titles"
			failures = append(failures, msg)
			logger.Print(msg)
			continue
		}
		best := titles[0]
		for _, t := range titles[1:] {
			if t.duration > best.duration {
				best = t
			}
		}
		outDir := m.dir
		if outputRoot != "" {
			outDir = outputRoot
		}
		final := filepath.Join(outDir, m.name+".mkv")
		fmt.Fprintf(os.Stderr, "[%d/%d] %s: longest title %d (%s); remuxing all streams...\n", i+1, len(movies), m.name, best.number, best.duration.Round(time.Second))
		logger.Printf("[%d/%d] selected longest title %d (%s) for %s", i+1, len(movies), best.number, best.duration, m.name)
		if err := remux(ctx, ffmpeg, m.source, best.number, final); err != nil {
			msg := fmt.Sprintf("%s: %v", m.name, err)
			failures = append(failures, msg)
			logger.Printf("[%d/%d] failed: %s", i+1, len(movies), msg)
			continue
		}
		fmt.Println(final)
		logger.Printf("[%d/%d] completed %s -> %s", i+1, len(movies), m.name, final)
	}
	if len(failures) > 0 {
		return fmt.Errorf("batch completed with %d failure(s): %s", len(failures), strings.Join(failures, "; "))
	}
	logger.Printf("MattMux Windows batch complete: movies=%d", len(movies))
	return nil
}

func discoverMovies(root string) ([]movie, error) {
	root = strings.TrimSpace(strings.Trim(root, "\""))
	if root == "" {
		return nil, errors.New("missing MOVIES_ROOT")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	var movies []movie
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		movieDir := filepath.Join(abs, entry.Name())
		videoTS := findChildDirFold(movieDir, "VIDEO_TS")
		if videoTS == "" || !fileExistsFold(videoTS, "VIDEO_TS.IFO") {
			continue
		}
		movies = append(movies, movie{name: entry.Name(), dir: movieDir, source: movieDir})
	}
	sort.Slice(movies, func(i, j int) bool { return strings.ToLower(movies[i].name) < strings.ToLower(movies[j].name) })
	if len(movies) == 0 {
		return nil, errors.New("no movie title folders containing VIDEO_TS/VIDEO_TS.IFO were found")
	}
	return movies, nil
}

func findChildDirFold(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func fileExistsFold(dir, name string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), name) {
			return true
		}
	}
	return false
}

func findTools(ctx context.Context) (string, string, error) {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "MattMuxData", "tools", "ffmpeg-2026-09-08"))
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		dirs = append(dirs, filepath.Join(local, "MattMux", "tools", "ffmpeg-2026-09-08"))
	}
	for _, dir := range dirs {
		ffmpeg := filepath.Join(dir, "ffmpeg.exe")
		ffprobe := filepath.Join(dir, "ffprobe.exe")
		if fileExists(ffmpeg) && fileExists(ffprobe) && hasDVDVideo(ctx, ffmpeg) {
			return ffmpeg, ffprobe, nil
		}
	}
	ffmpeg, e1 := exec.LookPath("ffmpeg.exe")
	ffprobe, e2 := exec.LookPath("ffprobe.exe")
	if e1 == nil && e2 == nil && hasDVDVideo(ctx, ffmpeg) {
		return ffmpeg, ffprobe, nil
	}
	return "", "", errors.New("compatible FFmpeg/FFprobe with dvdvideo was not found; install MattMux with bundled tools or keep the portable MattMuxData folder together")
}

func hasDVDVideo(ctx context.Context, ffmpeg string) bool {
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-demuxers").CombinedOutput()
	return err == nil && strings.Contains(string(out), "dvdvideo")
}

func discoverDVDTitlesViaDVDVideo(ctx context.Context, ffprobe, src string) []title {
	var titles []title
	for n := 1; n <= 99; n++ {
		if ctx.Err() != nil {
			break
		}
		d, err := readDVDVideoTitleDuration(ctx, ffprobe, src, n, false)
		if err != nil {
			d, err = readDVDVideoTitleDuration(ctx, ffprobe, src, n, true)
		}
		if err == nil && d > 0 {
			titles = append(titles, title{number: n, duration: d})
		}
	}
	return titles
}

func readDVDVideoTitleDuration(ctx context.Context, ffprobe, src string, n int, preindex bool) (time.Duration, error) {
	args := []string{"-v", "error", "-analyzeduration", "100M", "-probesize", "100M"}
	if preindex {
		args = append(args, "-preindex", "1")
	}
	args = append(args, "-f", "dvdvideo", "-title", strconv.Itoa(n), "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", src)
	out, err := exec.CommandContext(ctx, ffprobe, args...).CombinedOutput()
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(out))
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f <= 0 {
		return 0, errors.New("duration unavailable")
	}
	return time.Duration(f * float64(time.Second)), nil
}

func remux(ctx context.Context, ffmpeg, src string, titleN int, final string) error {
	if err := os.MkdirAll(filepath.Dir(final), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(final); err == nil {
		return fmt.Errorf("output already exists: %s", final)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(final), ".mattmux-*.partial.mkv")
	if err != nil {
		return err
	}
	partial := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(partial)
	defer os.Remove(partial)
	args := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", strconv.Itoa(titleN), "-i", src, "-map", "0", "-c", "copy", "-map_metadata", "0", "-map_chapters", "0", partial}
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	st, err := os.Stat(partial)
	if err != nil || st.Size() == 0 {
		return errors.New("FFmpeg did not create a valid MKV")
	}
	return os.Rename(partial, final)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func fatal(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "mattmux-cli:", err)
	os.Exit(1)
}
