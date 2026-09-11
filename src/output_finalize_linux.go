//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func reservePartialOutput(final string) (string, error) {
	pattern := ".mattmux-" + filepath.Base(final) + ".*.partial.mkv"
	f, err := os.CreateTemp(filepath.Dir(final), pattern)
	if err != nil {
		return "", fmt.Errorf("create unique temporary output: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("close temporary output reservation: %w", err)
	}
	return name, nil
}

func validateAndSyncOutput(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("FFmpeg finished but the partial MKV was not created: %w", err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("FFmpeg output is not a regular file: %s", path)
	}
	if st.Size() == 0 {
		return errors.New("FFmpeg finished successfully but produced an empty MKV")
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open completed MKV for final sync: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync completed MKV: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close completed MKV: %w", err)
	}
	return nil
}

func finalizeRemuxOutput(partial, final string) error {
	if err := validateAndSyncOutput(partial); err != nil {
		return err
	}
	if err := commitOutputNoReplace(partial, final); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(final))
}

func commitOutputNoReplace(partial, final string) error {
	// A hard-link creation is atomic and fails if final already exists. Because
	// both names live in the output directory, they are necessarily on the same
	// filesystem. Removing the temporary name after linking leaves the inode at
	// the final path without ever replacing another writer's file.
	if err := os.Link(partial, final); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output already exists: %s", final)
		}
		return fmt.Errorf("remux finished but atomic no-overwrite commit failed: %w", err)
	}
	if err := os.Remove(partial); err != nil {
		return fmt.Errorf("committed output but could not remove temporary name %s: %w", partial, err)
	}
	return nil
}

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open output directory for sync: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync output directory: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close output directory after sync: %w", err)
	}
	return nil
}
