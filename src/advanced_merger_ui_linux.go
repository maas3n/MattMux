//go:build linux && !cli

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (g *linuxGUI) buildAdvancedMerger() fyne.CanvasObject {
	var streams []mergerStream
	var checks []*widget.Check
	list := container.NewVBox()
	chapter := widget.NewEntry()
	chapter.SetPlaceHolder("Optional FFMETADATA1 chapter file")
	output := widget.NewEntry()
	output.SetText(g.outputEntry.Text)
	name := widget.NewEntry()
	name.SetText("merged.mkv")
	status := widget.NewLabel("Choose files, then select the streams to include.")
	status.Wrapping = fyne.TextWrapWord
	var controls []fyne.Disableable
	var baseControlCount int
	var cancel context.CancelFunc
	cancelBtn := widget.NewButton("Cancel", func() {
		if cancel != nil {
			cancel()
		}
	})
	cancelBtn.Disable()
	busy := false
	run := func(label string, work func(context.Context) (func(), error)) {
		if busy {
			return
		}
		busy = true
		for _, c := range controls {
			c.Disable()
		}
		cancelBtn.Enable()
		status.SetText(label)
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		go func() {
			done, err := work(ctx)
			c()
			fyne.Do(func() {
				busy = false
				cancel = nil
				for _, control := range controls {
					control.Enable()
				}
				cancelBtn.Disable()
				if err != nil {
					status.SetText(err.Error())
					dialog.ShowError(err, g.window)
				} else if done != nil {
					done()
				}
			})
		}()
	}
	add := func(kind string) {
		// A folder browser with checkboxes supports multiple files without external dialogs.
		dir := widget.NewEntry()
		home, _ := os.UserHomeDir()
		dir.SetText(home)
		files := container.NewVBox()
		selected := map[string]bool{}
		var refresh func()
		refresh = func() {
			entries, err := os.ReadDir(dir.Text)
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			files.Objects = nil
			files.Add(widget.NewButton("Parent folder", func() { dir.SetText(filepath.Dir(dir.Text)); refresh() }))
			for _, entry := range entries {
				entry := entry
				p := filepath.Join(dir.Text, entry.Name())
				if entry.IsDir() {
					files.Add(widget.NewButton(entry.Name()+"/", func() { dir.SetText(p); refresh() }))
				} else {
					check := widget.NewCheck(entry.Name(), func(on bool) { selected[p] = on })
					check.SetChecked(selected[p])
					files.Add(check)
				}
			}
			files.Refresh()
		}
		refresh()
		scroll := container.NewVScroll(files)
		scroll.SetMinSize(fyne.NewSize(600, 320))
		d := dialog.NewCustomConfirm("Choose "+kind+" files", "Add files", "Cancel", container.NewBorder(container.NewBorder(nil, nil, nil, widget.NewButton("Open folder", refresh), dir), nil, nil, nil, scroll), func(ok bool) {
			if !ok {
				return
			}
			var paths []string
			for p, on := range selected {
				if on {
					paths = append(paths, p)
				}
			}
			sort.Strings(paths)
			run("Reading streams…", func(ctx context.Context) (func(), error) {
				tools, err := mergerTools(ctx)
				if err != nil {
					return nil, err
				}
				var added []mergerStream
				for _, p := range paths {
					ss, err := probeMergerFile(ctx, tools.ffprobe, p, kind)
					if err != nil {
						return nil, err
					}
					added = append(added, ss...)
				}
				return func() {
					for _, s := range added {
						duplicate := false
						for _, existing := range streams {
							if existing.Path == s.Path && existing.Track.Index == s.Track.Index {
								duplicate = true
							}
						}
						if duplicate {
							continue
						}
						streams = append(streams, s)
						check := widget.NewCheck(filepath.Base(s.Path)+" — "+s.Track.Label(), nil)
						check.SetChecked(true)
						checks = append(checks, check)
						controls = append(controls, check)
						list.Add(check)
					}
					status.SetText(fmt.Sprintf("%d streams available", len(streams)))
				}, nil
			})
		}, g.window)
		d.Resize(fyne.NewSize(680, 460))
		d.Show()
	}
	movies := widget.NewButton("CHOOSE MOVIE FILES", func() { add("video") })
	audio := widget.NewButton("CHOOSE AUDIO FILES", func() { add("audio") })
	subs := widget.NewButton("CHOOSE SUBTITLE FILES", func() { add("subtitle") })
	chapters := widget.NewButton("CHOOSE CHAPTER FILE", func() {
		dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, g.window)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			r.Close()
			run("Checking chapters…", func(ctx context.Context) (func(), error) {
				tools, err := mergerTools(ctx)
				if err != nil {
					return nil, err
				}
				if err = validateMergerChapters(ctx, tools.ffprobe, path); err != nil {
					return nil, err
				}
				return func() { chapter.SetText(path); status.SetText("Chapter file ready") }, nil
			})
		}, g.window)
	})
	folder := widget.NewButton("CHOOSE OUTPUT FOLDER", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, g.window)
			} else if uri != nil {
				output.SetText(uri.Path())
			}
		}, g.window)
	})
	mux := widget.NewButton("MUX TO MKV", func() {
		var chosen []mergerStream
		for i, s := range streams {
			if checks[i].Checked {
				chosen = append(chosen, s)
			}
		}
		filename := strings.TrimSpace(name.Text)
		if filepath.Base(filename) != filename || !strings.HasSuffix(strings.ToLower(filename), ".mkv") {
			dialog.ShowInformation("Output filename", "Enter a filename ending in .mkv, without folders.", g.window)
			return
		}
		dir, chap := output.Text, chapter.Text
		run("Muxing selected streams…", func(ctx context.Context) (func(), error) {
			if err := validateOutputDir(dir); err != nil {
				return nil, err
			}
			tools, err := mergerTools(ctx)
			if err != nil {
				return nil, err
			}
			path := filepath.Join(dir, filename)
			if err = muxMerger(ctx, tools, chosen, chap, path); err != nil {
				return nil, err
			}
			return func() { status.SetText("Completed: " + path) }, nil
		})
	})
	mux.Importance = widget.HighImportance
	clear := widget.NewButton("Clear streams", func() {
		streams = nil
		checks = nil
		controls = controls[:baseControlCount]
		list.Objects = nil
		list.Refresh()
		status.SetText("Choose files to add streams.")
	})
	controls = []fyne.Disableable{movies, audio, subs, chapters, folder, mux, clear, chapter, output, name}
	baseControlCount = len(controls)
	scroll := container.NewVScroll(list)
	return container.NewBorder(container.NewVBox(container.NewGridWithColumns(3, movies, audio, subs), widget.NewLabel("Select Streams")), container.NewVBox(clear, container.NewBorder(nil, nil, nil, chapters, chapter), container.NewBorder(nil, nil, nil, folder, output), container.NewBorder(nil, nil, widget.NewLabel("Output filename"), nil, name), status, container.NewHBox(mux, cancelBtn)), nil, nil, scroll)
}
