//go:build windows || linux

package main

import "strconv"

func appendDesktopMediaInput(args []string, input string) []string {
	// Ordinary files (including staged MKVs) keep their normal input policy.
	return append(args, "-i", input)
}

func appendDesktopDVDInput(args []string, title int, input string) []string {
	return append(args, "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", strconv.Itoa(title), "-i", input)
}
