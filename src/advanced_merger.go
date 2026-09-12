//go:build windows || linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type mergerStream struct {
	Path  string
	Track trackOption
}

func probeMergerFile(ctx context.Context, probe, path, kind string) ([]mergerStream, error) {
	if kind != "video" && kind != "audio" && kind != "subtitle" {
		return nil, errors.New("invalid stream category")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("choose a regular media file")
	}
	data, err := runMergerCommand(ctx, probe, "-v", "error", "-show_streams", "-of", "json", abs)
	if err != nil {
		return nil, err
	}
	var result ffprobeResult
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	var streams []mergerStream
	for _, t := range trackOptionsFromProbe(result) {
		if t.Kind == kind {
			streams = append(streams, mergerStream{abs, t})
		}
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("%s contains no %s streams", filepath.Base(abs), kind)
	}
	return streams, nil
}

func validateMergerChapters(ctx context.Context, probe, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header, err := bufio.NewReader(f).ReadSlice('\n')
	if (err != nil) || strings.TrimRight(string(header), "\r\n") != ";FFMETADATA1" {
		return errors.New("chapter file must start with ;FFMETADATA1 on its own line")
	}
	data, err := runMergerCommand(ctx, probe, "-v", "error", "-f", "ffmetadata", "-show_chapters", "-of", "json", path)
	if err != nil {
		return err
	}
	var result ffprobeChapterResult
	if err = json.Unmarshal(data, &result); err != nil {
		return err
	}
	if len(result.Chapters) == 0 {
		return errors.New("chapter file contains no chapters")
	}
	for _, chapter := range result.Chapters {
		start, e1 := strconv.ParseFloat(chapter.StartTime, 64)
		end, e2 := strconv.ParseFloat(chapter.EndTime, 64)
		if e1 != nil || e2 != nil || math.IsNaN(start) || math.IsNaN(end) || math.IsInf(start, 0) || math.IsInf(end, 0) || start < 0 || end <= start {
			return errors.New("chapter timestamps must have a nonnegative start and an end after the start")
		}
	}
	return nil
}

func mergerArgs(streams []mergerStream, chapters, output string) ([]string, error) {
	if len(streams) == 0 {
		return nil, errors.New("select at least one stream")
	}
	args := []string{"-hide_banner", "-nostdin", "-y"}
	inputs := map[string]int{}
	for _, s := range streams {
		if s.Path == "" || s.Track.Index < 0 {
			return nil, errors.New("invalid selected stream")
		}
		if s.Track.Kind != "video" && s.Track.Kind != "audio" && s.Track.Kind != "subtitle" {
			return nil, errors.New("unsupported selected stream type")
		}
		if _, ok := inputs[s.Path]; !ok {
			inputs[s.Path] = len(inputs)
			args = append(args, "-fflags", "+genpts", "-i", s.Path)
		}
	}
	if chapters != "" {
		args = append(args, "-f", "ffmetadata", "-i", chapters)
	}
	seen := map[string]bool{}
	for _, s := range streams {
		key := fmt.Sprintf("%d:%d", inputs[s.Path], s.Track.Index)
		if !seen[key] {
			args = append(args, "-map", key)
			seen[key] = true
		}
	}
	args = append(args, "-map_metadata", "-1", "-map_chapters")
	if chapters == "" {
		args = append(args, "-1")
	} else {
		args = append(args, fmt.Sprint(len(inputs)))
	}
	return append(args, "-c", "copy", "-f", "matroska", output), nil
}

func muxMerger(ctx context.Context, tools toolPaths, streams []mergerStream, chapters, output string) error {
	if chapters != "" {
		if err := validateMergerChapters(ctx, tools.ffprobe, chapters); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(output); err == nil {
		return errors.New("output already exists; choose a different filename")
	} else if !os.IsNotExist(err) {
		return err
	}
	partial, err := reservePartialOutput(output)
	if err != nil {
		return err
	}
	args, err := mergerArgs(streams, chapters, partial)
	if err != nil {
		os.Remove(partial)
		return err
	}
	if _, err = runMergerCommand(ctx, tools.ffmpeg, args...); err != nil {
		os.Remove(partial)
		return err
	}
	if err = finalizeRemuxOutput(partial, output); err != nil {
		return fmt.Errorf("%w (completed temporary file: %s)", err, partial)
	}
	return nil
}
