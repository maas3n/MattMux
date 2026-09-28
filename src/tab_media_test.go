//go:build windows || linux

package main

import (
	"bytes"
	"context"
	"encoding/hex"
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
	packetTimes := func(path string) string {
		t.Helper()
		b, e := exec.Command(ffprobe, "-v", "error", "-select_streams", "s", "-show_packets", "-show_entries", "packet=pts_time,data_hash", "-show_data_hash", "sha256", "-of", "json", path).Output()
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	if packetTimes(source) != packetTimes(filepath.Join(output, "track-02.srt")) {
		t.Fatalf("subtitle timing/payload changed: %s -> %s", packetTimes(source), packetTimes(filepath.Join(output, "track-02.srt")))
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

func TestDVDSubtitleExport(t *testing.T) {
	ffmpeg, e := exec.LookPath("ffmpeg")
	if e != nil {
		t.Skip("ffmpeg unavailable")
	}
	ffprobe, e := exec.LookPath("ffprobe")
	if e != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	idx := filepath.Join(dir, "input.idx")
	sub := filepath.Join(dir, "input.sub")
	// Small synthetic DVD SPU packets, in ordinary VobSub packs (no IFO parsing).
	spu, _ := hex.DecodeString("001e000611110000000603012304fff005000001000001060004000501ff")
	var packs []byte
	for _, ms := range []int64{200, 600} {
		pts := ms * 90
		stamp := []byte{0x21 | byte((pts>>29)&14), byte(pts >> 22), byte((pts>>14)&254) | 1, byte(pts >> 7), byte((pts<<1)&254) | 1}
		data := append([]byte{0x80, 0x80, 5}, stamp...)
		data = append(data, 0x20)
		data = append(data, spu...)
		pack, _ := hex.DecodeString("000001ba4400040004010189c3f8")
		pack = append(pack, 0, 0, 1, 0xbd, byte(len(data)>>8), byte(len(data)))
		pack = append(pack, data...)
		padding := 2048 - len(pack) - 6
		pack = append(pack, 0, 0, 1, 0xbe, byte(padding>>8), byte(padding))
		pack = append(pack, bytes.Repeat([]byte{255}, padding)...)
		packs = append(packs, pack...)
	}
	os.WriteFile(sub, packs, 0600)
	palette := "size: 720x576\npalette: 000000, ffffff, ff0000, 00ff00, 0000ff, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000\n"
	os.WriteFile(idx, []byte("# VobSub index file, v7 (do not modify this line!)\n"+palette+"id: en, index: 0\ntimestamp: 00:00:00:200, filepos: 000000000\ntimestamp: 00:00:00:600, filepos: 000000800\n"), 0600)
	source := filepath.Join(dir, "dvd.mkv")
	b, e := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size=32x32:rate=25:duration=1", "-i", idx, "-map", "0:v", "-map", "1:s", "-c:v", "mpeg2video", "-bf", "0", "-c:s", "copy", source).CombinedOutput()
	if e != nil {
		t.Fatalf("fixture: %v %s", e, b)
	}
	tools := toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}
	for _, format := range []string{"mpeg2", "vob"} {
		out, e := demuxTab(context.Background(), tools, source, 1, dir, nil, false, format)
		if e != nil {
			t.Fatal(e)
		}
		b, e = os.ReadFile(filepath.Join(out, "track-01.idx"))
		if e != nil || !strings.Contains(string(b), palette) || !strings.Contains(string(b), "00:00:00:200") || !strings.Contains(string(b), "00:00:00:600") {
			t.Fatalf("IDX timing/palette: %v %s", e, b)
		}
		hash := func(path string) string {
			t.Helper()
			b, e := exec.Command(ffprobe, "-v", "error", "-select_streams", "s", "-show_packets", "-show_entries", "packet=pts_time,data_hash", "-show_data_hash", "sha256", "-of", "json", path).Output()
			if e != nil {
				t.Fatal(e)
			}
			return string(b)
		}
		if hash(idx) != hash(filepath.Join(out, "track-01.idx")) {
			t.Fatal("DVD subtitle payload/timing changed")
		}
		ext := "mpeg2"
		if format == "vob" {
			ext = "VOB"
		}
		if _, e = exec.Command(ffprobe, "-v", "error", filepath.Join(out, "track-00."+ext)).CombinedOutput(); e != nil {
			t.Fatal(e)
		}
	}
}
