//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
)

func TestWindowsAdvancedMergerTab(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := createMainWindow(); err != nil {
		t.Fatal(err)
	}
	defer procDestroyWindow.Call(app.hwnd)
	if mergerWindow.tab == 0 || mergerWindow.list == 0 {
		t.Fatal("missing merger controls")
	}
	procSendMessageW.Call(mergerWindow.tab, 0x130c, 1, 0)
	showMergerWindowsTab()
	selected, _, _ := procSendMessageW.Call(mergerWindow.tab, 0x130b, 0, 0)
	if selected != 1 {
		t.Fatal("merger tab not selected")
	}
	visible := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible")
	for _, c := range mergerWindow.controls {
		v, _, _ := visible.Call(c)
		if v == 0 {
			t.Fatal("merger control hidden")
		}
	}
	for _, c := range mergerWindow.dvd {
		v, _, _ := visible.Call(c)
		if v != 0 {
			t.Fatal("DVD control overlapping merger tab")
		}
	}
	procSendMessageW.Call(mergerWindow.tab, 0x130c, 0, 0)
	showMergerWindowsTab()
}
