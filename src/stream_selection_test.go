//go:build windows || linux

package main

import (
	"reflect"
	"testing"
)

func TestFFmpegStreamMapArgsDefaultsToAll(t *testing.T) {
	got, err := ffmpegStreamMapArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-map", "0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFFmpegStreamMapArgsExplicitSelection(t *testing.T) {
	got, err := ffmpegStreamMapArgs([]int{4, 1, 4})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-map", "0:1", "-map", "0:4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFFmpegStreamMapArgsRejectsEmptySelection(t *testing.T) {
	if _, err := ffmpegStreamMapArgs([]int{}); err == nil {
		t.Fatal("expected empty explicit selection to fail")
	}
}
