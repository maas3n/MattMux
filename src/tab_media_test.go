//go:build windows || linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMKVTabRemuxDemux(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg fixture generator unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	ctx := context.Background()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if b, e := exec.Command(ffmpeg, args...).CombinedOutput(); e != nil {
			t.Fatalf("fixture: %v: %s", e, b)
		}
	}
	srt := filepath.Join(dir, "captions.srt")
	os.WriteFile(srt, []byte("1\n00:00:00,200 --> 00:00:00,900\nHello\n"), 0600)
	chapters := filepath.Join(dir, "chapters.txt")
	os.WriteFile(chapters, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1000\ntitle=Opening\n"), 0600)
	source := filepath.Join(dir, "source.mkv")
	run("-v", "error", "-f", "lavfi", "-i", "color=size=32x32:rate=25:duration=1", "-f", "lavfi", "-i", "sine=duration=1", "-i", srt, "-f", "ffmetadata", "-i", chapters, "-map", "0:v", "-map", "1:a", "-map", "2:s", "-map_chapters", "3", "-c:v", "libx264", "-c:a", "ac3", "-c:s", "srt", source)
	tools := toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}
	if mi, e := exec.LookPath("mediainfo"); e == nil {
		tools.mediainfo = mi
		titles, e := scanMKV(ctx, tools, source)
		if e != nil || len(titles) != 1 {
			t.Fatalf("MediaInfo scan: %v %v", titles, e)
		}
		if _, details, e := probeTabMKV(ctx, tools, source); e != nil || !strings.Contains(details, "Matroska") {
			t.Fatalf("MediaInfo metadata: %v %s", e, details)
		}
	}
	probe := func(path string) map[string]json.RawMessage {
		t.Helper()
		b, e := exec.Command(ffprobe, "-v", "error", "-show_streams", "-show_chapters", "-of", "json", path).Output()
		if e != nil {
			t.Fatal(e)
		}
		var p map[string]json.RawMessage
		json.Unmarshal(b, &p)
		return p
	}
	output, e := demuxTab(ctx, tools, source, 1, dir, []int{0, 1, 2}, true, "mpeg2")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"track-00.h264", "track-01.ac3", "track-02.srt", "Chapters.txt"} {
		if st, e := os.Stat(filepath.Join(output, name)); e != nil || st.Size() == 0 {
			t.Fatalf("missing export %s: %v", name, e)
		}
	}
	probe(filepath.Join(output, "track-00.h264"))
	probe(filepath.Join(output, "track-01.ac3"))
	probe(filepath.Join(output, "track-02.srt"))
	b, _ := os.ReadFile(filepath.Join(output, "Chapters.txt"))
	if !strings.Contains(string(b), "CHAPTER01NAME=Opening") {
		t.Fatalf("chapters: %s", b)
	}
	b, _ = os.ReadFile(filepath.Join(output, "track-02.srt"))
	if !strings.Contains(string(b), "00:00:00,206") {
		t.Fatalf("subtitle timing: %s", b)
	}
	remux, e := remuxMKV(ctx, tools, source, dir, []int{1, 2}, false)
	if e != nil {
		t.Fatal(e)
	}
	p := probe(remux)
	var streams []struct {
		Codec string `json:"codec_name"`
	}
	json.Unmarshal(p["streams"], &streams)
	if len(streams) != 2 || streams[0].Codec != "ac3" || streams[1].Codec != "subrip" {
		t.Fatalf("selection: %s", p["streams"])
	}
	if string(p["chapters"]) != "[\n\n    ]" && strings.TrimSpace(string(p["chapters"])) != "[]" {
		var ch []any
		json.Unmarshal(p["chapters"], &ch)
		if len(ch) != 0 {
			t.Fatal("chapters retained despite deselection")
		}
	}
	if _, e = remuxMKV(ctx, tools, source, dir, nil, true); e == nil {
		t.Fatal("overwrote previous remux")
	}
	if _, e = demuxTab(ctx, tools, source, 1, dir, []int{99}, true, "mpeg2"); e == nil {
		t.Fatal("accepted missing stream")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = demuxTab(cancelled, tools, source, 1, dir, nil, true, "mpeg2"); e == nil {
		t.Fatal("ignored cancellation")
	}
}
