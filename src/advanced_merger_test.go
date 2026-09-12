//go:build windows || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMergerExplicitMaps(t *testing.T) {
	streams := []mergerStream{{"one.mkv", trackOption{Index: 2, Kind: "video"}}, {"two.mp4", trackOption{Index: 1, Kind: "audio"}}, {"one.mkv", trackOption{Index: 5, Kind: "subtitle"}}}
	args, err := mergerArgs(streams, "chapters.txt", "out.mkv")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "one.mkv", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "two.mp4", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "chapters.txt", "-map", "0:2", "-map_metadata:s:0", "0:s:2", "-map", "1:1", "-map_metadata:s:1", "1:s:1", "-map", "0:5", "-map_metadata:s:2", "0:s:5", "-map_metadata", "0", "-map_chapters", "2", "-c", "copy", "-f", "matroska", "out.mkv"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q", args)
	}
	if _, err = mergerArgs(nil, "", "out.mkv"); err == nil {
		t.Fatal("empty selection accepted")
	}
	args, _ = mergerArgs(streams, "", "out.mkv")
	if !strings.Contains(strings.Join(args, " "), "-map_chapters -1") {
		t.Fatal("implicit chapters allowed")
	}
}

func TestMergerFFmpegIntegration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "mixed input.mkv")
	sub := filepath.Join(dir, "captions.srt")
	chap := filepath.Join(dir, "chapters.txt")
	os.WriteFile(sub, []byte("1\n00:00:00,000 --> 00:00:00,900\nHello\n"), 0600)
	os.WriteFile(chap, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1000\ntitle=Opening\n"), 0600)
	_, err = runMergerCommand(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size=32x32:rate=25:duration=1", "-f", "lavfi", "-i", "sine=duration=1", "-i", sub, "-f", "ffmetadata", "-i", chap, "-map", "0:v", "-map", "0:v", "-map", "1:a", "-map", "1:a", "-map", "2:s", "-map_chapters", "3", "-c:v", "mpeg4", "-c:a", "pcm_s16le", "-c:s", "srt", "-attach", sub, "-metadata:s:t", "mimetype=text/plain", source)
	if err != nil {
		t.Fatal(err)
	}
	all, err := probeMergerFile(ctx, ffprobe, source, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 7 {
		t.Fatalf("want six streams plus chapters, got %v", all)
	}
	if all[5].Track.Kind != "attachment" || all[6].Track.Kind != "chapters" {
		t.Fatalf("missing attachment/chapter rows: %v", all)
	}
	allOutput := filepath.Join(dir, "all.mkv")
	if err = muxMerger(ctx, toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}, all, "", allOutput); err != nil {
		t.Fatal(err)
	}
	allProbe, err := probeMergerFile(ctx, ffprobe, allOutput, "all")
	if err != nil || len(allProbe) != 7 {
		t.Fatalf("all-stream output: %v %v", allProbe, err)
	}
	if err = validateMergerChapters(ctx, ffprobe, source); err != nil {
		t.Fatal("MKV chapters rejected", err)
	}
	stripped := filepath.Join(dir, "chapter-override.mkv")
	if err = muxMerger(ctx, toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}, all[:1], source, stripped); err != nil {
		t.Fatal(err)
	}
	strippedProbe, err := probeMergerFile(ctx, ffprobe, stripped, "all")
	if err != nil || len(strippedProbe) != 2 || strippedProbe[1].Track.Kind != "chapters" {
		t.Fatalf("chapter override leaked media: %v %v", strippedProbe, err)
	}
	// Movie imports may also contain MP4 chapters; the dedicated chapter picker
	// remains limited to MKV or FFMETADATA1.
	mp4 := filepath.Join(dir, "chapter-movie.mp4")
	if _, err = runMergerCommand(ctx, ffmpeg, "-v", "error", "-i", source, "-map", "0:v:0", "-map_chapters", "0", "-c", "copy", mp4); err != nil {
		t.Fatal(err)
	}
	mp4All, err := probeMergerFile(ctx, ffprobe, mp4, "all")
	if err != nil {
		t.Fatal(err)
	}
	var mp4Media []mergerStream
	for _, row := range mp4All {
		if row.Track.Kind == "video" || row.Track.Kind == "chapters" {
			mp4Media = append(mp4Media, row)
		}
	}
	if len(mp4Media) != 2 {
		t.Fatal("MP4 movie chapters not listed")
	}
	if err = muxMerger(ctx, toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}, mp4Media, "", filepath.Join(dir, "mp4-chapters.mkv")); err != nil {
		t.Fatal(err)
	}
	if err = validateMergerChapters(ctx, ffprobe, mp4); err == nil {
		t.Fatal("standalone chapter picker accepted MP4")
	}
	var selected []mergerStream
	for _, tc := range []struct {
		kind  string
		count int
	}{{"video", 2}, {"audio", 2}, {"subtitle", 1}} {
		streams, err := probeMergerFile(ctx, ffprobe, source, tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(streams) != tc.count {
			t.Fatalf("%s count %d", tc.kind, len(streams))
		}
		for _, s := range streams {
			if s.Track.Kind != tc.kind {
				t.Fatal("category leaked")
			}
		}
		selected = append(selected, streams[len(streams)-1])
	}
	raw := filepath.Join(dir, "external.ac3")
	if _, err = runMergerCommand(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=700:duration=1", "-c:a", "ac3", raw); err != nil {
		t.Fatal(err)
	}
	audio, err := probeMergerFile(ctx, ffprobe, raw, "audio")
	if err != nil {
		t.Fatal(err)
	}
	selected = append(selected, audio...)
	subs, err := probeMergerFile(ctx, ffprobe, sub, "subtitle")
	if err != nil {
		t.Fatal(err)
	}
	selected = append(selected, subs...)
	tools := toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}
	out := filepath.Join(dir, "merged.mkv")
	if err = muxMerger(ctx, tools, selected, chap, out); err != nil {
		t.Fatal(err)
	}
	data, err := runMergerCommand(ctx, ffprobe, "-v", "error", "-show_streams", "-show_chapters", "-of", "json", out)
	if err != nil {
		t.Fatal(err)
	}
	var result ffprobeResult
	json.Unmarshal(data, &result)
	var chapters ffprobeChapterResult
	json.Unmarshal(data, &chapters)
	if len(result.Streams) != 5 || len(chapters.Chapters) != 1 {
		t.Fatalf("unexpected output: %s", data)
	}
	if chapters.Chapters[0].Tags["title"] != "Opening" {
		t.Fatalf("chapter title lost: %s", data)
	}
	want := []string{"mpeg4", "pcm_s16le", "subrip", "ac3", "subrip"}
	for i, s := range result.Streams {
		if s.CodecName != want[i] {
			t.Fatalf("stream %d codec %s", i, s.CodecName)
		}
	}
	before, _ := os.ReadFile(out)
	if err = muxMerger(ctx, tools, selected, "", out); err == nil {
		t.Fatal("overwrote existing output")
	}
	after, _ := os.ReadFile(out)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("existing output changed")
	}
	noChapter := filepath.Join(dir, "no-chapters.mkv")
	if err = muxMerger(ctx, tools, selected, "", noChapter); err != nil {
		t.Fatal(err)
	}
	data, _ = runMergerCommand(ctx, ffprobe, "-v", "error", "-show_chapters", "-of", "json", noChapter)
	var none ffprobeChapterResult
	json.Unmarshal(data, &none)
	if len(none.Chapters) != 0 {
		t.Fatal("source chapters leaked")
	}
	bad := filepath.Join(dir, "invalid.txt")
	os.WriteFile(bad, []byte("not metadata"), 0600)
	if err = validateMergerChapters(ctx, ffprobe, bad); err == nil {
		t.Fatal("invalid chapters accepted")
	}
	cancelled, c := context.WithCancel(ctx)
	c()
	cancelOut := filepath.Join(dir, "cancelled.mkv")
	if err = muxMerger(cancelled, tools, selected, "", cancelOut); err == nil {
		t.Fatal("cancel succeeded")
	}
	if _, err = os.Stat(cancelOut); !os.IsNotExist(err) {
		t.Fatal("cancel left final file")
	}
}

func TestMergerRawVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, tc := range []struct{ ext, codec string }{{"h264", "libx264"}, {"m2v", "mpeg2video"}, {"vob", "mpeg2video"}} {
		t.Run(tc.ext, func(t *testing.T) {
			dir := t.TempDir()
			raw := filepath.Join(dir, "raw."+tc.ext)
			if _, err := runMergerCommand(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size=32x32:rate=25:duration=1", "-c:v", tc.codec, "-bf", "0", raw); err != nil {
				t.Fatal(err)
			}
			streams, err := probeMergerFile(ctx, ffprobe, raw, "video")
			if err != nil {
				t.Fatal(err)
			}
			if err = muxMerger(ctx, toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}, streams, "", filepath.Join(dir, "out.mkv")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMergerChapterSelection(t *testing.T) {
	media := mergerStream{"movie.mkv", trackOption{Index: 0, Kind: "video"}}
	one := mergerStream{"one.mkv", trackOption{Index: -1, Kind: "chapters"}}
	two := mergerStream{"two.mkv", trackOption{Index: -1, Kind: "chapters"}}
	if _, _, err := resolveMergerSelection([]mergerStream{media, one, two}, ""); err == nil {
		t.Fatal("ambiguous chapter sets accepted")
	}
	streams, chapter, err := resolveMergerSelection([]mergerStream{media, one, two}, "override.mkv")
	if err != nil || chapter != "override.mkv" || len(streams) != 1 {
		t.Fatal("override failed")
	}
	if _, _, err = resolveMergerSelection([]mergerStream{one}, ""); err == nil {
		t.Fatal("chapters-only output accepted")
	}
}
