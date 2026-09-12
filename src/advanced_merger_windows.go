//go:build windows

package main

import "context"

func mergerTools(ctx context.Context) (toolPaths, error) { return ensureTools(ctx, false) }
func runMergerCommand(ctx context.Context, path string, args ...string) ([]byte, error) {
	return runHidden(ctx, path, args...)
}
