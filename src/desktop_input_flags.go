//go:build windows || linux

package main

import "strconv"

func appendDesktopRobustInput(args []string, input string) []string {
	return append(args, "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", input)
}

func appendDesktopDVDInput(args []string, title int, input string) []string {
	return append(args, "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", strconv.Itoa(title), "-i", input)
}
