//go:build windows || linux

package main

import (
	"context"
	"fmt"
)

func validateSelectedMergerChapters(ctx context.Context, probe, path string, movie bool, title int) error {
	if title == 0 {
		return validateMergerChapterSource(ctx, probe, path, movie)
	}
	chapters, err := probeChapters(ctx, probe, path, title)
	if err != nil {
		return err
	}
	if len(chapters) == 0 {
		return fmt.Errorf("DVD title %d contains no chapters", title)
	}
	return nil
}
