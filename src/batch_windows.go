//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
)

func batchPlatformDeps() batchDeps {
	return batchDeps{
		ensureTools: func(ctx context.Context, progress batchProgressFunc) (toolPaths, error) {
			progress(0, "Preparing FFmpeg / FFprobe…")
			tools, err := ensureTools(ctx, false)
			if err == nil {
				progress(1, "FFmpeg ready.")
			}
			return tools, err
		},
		discoverDVDTitlesViaDVDVideo: batchScanTitlesWindows,
		remuxTitle:                   batchRemuxTitleWindows,
	}
}

func batchScanTitlesWindows(ctx context.Context, src string, tools toolPaths, progress batchProgressFunc) ([]titleInfo, error) {
	src, err := normalizeSource(src)
	if err != nil {
		return nil, err
	}
	var titles []titleInfo
	for n := 1; n <= 99; n++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(float64(n-1)/99.0, fmt.Sprintf("Scanning DVD title %d of 99 through dvdvideo/libdvdread/libdvdnav…", n))
		d, err := readDVDVideoTitleDuration(ctx, tools.ffprobe, src, n)
		if err != nil {
			continue
		}
		titles = append(titles, titleInfo{Number: n, Duration: d})
	}
	if len(titles) == 0 {
		return nil, errors.New("no readable DVD titles were found")
	}
	sort.Slice(titles, func(i, j int) bool { return titles[i].Number < titles[j].Number })
	progress(1, fmt.Sprintf("Found %d readable title(s).", len(titles)))
	return titles, nil
}

func longestTitle(titles []titleInfo) (titleInfo, error) {
	if len(titles) == 0 {
		return titleInfo{}, errors.New("no titles")
	}
	best := titles[0]
	for _, title := range titles[1:] {
		if title.Duration > best.Duration {
			best = title
		}
	}
	return best, nil
}

func batchRemuxTitleWindows(ctx context.Context, src string, title titleInfo, outDir string, tools toolPaths, progress batchProgressFunc) (string, error) {
	src, err := normalizeSource(src)
	if err != nil {
		return "", err
	}
	if err := validateOutputDir(outDir); err != nil {
		return "", err
	}
	final := outputPath(src, outDir, title.Number)
	if _, err := os.Stat(final); err == nil {
		return "", fmt.Errorf("output already exists: %s", final)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("check output path: %w", err)
	}
	partial, err := reservePartialOutput(final)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(partial)
		}
	}()
	progress(0, fmt.Sprintf("Remuxing longest title %d with fixed timestamps and all streams…", title.Number))
	args := []string{"-hide_banner", "-nostdin", "-y"}
	args = appendDesktopDVDInput(args, title.Number, src)
	args = append(args, "-map", "0", "-c", "copy", "-map_metadata", "0", "-map_chapters", "0", partial)
	if _, err := runHidden(ctx, tools.ffmpeg, args...); err != nil {
		return "", fmt.Errorf("FFmpeg batch remux failed: %w", err)
	}
	if err := validateAndSyncOutput(partial); err != nil {
		return "", err
	}
	if err := commitOutputNoReplace(partial, final); err != nil {
		cleanup = false
		return "", fmt.Errorf("%w; completed MKV retained at %s", err, partial)
	}
	cleanup = false
	progress(1, "Completed: "+final)
	return final, nil
}
