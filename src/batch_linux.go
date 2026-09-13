//go:build linux

package main

import "context"

func batchPlatformDeps() batchDeps {
	return batchDeps{
		ensureTools: func(ctx context.Context, progress batchProgressFunc) (toolPaths, error) {
			return ensureTools(ctx, false, func(frac float64, status string) { progress(frac, status) })
		},
		discoverDVDTitlesViaDVDVideo: func(ctx context.Context, src string, tools toolPaths, progress batchProgressFunc) ([]titleInfo, error) {
			return discoverDVDTitlesViaDVDVideo(ctx, src, tools, func(frac float64, status string) { progress(frac, status) })
		},
		remuxTitle: func(ctx context.Context, src string, title titleInfo, outDir string, tools toolPaths, progress batchProgressFunc) (string, error) {
			// Batch is intentionally one-click: include every stream and preserve chapters.
			// remuxTitle uses appendDesktopDVDInput, so analyzeduration/probesize/genpts
			// are always present on the dvdvideo input.
			return remuxTitle(ctx, src, title, outDir, true, nil, tools, func(frac float64, status string) { progress(frac, status) })
		},
	}
}
