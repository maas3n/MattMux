//go:build linux

package main

import (
	"context"
	"os/exec"
)

func mergerTools(ctx context.Context) (toolPaths, error) {
	// Ordinary media does not require the dvdvideo demuxer.
	ffmpeg, e1 := exec.LookPath("ffmpeg")
	ffprobe, e2 := exec.LookPath("ffprobe")
	if e1 == nil && e2 == nil {
		return toolPaths{ffmpeg: ffmpeg, ffprobe: ffprobe}, nil
	}
	return ensureTools(ctx, false, func(float64, string) {})
}

func runMergerCommand(ctx context.Context, path string, args ...string) ([]byte, error) {
	return runCommand(ctx, path, args...)
}
