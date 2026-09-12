//go:build linux && !cli

package main

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
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
	tabs.SelectIndex(1)
	if tabs.SelectedIndex() != 1 {
		t.Fatal("cannot select merger tab")
	}
}
