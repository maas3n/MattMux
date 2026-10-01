//go:build windows || linux

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestDesktopMediaInputArgs(t *testing.T) {
	got := appendDesktopMediaInput([]string{"-y"}, "input.mkv")
	want := []string{"-y", "-i", "input.mkv"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
}

func TestDesktopDVDInputGeneratesMissingPTS(t *testing.T) {
	got := appendDesktopDVDInput([]string{"-hide_banner", "-nostdin", "-y"}, 7, "/dvd")
	want := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", "7", "-i", "/dvd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
	if strings.Contains(strings.Join(got, " "), "-preindex") {
		t.Fatalf("DVD remux input unexpectedly pre-indexes: %q", got)
	}
}

func TestMergerDVDOptionsStayWithTheirInput(t *testing.T) {
	for _, dvd := range []string{"disc.iso", "/movies/VIDEO_TS"} {
		for _, dvdFirst := range []bool{false, true} {
			streams := []mergerStream{
				{"movie.mkv", trackOption{Index: 0, Kind: "video"}},
				{"movie.mkv", trackOption{Index: 2, Kind: "subtitle"}},
				{dvd, trackOption{DVDTitle: 2, Index: 1, Kind: "audio"}},
				{"extra.ac3", trackOption{Index: 0, Kind: "audio"}},
			}
			if dvdFirst {
				streams[0], streams[2] = streams[2], streams[0]
			}
			args, err := mergerArgs(streams, "chapters.txt", "output.mkv")
			if err != nil {
				t.Fatal(err)
			}
			assertDVDInputOptions(t, args, dvd)
		}
	}
}

func assertDVDInputOptions(t *testing.T, args []string, dvd string) {
	t.Helper()
	var options []string
	seenDVD := false
	for i := 0; i < len(args); i++ {
		if args[i] != "-i" {
			options = append(options, args[i])
			continue
		}
		i++
		if i >= len(args) {
			t.Fatal("missing input path")
		}
		joined := strings.Join(options, " ")
		if args[i] == dvd {
			seenDVD = true
			for _, flag := range []string{"-analyzeduration 100M", "-probesize 100M", "-fflags +genpts"} {
				if !strings.Contains(joined, flag) {
					t.Fatalf("DVD input missing %s: %q", flag, args)
				}
			}
		} else {
			for _, flag := range []string{"-analyzeduration", "-probesize", "-fflags", "-safe"} {
				if strings.Contains(joined, flag) {
					t.Fatalf("ordinary input %s inherited %s: %q", args[i], flag, args)
				}
			}
		}
		options = nil
	}
	if dvd != "" && !seenDVD {
		t.Fatalf("DVD input missing: %q", args)
	}
}

func TestOrdinaryMergerAndTabInputsHaveNoDVDOptions(t *testing.T) {
	var streams []mergerStream
	for _, path := range []string{"movie.mkv", "movie.mp4", "video.h264", "video.mpeg2", "audio.ac3", "captions.srt"} {
		kind := "video"
		if strings.HasSuffix(path, ".ac3") {
			kind = "audio"
		}
		if strings.HasSuffix(path, ".srt") {
			kind = "subtitle"
		}
		streams = append(streams, mergerStream{path, trackOption{Index: 0, Kind: kind}})
	}
	args, err := mergerArgs(streams, "chapters.txt", "output.mkv")
	if err != nil {
		t.Fatal(err)
	}
	assertDVDInputOptions(t, args, "")
	assertDVDInputOptions(t, tabInput(nil, "movie.mkv", 0), "")
	for _, dvd := range []string{"disc.iso", "/movies/VIDEO_TS"} {
		assertDVDInputOptions(t, tabInput(nil, dvd, 1), dvd)
	}
}
