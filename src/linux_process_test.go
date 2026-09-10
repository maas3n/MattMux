//go:build linux

package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunCommandKeepsStderrOutOfStdout(t *testing.T) {
	out, err := runCommand(context.Background(), "/bin/sh", "-c", `printf '{"streams":[]}' ; printf '| libdvdnav diagnostic\n' >&2`)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}
	if got, want := string(out), `{"streams":[]}`; got != want {
		t.Fatalf("stdout contaminated by stderr: got %q want %q", got, want)
	}
}

func TestRunCommandReportsStderrOnFailure(t *testing.T) {
	out, err := runCommand(context.Background(), "/bin/sh", "-c", `printf 'partial-output'; printf 'useful failure detail\n' >&2; exit 7`)
	if err == nil {
		t.Fatal("expected command failure")
	}
	if got, want := string(out), "partial-output"; got != want {
		t.Fatalf("unexpected stdout: got %q want %q", got, want)
	}
	if !strings.Contains(err.Error(), "useful failure detail") {
		t.Fatalf("stderr detail missing from error: %v", err)
	}
}
