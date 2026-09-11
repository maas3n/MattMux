//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppDataRootPrefersPortableDirectory(t *testing.T) {
	root := t.TempDir()
	portable := filepath.Join(root, "MattMuxData")
	if err := os.MkdirAll(portable, 0755); err != nil {
		t.Fatal(err)
	}

	got := resolveAppDataRootFor(
		filepath.Join(root, "MattMux-Portable.exe"),
		filepath.Join(root, "LocalAppData"),
		filepath.Join(root, "Home"),
	)
	if got != portable {
		t.Fatalf("portable root = %q, want %q", got, portable)
	}
}

func TestResolveAppDataRootFallsBackToLocalAppData(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "LocalAppData")
	want := filepath.Join(local, "MattMux")

	got := resolveAppDataRootFor(
		filepath.Join(root, "MattMux.exe"),
		local,
		filepath.Join(root, "Home"),
	)
	if got != want {
		t.Fatalf("installed root = %q, want %q", got, want)
	}
	if st, err := os.Stat(want); err != nil || !st.IsDir() {
		t.Fatalf("installed data directory was not created: err=%v", err)
	}
}
