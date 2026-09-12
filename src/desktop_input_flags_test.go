//go:build windows || linux

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestDesktopRobustInputArgs(t *testing.T) {
	got := appendDesktopRobustInput([]string{"-y"}, "input.mkv")
	want := []string{"-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "input.mkv"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
}

func TestDesktopDVDInputStartsImmediatelyWithFixedTimestamps(t *testing.T) {
	got := appendDesktopDVDInput([]string{"-hide_banner", "-nostdin", "-y"}, 7, "/dvd")
	want := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", "7", "-i", "/dvd"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q; want %q", got, want)
	}
	if strings.Contains(strings.Join(got, " "), "-preindex") {
		t.Fatalf("DVD remux input unexpectedly pre-indexes: %q", got)
	}
}
