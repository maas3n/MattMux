//go:build linux && !cli

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"image/png"
	"os"
	"testing"
)

func TestAdvancedMergerAndBatchTabs(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("test")
	defer w.Close()
	g := &linuxGUI{window: w}
	g.build()
	tabs, ok := w.Content().(*container.AppTabs)
	if !ok {
		t.Fatal("missing tabs")
	}
	if len(tabs.Items) != 3 {
		t.Fatalf("tabs = %d; want 3", len(tabs.Items))
	}
	if tabs.Items[1].Text != "Advanced Merger" {
		t.Fatal("missing Advanced Merger tab")
	}
	if tabs.Items[2].Text != "BATCH" {
		t.Fatal("missing BATCH tab")
	}
	w.Resize(fyne.NewSize(840, 620))
	tabs.SelectIndex(2)
	if path := os.Getenv("MATTMUX_UI_CAPTURE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err = png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	if tabs.SelectedIndex() != 2 {
		t.Fatal("cannot select BATCH tab")
	}
}
