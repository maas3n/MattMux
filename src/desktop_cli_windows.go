//go:build windows

package main

import (
	"context"
	"encoding/json"
)

func desktopCLITools(ctx context.Context, media bool) (toolPaths, error) {
	return ensureTools(ctx, media)
}
func desktopCLIMetadata(ctx context.Context, src string, title titleInfo, tools toolPaths) (string, error) {
	p, err := probeStreams(ctx, tools.ffprobe, src, title.Number)
	if err != nil {
		return "", err
	}
	c, err := probeChapters(ctx, tools.ffprobe, src, title.Number)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(struct {
		Title    titleInfo
		Streams  ffprobeResult
		Chapters []DVDChapter
	}{title, p, c}, "", "  ")
	return string(b) + "\n", err
}
func desktopCLIRemux(ctx context.Context, src string, title titleInfo, out string, chapters bool, indexes []int, tools toolPaths) (string, error) {
	return remuxDVDWindows(ctx, src, title, out, chapters, indexes, tools, cliStatus)
}
