//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
)

func TestWindowsAdvancedMergerAndBatchTabs(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := createMainWindow(); err != nil {
		t.Fatal(err)
	}
	defer procDestroyWindow.Call(app.hwnd)
	if mergerWindow.tab == 0 || mergerWindow.list == 0 {
		t.Fatal("missing merger controls")
	}
	visible := syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible")

	procSendMessageW.Call(mergerWindow.tab, 0x130c, 1, 0)
	showMergerWindowsTab()
	selected, _, _ := procSendMessageW.Call(mergerWindow.tab, 0x130b, 0, 0)
	if selected != 1 {
		t.Fatal("merger tab not selected")
	}
	for _, c := range mergerWindow.controls {
		v, _, _ := visible.Call(c)
		if v == 0 {
			t.Fatal("merger control hidden")
		}
	}
	for _, c := range batchWindow.controls {
		v, _, _ := visible.Call(c)
		if v != 0 {
			t.Fatal("BATCH control overlapping merger tab")
		}
	}

	procSendMessageW.Call(mergerWindow.tab, 0x130c, 2, 0)
	showMergerWindowsTab()
	selected, _, _ = procSendMessageW.Call(mergerWindow.tab, 0x130b, 0, 0)
	if selected != 2 {
		t.Fatal("BATCH tab not selected")
	}
	if batchWindow.input == 0 || batchWindow.oneClick == 0 {
		t.Fatal("missing BATCH controls")
	}
	for _, c := range batchWindow.controls {
		v, _, _ := visible.Call(c)
		if v == 0 {
			t.Fatal("BATCH control hidden")
		}
	}
	for _, c := range mergerWindow.controls {
		v, _, _ := visible.Call(c)
		if v != 0 {
			t.Fatal("merger control overlapping BATCH tab")
		}
	}

	procSendMessageW.Call(mergerWindow.tab, 0x130c, 0, 0)
	showMergerWindowsTab()
}
