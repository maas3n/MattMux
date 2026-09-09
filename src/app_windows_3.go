//go:build windows

package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ensureTools(ctx context.Context, needMediaInfo bool) (toolPaths, error) {
	root := appDataRoot()
	ffDir := filepath.Join(root, "tools", "ffmpeg-2026-09-08")
	miDir := filepath.Join(root, "tools", "mediainfo-"+mediaInfoVersion)
	tools := toolPaths{ffmpeg: filepath.Join(ffDir, "ffmpeg.exe"), ffprobe: filepath.Join(ffDir, "ffprobe.exe"), mediainfo: filepath.Join(miDir, "MediaInfo.exe")}
	if !fileExists(tools.ffmpeg) || !fileExists(tools.ffprobe) { setStatus("Preparing FFmpeg — downloading verified release…"); if err := installFFmpeg(ctx, ffDir); err != nil { return toolPaths{}, err } }
	if err := checkDVDDemuxer(ctx, tools.ffmpeg); err != nil { return toolPaths{}, err }
	if needMediaInfo && !fileExists(tools.mediainfo) { setStatus("Preparing MediaInfo — downloading verified release…"); if err := installMediaInfo(ctx, miDir); err != nil { return toolPaths{}, err } }
	return tools, nil
}

func installFFmpeg(ctx context.Context, dst string) error {
	base := "https://github.com/BtbN/FFmpeg-Builds/releases/download/" + ffmpegReleaseTag
	work, err := os.MkdirTemp(appDataRoot(), "ffmpeg-install-"); if err != nil { return err }; defer os.RemoveAll(work)
	manifest := filepath.Join(work, "checksums.sha256")
	if err := downloadFile(ctx, base+"/checksums.sha256", manifest, 0.00, 0.05, 1<<20); err != nil { return fmt.Errorf("FFmpeg checksum manifest download failed: %w", err) }
	if err := verifySHA256(manifest, ffmpegManifestSHA); err != nil { return fmt.Errorf("FFmpeg checksum manifest verification failed: %w", err) }
	data, err := os.ReadFile(manifest); if err != nil { return err }
	archiveSHA := checksumForAsset(string(data), ffmpegAssetName); if archiveSHA == "" { return errors.New("FFmpeg checksum manifest does not contain the pinned Windows archive") }
	archive := filepath.Join(work, ffmpegAssetName)
	if err := downloadFile(ctx, base+"/"+ffmpegAssetName, archive, 0.05, 0.82, 300<<20); err != nil { return fmt.Errorf("FFmpeg download failed: %w", err) }
	if err := verifySHA256(archive, archiveSHA); err != nil { return fmt.Errorf("FFmpeg archive verification failed: %w", err) }
	extract := filepath.Join(work, "extract"); if err := unzipSafe(archive, extract); err != nil { return fmt.Errorf("FFmpeg extraction failed: %w", err) }
	ffmpegPath, err := findFile(extract, "ffmpeg.exe"); if err != nil { return err }; binDir := filepath.Dir(ffmpegPath)
	_ = os.RemoveAll(dst); if err := os.MkdirAll(dst, 0755); err != nil { return err }
	entries, err := os.ReadDir(binDir); if err != nil { return err }
	for _, e := range entries { if e.IsDir() { continue }; if err := copyFile(filepath.Join(binDir, e.Name()), filepath.Join(dst, e.Name())); err != nil { return err } }
	if !fileExists(filepath.Join(dst, "ffprobe.exe")) { return errors.New("downloaded FFmpeg package did not contain ffprobe.exe") }
	setProgress(0.84); return nil
}

func installMediaInfo(ctx context.Context, dst string) error {
	work, err := os.MkdirTemp(appDataRoot(), "mediainfo-install-"); if err != nil { return err }; defer os.RemoveAll(work)
	url := "https://mediaarea.net/download/binary/mediainfo/" + mediaInfoVersion + "/" + mediaInfoAssetName
	archive := filepath.Join(work, mediaInfoAssetName)
	if err := downloadFile(ctx, url, archive, 0.84, 0.98, 50<<20); err != nil { return fmt.Errorf("MediaInfo download failed: %w", err) }
	if err := verifySHA256(archive, mediaInfoSHA); err != nil { return fmt.Errorf("MediaInfo archive verification failed: %w", err) }
	extract := filepath.Join(work, "extract"); if err := unzipSafe(archive, extract); err != nil { return fmt.Errorf("MediaInfo extraction failed: %w", err) }
	p, err := findFile(extract, "MediaInfo.exe"); if err != nil { return err }
	_ = os.RemoveAll(dst); if err := os.MkdirAll(dst, 0755); err != nil { return err }; if err := copyFile(p, filepath.Join(dst, "MediaInfo.exe")); err != nil { return err }
	setProgress(1); return nil
}

func checkDVDDemuxer(ctx context.Context, ffmpeg string) error { out, err := runHidden(ctx, ffmpeg, "-hide_banner", "-demuxers"); if err != nil { return fmt.Errorf("FFmpeg self-check failed: %w", err) }; if !strings.Contains(string(out), "dvdvideo") { return errors.New("the installed FFmpeg build does not expose the dvdvideo demuxer") }; return nil }

func downloadFile(ctx context.Context, url, dest string, start, end float64, maxBytes int64) error {
	if !strings.HasPrefix(strings.ToLower(url), "https://") { return errors.New("refusing non-HTTPS download") }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil); if err != nil { return err }; req.Header.Set("User-Agent", appName+"/"+appVersion)
	client := &http.Client{Timeout: 20*time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error { if len(via) >= 10 { return errors.New("too many HTTP redirects") }; if !strings.EqualFold(req.URL.Scheme, "https") { return errors.New("refusing download redirect to non-HTTPS URL") }; return nil }}
	resp, err := client.Do(req); if err != nil { return err }; defer resp.Body.Close(); if resp.StatusCode < 200 || resp.StatusCode >= 300 { return fmt.Errorf("HTTP %s", resp.Status) }
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil { return err }; f, err := os.Create(dest); if err != nil { return err }; defer func(){ _ = f.Close() }()
	total := resp.ContentLength; if maxBytes > 0 && total > maxBytes { return fmt.Errorf("download is unexpectedly large (%d bytes)", total) }
	buf := make([]byte, 128*1024); var written int64; last := time.Now().Add(-time.Second)
	for { n, er := resp.Body.Read(buf); if n > 0 { if _, ew := f.Write(buf[:n]); ew != nil { return ew }; written += int64(n); if maxBytes > 0 && written > maxBytes { return fmt.Errorf("download exceeded the %d-byte safety limit", maxBytes) }; if total > 0 && time.Since(last) > 120*time.Millisecond { frac := float64(written)/float64(total); setProgress(start+frac*(end-start)); setStatus(fmt.Sprintf("Downloading tools… %.1f / %.1f MB", float64(written)/(1024*1024), float64(total)/(1024*1024))); last = time.Now() } }; if er == io.EOF { break }; if er != nil { return er }; if err := ctx.Err(); err != nil { return err } }
	if err := f.Sync(); err != nil { return err }; return f.Close()
}

func verifySHA256(path, expected string) error { f, err := os.Open(path); if err != nil { return err }; defer f.Close(); h := sha256.New(); if _, err := io.Copy(h, f); err != nil { return err }; got := hex.EncodeToString(h.Sum(nil)); expected = strings.ToLower(strings.TrimSpace(expected)); if got != expected { return fmt.Errorf("SHA-256 mismatch (got %s)", got) }; return nil }
func checksumForAsset(manifest, asset string) string { for _, line := range strings.Split(manifest, "\n") { fields := strings.Fields(strings.TrimSpace(line)); if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == asset { if len(fields[0]) == 64 { return strings.ToLower(fields[0]) } } }; return "" }

func unzipSafe(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath); if err != nil { return err }; defer zr.Close(); if err := os.MkdirAll(dest, 0755); err != nil { return err }; cleanDest, _ := filepath.Abs(dest)
	for _, zf := range zr.File { name := filepath.Clean(zf.Name); if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." { return fmt.Errorf("unsafe ZIP path: %q", zf.Name) }; outPath := filepath.Join(cleanDest, name); absOut, _ := filepath.Abs(outPath); if absOut != cleanDest && !strings.HasPrefix(strings.ToLower(absOut), strings.ToLower(cleanDest)+string(os.PathSeparator)) { return fmt.Errorf("unsafe ZIP path: %q", zf.Name) }; if zf.FileInfo().IsDir() { if err := os.MkdirAll(absOut, 0755); err != nil { return err }; continue }; if err := os.MkdirAll(filepath.Dir(absOut), 0755); err != nil { return err }; if zf.UncompressedSize64 > 2<<30 { return fmt.Errorf("ZIP entry is unexpectedly large: %q", zf.Name) }; rc, err := zf.Open(); if err != nil { return err }; out, err := os.OpenFile(absOut, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644); if err != nil { rc.Close(); return err }; written, cpErr := io.Copy(out, rc); closeErr := out.Close(); rc.Close(); if cpErr != nil { return cpErr }; if uint64(written) != zf.UncompressedSize64 { return fmt.Errorf("ZIP entry size mismatch: %q", zf.Name) }; if closeErr != nil { return closeErr } }
	return nil
}
