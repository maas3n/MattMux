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

func TestAdvancedMergerTab(t *testing.T) {
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
	if len(tabs.Items) != 2 || tabs.Items[1].Text != "Advanced Merger" {
		t.Fatal("missing Advanced Merger tab")
	}
	w.Resize(fyne.NewSize(840, 620))
	tabs.SelectIndex(1)
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
	if tabs.SelectedIndex() != 1 {
		t.Fatal("cannot select merger tab")
	}
}
