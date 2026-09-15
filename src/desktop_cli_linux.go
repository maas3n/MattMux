//go:build linux

package main

import "context"

func desktopCLITools(ctx context.Context, media bool) (toolPaths, error) {
	return ensureTools(ctx, media, cliStatus)
}
func desktopCLIMetadata(ctx context.Context, src string, title titleInfo, tools toolPaths) (string, error) {
	return metadataText(ctx, src, title, tools, true)
}
func desktopCLIRemux(ctx context.Context, src string, title titleInfo, out string, chapters bool, indexes []int, tools toolPaths) (string, error) {
	return remuxTitle(ctx, src, title, out, chapters, indexes, tools, cliStatus)
}
