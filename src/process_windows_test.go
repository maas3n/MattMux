//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHiddenKeepsStdoutAndStderrSeparate(t *testing.T) {
	script := filepath.Join(t.TempDir(), "mixed-output.cmd")
	body := "@echo off\r\necho {\"streams\":[]}\r\necho dvdnav diagnostic 1>&2\r\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := runHidden(context.Background(), "cmd.exe", "/D", "/C", script)
	if err != nil {
		t.Fatalf("runHidden returned error: %v", err)
	}
	if strings.Contains(string(out), "dvdnav diagnostic") {
		t.Fatalf("stderr leaked into stdout: %q", out)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("stdout was not clean JSON: %v; output=%q", err, out)
	}
}

func TestRunHiddenReportsStderrOnFailure(t *testing.T) {
	script := filepath.Join(t.TempDir(), "failed-output.cmd")
	body := "@echo off\r\necho partial-stdout\r\necho mediainfo failure 1>&2\r\nexit /b 7\r\n"
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := runHidden(context.Background(), "cmd.exe", "/D", "/C", script)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(string(out), "partial-stdout") {
		t.Fatalf("stdout was lost on failure: %q", out)
	}
	if !strings.Contains(err.Error(), "mediainfo failure") {
		t.Fatalf("stderr missing from error: %v", err)
	}
}
