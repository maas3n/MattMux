//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestWindowsAdvancedMergerAndBatchTabs(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	icc := INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(INITCOMMONCONTROLSEX{})), DwICC: ICC_PROGRESS_CLASS | ICC_LISTVIEW_CLASSES | 0x8}
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	if err := createMainWindow(); err != nil {
		t.Fatal(err)
	}
	defer procDestroyWindow.Call(app.hwnd)
	if mergerWindow.tab == 0 || mergerWindow.list == 0 {
		t.Fatal("missing merger controls")
	}
	visible := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible")

	if len(mergerWindow.dvd) == 0 || len(mergerWindow.controls) == 0 || len(batchWindow.controls) == 0 {
		t.Fatal("missing tab page controls")
	}
	pages := [][]uintptr{mergerWindow.dvd, mergerWindow.controls, batchWindow.controls}
	// Exercise the notification handler used by real clicks, including repeated
	// returns to DVD Remux. TCM_SETCURSEL alone does not send TCN_SELCHANGE.
	for _, selected := range []uintptr{1, 2, 0, 2, 1, 0} {
		procSendMessageW.Call(mergerWindow.tab, 0x130c, selected, 0)
		header := mergerNotifyHeader{From: mergerWindow.tab, Code: -551}
		procSendMessageW.Call(app.hwnd, 0x004e, 0, uintptr(unsafe.Pointer(&header)))
		for page, controls := range pages {
			for _, c := range controls {
				v, _, _ := visible.Call(c)
				if c == 0 || (v != 0) != (uintptr(page) == selected) {
					t.Fatalf("tab %d: page %d control %#x has incorrect visibility", selected, page, c)
				}
			}
		}
		assertWindowsTabBackgroundErased(t)
	}
}

func assertWindowsTabBackgroundErased(t *testing.T) {
	t.Helper()
	// Paint into an offscreen DC so this tests actual background erasure without
	// depending on screen capture, desktop occlusion, or the CI monitor's DPI.
	gdi := syscall.NewLazyDLL("gdi32.dll")
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		t.Fatal("CreateCompatibleDC failed")
	}
	defer gdi.NewProc("DeleteDC").Call(dc)
	bitmap, _, _ := gdi.NewProc("CreateBitmap").Call(4, 4, 1, 32, 0)
	if bitmap == 0 {
		t.Fatal("CreateBitmap failed")
	}
	defer gdi.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi.NewProc("SelectObject").Call(dc, bitmap)
	defer gdi.NewProc("SelectObject").Call(dc, old)
	expected, _, _ := user32.NewProc("GetSysColor").Call(15) // COLOR_BTNFACE
	marker := expected ^ 0x00ffffff
	gdi.NewProc("SetPixel").Call(dc, 1, 1, marker)
	before, _, _ := gdi.NewProc("GetPixel").Call(dc, 1, 1)
	if before != marker {
		t.Fatalf("could not seed stale pixel: got %#x, want %#x", before, marker)
	}
	procSendMessageW.Call(app.hwnd, 0x0014, dc, 0) // WM_ERASEBKGND
	after, _, _ := gdi.NewProc("GetPixel").Call(dc, 1, 1)
	if after != expected {
		t.Fatalf("old tab pixel survived background erase: got %#x, want %#x", after, expected)
	}
}
