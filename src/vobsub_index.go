//go:build windows || linux

package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

func probeExtraBytes(dump string) ([]byte, error) {
	var result []byte
	for _, line := range strings.Split(dump, "\n") {
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		value := strings.TrimLeft(line[colon+1:], " ")
		value = strings.SplitN(value, "  ", 2)[0]
		b, err := hex.DecodeString(strings.ReplaceAll(value, " ", ""))
		if err != nil {
			return nil, err
		}
		result = append(result, b...)
	}
	return result, nil
}

// FFmpeg's VOB muxer creates MPEG-2 pack/PES records. Index their PTS and pack
// positions; preserve the original DVD subtitle palette from codec extradata.
func vobSubHeader(extra string, dvdWidth, dvdHeight int) ([]byte, error) {
	palette, err := probeExtraBytes(extra)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(palette), "size:") && dvdWidth > 0 && dvdHeight > 0 {
		palette = append([]byte(fmt.Sprintf("size: %dx%d\n", dvdWidth, dvdHeight)), palette...)
	}
	if !strings.Contains(string(palette), "palette:") || !strings.Contains(string(palette), "size:") {
		return nil, errors.New("DVD subtitle palette/size is missing; cannot create a correct IDX/SUB pair")
	}
	return palette, nil
}

func writeVobSubIndex(path, extra, language string, dvdWidth, dvdHeight int) error {
	palette, err := vobSubHeader(extra, dvdWidth, dvdHeight)
	if err != nil {
		return err
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()
	if len(language) != 2 {
		language = "--"
	}
	var idx strings.Builder
	fmt.Fprintf(&idx, "# VobSub index file, v7 (do not modify this line!)\n%s\nid: %s, index: 0\n", strings.TrimSpace(string(palette)), language)
	block := make([]byte, 2048)
	var pos int64
	count := 0
	var last int64 = -1
	for {
		n, e := io.ReadFull(input, block)
		if e == io.EOF {
			break
		}
		if e != nil {
			return fmt.Errorf("invalid VobSub pack: %w", e)
		}
		if n != 2048 || string(block[:4]) != "\x00\x00\x01\xba" {
			return errors.New("invalid VobSub MPEG pack")
		}
		for i := 14; i+14 < n; i++ {
			if string(block[i:i+4]) != "\x00\x00\x01\xbd" {
				continue
			}
			if block[i+7]&0x80 == 0 || block[i+8] < 5 {
				continue
			}
			payload := i + 9 + int(block[i+8])
			if payload >= n || block[payload] != 0x20 {
				continue
			}
			p := block[i+9 : i+14]
			pts := (int64(p[0]&14) << 29) | (int64(p[1]) << 22) | (int64(p[2]&254) << 14) | (int64(p[3]) << 7) | int64(p[4]>>1)
			if pts != last {
				fmt.Fprintf(&idx, "timestamp: %s, filepos: %09x\n", strings.ReplaceAll(chapterTimestamp(pts/90), ".", ":"), pos)
				count++
				last = pts
			}
			break
		}
		pos += 2048
	}
	if count == 0 {
		return errors.New("no timed DVD subtitle packets were exported")
	}
	return os.WriteFile(strings.TrimSuffix(path, ".sub")+".idx", []byte(idx.String()), 0644)
}
