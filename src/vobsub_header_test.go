//go:build windows || linux

package main

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestVobSubCanvasMetadata(t *testing.T) {
	palette := "palette: 000000, ffffff\n"
	dump := func(s string) string { return "00000000: " + hex.EncodeToString([]byte(s)) + "  text\n" }
	for _, tc := range []struct{ width, height int }{{720, 576}, {720, 480}} {
		header, err := vobSubHeader(dump(palette), tc.width, tc.height)
		if err != nil || string(header) != fmt.Sprintf("size: %dx%d\n%s", tc.width, tc.height, palette) {
			t.Fatalf("DVD canvas not restored: %q %v", header, err)
		}
	}
	original := "size: 1920x1080\n" + palette
	header, err := vobSubHeader(dump(original), 720, 576)
	if err != nil || string(header) != original {
		t.Fatalf("Existing canvas changed: %q %v", header, err)
	}
	if _, err := vobSubHeader(dump(palette), 0, 0); err == nil {
		t.Fatal("Guessed missing MKV canvas")
	}
	if _, err := vobSubHeader(dump("size: 720x576\n"), 720, 576); err == nil {
		t.Fatal("Accepted missing palette")
	}
}
