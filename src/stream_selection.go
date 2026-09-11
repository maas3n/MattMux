//go:build windows || linux

package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type trackOption struct {
	Index         int
	Kind          string
	Codec         string
	Language      string
	Title         string
	Width         int
	Height        int
	Channels      int
	ChannelLayout string
}

func trackOptionsFromProbe(probe ffprobeResult) []trackOption {
	options := make([]trackOption, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		if stream.CodecType != "video" && stream.CodecType != "audio" && stream.CodecType != "subtitle" {
			continue
		}
		options = append(options, trackOption{
			Index:         stream.Index,
			Kind:          stream.CodecType,
			Codec:         stream.CodecName,
			Language:      strings.TrimSpace(stream.Tags["language"]),
			Title:         strings.TrimSpace(stream.Tags["title"]),
			Width:         stream.Width,
			Height:        stream.Height,
			Channels:      stream.Channels,
			ChannelLayout: strings.TrimSpace(stream.ChannelLayout),
		})
	}
	return options
}

func (t trackOption) Label() string {
	kind := t.Kind
	switch t.Kind {
	case "video":
		kind = "Video"
	case "audio":
		kind = "Audio"
	case "subtitle":
		kind = "Subtitle"
	}
	parts := []string{fmt.Sprintf("%s  #%d  %s", kind, t.Index, friendlyCodec(t.Codec))}
	if t.Width > 0 && t.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", t.Width, t.Height))
	}
	if t.Channels > 0 {
		channels := fmt.Sprintf("%d ch", t.Channels)
		if t.ChannelLayout != "" {
			channels += " (" + t.ChannelLayout + ")"
		}
		parts = append(parts, channels)
	}
	if t.Language != "" {
		parts = append(parts, "["+t.Language+"]")
	}
	if t.Title != "" {
		parts = append(parts, t.Title)
	}
	return strings.Join(parts, "  ")
}

// ffmpegStreamMapArgs converts the user's absolute input stream indexes to
// explicit FFmpeg maps. A nil slice means the metadata window was never used,
// so the historical "include every stream" behavior is preserved. An explicit
// empty slice is rejected so a user cannot accidentally create an empty MKV.
func ffmpegStreamMapArgs(selected []int) ([]string, error) {
	if selected == nil {
		return []string{"-map", "0"}, nil
	}
	if len(selected) == 0 {
		return nil, errors.New("select at least one video, audio, or subtitle track before remuxing")
	}
	unique := make(map[int]struct{}, len(selected))
	for _, index := range selected {
		if index < 0 {
			return nil, fmt.Errorf("invalid stream index %d", index)
		}
		unique[index] = struct{}{}
	}
	indexes := make([]int, 0, len(unique))
	for index := range unique {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	args := make([]string, 0, len(indexes)*2)
	for _, index := range indexes {
		args = append(args, "-map", fmt.Sprintf("0:%d", index))
	}
	return args, nil
}
