//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeRemuxRejectsEmptyOutput(t *testing.T) {
	root := t.TempDir()
	partial := filepath.Join(root, "partial.mkv")
	final := filepath.Join(root, "final.mkv")
	if err := os.WriteFile(partial, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := finalizeRemuxOutput(partial, final); err == nil {
		t.Fatal("empty output was accepted")
	}
	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Fatalf("unexpected final output: %v", err)
	}
}

func TestFinalizeRemuxPreservesExistingDestination(t *testing.T) {
	root := t.TempDir()
	partial := filepath.Join(root, "partial.mkv")
	final := filepath.Join(root, "final.mkv")
	if err := os.WriteFile(partial, []byte("new output"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(final, []byte("other writer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := finalizeRemuxOutput(partial, final); err == nil {
		t.Fatal("existing destination was replaced")
	}
	b, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "other writer" {
		t.Fatalf("destination changed: %q", b)
	}
}

func TestReservePartialOutputIsUnique(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "movie.mkv")
	a, err := reservePartialOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(a)
	b, err := reservePartialOutput(final)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(b)
	if a == b {
		t.Fatalf("temporary output names collided: %s", a)
	}
}
