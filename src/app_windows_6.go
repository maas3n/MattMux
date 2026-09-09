//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func centerWindow(hwnd uintptr, w, h int32) {
	sw, _, _ := procGetSystemMetrics.Call(0)
	sh, _, _ := procGetSystemMetrics.Call(1)
	x := (int32(sw) - w) / 2
	y := (int32(sh) - h) / 2
	if x < 0 { x = 0 }
	if y < 0 { y = 0 }
	procSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0004)
}

func utf16Ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func utf16FromStringWithNulls(s string) []uint16 {
	r := []rune(s)
	out := make([]uint16, 0, len(r)+1)
	for _, rr := range r {
		if rr == 0 { out = append(out, 0); continue }
		if rr <= 0xffff { out = append(out, uint16(rr)) } else { rr -= 0x10000; out = append(out, uint16(0xD800+(rr>>10)), uint16(0xDC00+(rr&0x3FF))) }
	}
	if len(out) == 0 || out[len(out)-1] != 0 { out = append(out, 0) }
	return out
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

func findFile(root, name string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if !info.IsDir() && strings.EqualFold(info.Name(), name) { found = path; return io.EOF }
		return nil
	})
	if err == io.EOF && found != "" { return found, nil }
	if err != nil { return "", err }
	return "", fmt.Errorf("%s was not found in the downloaded package", name)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src); if err != nil { return err }; defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { return err }
	out, err := os.Create(dst); if err != nil { return err }
	_, cpErr := io.Copy(out, in); closeErr := out.Close(); if cpErr != nil { return cpErr }; return closeErr
}

func tail(s string, max int) string { if len(s) <= max { return s }; return s[len(s)-max:] }
func firstLine(s string) string { s = strings.TrimSpace(s); if i := strings.IndexAny(s, "\r\n"); i >= 0 { s = s[:i] }; if len(s) > 180 { s = s[:177] + "…" }; return s }
