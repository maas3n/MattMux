from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly one match, got {count}\n--- expected ---\n{old}")
    p.write_text(text.replace(old, new, 1))


replace_once(
    "src/app_windows_2.go",
    '''\tpreserveChapters := isChecked(app.preserveChapters)\n\tif preserveChapters {\n\t\tsetStatus(fmt.Sprintf("Indexing chapters, then remuxing title %d to %s…", t.Number, filepath.Base(final)))\n\t} else {\n\t\tsetStatus(fmt.Sprintf("Remuxing title %d to %s…", t.Number, filepath.Base(final)))\n\t}\n\tsetProgress(0)\n\targs := []string{"-hide_banner", "-nostdin", "-y", "-fflags", "+genpts", "-probesize", "100M", "-analyzeduration", "100M", "-f", "dvdvideo", "-title", strconv.Itoa(t.Number)}\n\tif preserveChapters {\n\t\targs = append(args, "-preindex", "1")\n\t}\n\targs = append(args, "-i", src)''',
    '''\tpreserveChapters := isChecked(app.preserveChapters)\n\tsetStatus(fmt.Sprintf("Remuxing title %d with fixed timestamps to %s…", t.Number, filepath.Base(final)))\n\tsetProgress(0)\n\targs := []string{"-hide_banner", "-nostdin", "-y"}\n\targs = appendDesktopDVDInput(args, t.Number, src)'''
)
replace_once(
    "src/app_windows_2.go",
    '''\t\t\t\tsetStatus(fmt.Sprintf("Remuxing title %d… %d%%", t.Number, int(ratio*100)))''',
    '''\t\t\t\tsetStatus(fmt.Sprintf("Remuxing title %d with fixed timestamps… %d%%", t.Number, int(ratio*100)))'''
)

replace_once(
    "src/linux_core.go",
    '''\tif progress == nil {\n\t\tprogress = noopProgress\n\t}\n\targs := []string{"-hide_banner", "-nostdin", "-y", "-fflags", "+genpts", "-probesize", "100M", "-analyzeduration", "100M", "-f", "dvdvideo", "-title", strconv.Itoa(title.Number)}\n\tif preserve {\n\t\targs = append(args, "-preindex", "1")\n\t}\n\targs = append(args, "-i", src)''',
    '''\tif progress == nil {\n\t\tprogress = noopProgress\n\t}\n\tprogress(0, fmt.Sprintf("Remuxing title %d with fixed timestamps…", title.Number))\n\targs := []string{"-hide_banner", "-nostdin", "-y"}\n\targs = appendDesktopDVDInput(args, title.Number, src)'''
)
replace_once(
    "src/linux_core.go",
    '''\t\t\t\tprogress(ratio, fmt.Sprintf("Remuxing title %d… %d%%", title.Number, int(ratio*100)))''',
    '''\t\t\t\tprogress(ratio, fmt.Sprintf("Remuxing title %d with fixed timestamps… %d%%", title.Number, int(ratio*100)))'''
)

replace_once(
    "src/advanced_merger.go",
    '''\t\tif _, ok := inputs[s.Path]; !ok {\n\t\t\tinputs[s.Path] = len(inputs)\n\t\t\targs = append(args, "-fflags", "+genpts", "-i", s.Path)\n\t\t}''',
    '''\t\tif _, ok := inputs[s.Path]; !ok {\n\t\t\tinputs[s.Path] = len(inputs)\n\t\t\targs = appendDesktopRobustInput(args, s.Path)\n\t\t}'''
)
replace_once(
    "src/advanced_merger.go",
    '''\t\t} else {\n\t\t\tchapterIndex = len(inputs)\n\t\t\targs = append(args, "-i", chapters)\n\t\t}''',
    '''\t\t} else {\n\t\t\tchapterIndex = len(inputs)\n\t\t\targs = appendDesktopRobustInput(args, chapters)\n\t\t}'''
)

replace_once(
    "src/advanced_merger_test.go",
    '''\twant := []string{"-hide_banner", "-nostdin", "-y", "-fflags", "+genpts", "-i", "one.mkv", "-fflags", "+genpts", "-i", "two.mp4", "-i", "chapters.txt", "-map", "0:2", "-map_metadata:s:0", "0:s:2", "-map", "1:1", "-map_metadata:s:1", "1:s:1", "-map", "0:5", "-map_metadata:s:2", "0:s:5", "-map_metadata", "0", "-map_chapters", "2", "-c", "copy", "-f", "matroska", "out.mkv"}''',
    '''\twant := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "one.mkv", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "two.mp4", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "chapters.txt", "-map", "0:2", "-map_metadata:s:0", "0:s:2", "-map", "1:1", "-map_metadata:s:1", "1:s:1", "-map", "0:5", "-map_metadata:s:2", "0:s:5", "-map_metadata", "0", "-map_chapters", "2", "-c", "copy", "-f", "matroska", "out.mkv"}'''
)

replace_once(
    "docs/ADVANCED_MERGER.md",
    '''MattMux maps the exact selected input stream indexes and uses stream copy (`-c copy` on desktop, equivalent native libav packet copying on Android/ChromeOS). It does not intentionally re-encode video or audio. Desktop inputs use generated presentation timestamps (`-fflags +genpts`) before muxing.''',
    '''MattMux maps the exact selected input stream indexes and uses stream copy (`-c copy` on desktop, equivalent native libav packet copying on Android/ChromeOS). It does not intentionally re-encode video or audio. On Windows and Linux, every Advanced Merger input is opened with `-analyzeduration 100M -probesize 100M -fflags +genpts` before muxing so damaged or timestamp-irregular sources get the same robust input handling as DVD remuxes.'''
)

changelog = Path("CHANGELOG.md")
text = changelog.read_text()
marker = "# Changelog\n\n"
if not text.startswith(marker):
    raise SystemExit("unexpected CHANGELOG.md header")
section = '''## 1.4.6 — 2026-09-12\n\n- Keep the 1.4.5 Windows/Linux **Scan Titles** implementation unchanged: title discovery remains entirely on FFmpeg `dvdvideo` with `libdvdread`/`libdvdnav`, and MattMux's old `ReadDVDTitleCount()` parser stays removed.\n- Make Windows/Linux DVD remux inputs always use `-analyzeduration 100M -probesize 100M -fflags +genpts`.\n- Remove `-preindex 1` from the Windows/Linux **Start Remux** command so remuxing starts immediately instead of pre-indexing the DVD when chapter preservation is enabled; chapter mapping remains controlled by the existing Preserve chapters option.\n- Apply the same `-analyzeduration 100M -probesize 100M -fflags +genpts` input policy to every Windows/Linux Advanced Merger media input and separate chapter input.\n- Android/ChromeOS behavior is unchanged from 1.4.5.\n\n'''
changelog.write_text(marker + section + text[len(marker):])

readme = Path("README.md")
text = readme.read_text()
text = text.replace("## Download MattMux 1.4.5", "## Download MattMux 1.4.6", 1)
text = text.replace("**MattMux 1.4.5** is the current unified release line.", "**MattMux 1.4.6** is the current unified release line.", 1)
old_desc = "MattMux 1.4.5 is a desktop title-scanning hotfix on top of 1.4.4. On Windows and Linux, **Scan Titles** now uses FFmpeg `dvdvideo`/`libdvdread`/`libdvdnav` for both DVD folders and ISO images instead of using MattMux's own title-count parser. Advanced Merger behavior is unchanged from 1.4.4."
new_desc = "MattMux 1.4.6 keeps the 1.4.5 FFmpeg `dvdvideo`/`libdvdread`/`libdvdnav` title scanner and updates Windows/Linux mux input handling. DVD remuxes and Advanced Merger inputs now always use `-analyzeduration 100M -probesize 100M -fflags +genpts`, while **Start Remux** no longer enables `-preindex 1` before muxing."
if old_desc not in text:
    raise SystemExit("README release description changed")
text = text.replace(old_desc, new_desc, 1)
text = text.replace("[**Download MattMux 1.4.5**]", "[**Download MattMux 1.4.6**]", 1)
text = text.replace("MattMux-1.4.5-", "MattMux-1.4.6-")
text = text.replace("/releases/tag/v1.4.5", "/releases/tag/v1.4.6")
text = text.replace("MattMux 1.4.5 release", "MattMux 1.4.6 release")
scan_bullet = "- Windows/Linux **Scan Titles** probes DVD title numbers through FFmpeg `dvdvideo` (`libdvdread` + `libdvdnav`) for both folders and ISOs; MattMux no longer parses the DVD title count itself\n"
if scan_bullet not in text:
    raise SystemExit("README scan bullet changed")
text = text.replace(scan_bullet, scan_bullet + "- Windows/Linux DVD remux and Advanced Merger inputs always use `-analyzeduration 100M -probesize 100M -fflags +genpts`; DVD **Start Remux** does not pre-index before muxing\n", 1)
history_old = "MattMux **1.4.0** was the first unified release. MattMux **1.4.1** introduced persistent Android distribution signing plus the release-audit fixes. MattMux **1.4.2** added explicit Android phone/tablet and ChromeOS APK asset names for the same signed universal build. MattMux **1.4.3** introduced the cross-platform Advanced Merger. MattMux **1.4.4** expanded it with all-stream movie imports, embedded chapter selection, MKV/FFMETADATA1 chapter overrides, and metadata preservation. MattMux **1.4.5** changes Windows/Linux title scanning to rely entirely on FFmpeg `dvdvideo` with `libdvdread`/`libdvdnav` for title discovery."
history_new = history_old + " MattMux **1.4.6** makes robust 100M analyze/probe limits plus generated timestamps unconditional for Windows/Linux mux inputs and removes DVD remux pre-indexing while retaining the 1.4.5 libdvdread/libdvdnav title scanner."
if history_old not in text:
    raise SystemExit("README release history changed")
text = text.replace(history_old, history_new, 1)
text = text.replace("For current downloads, use the unified **MattMux 1.4.5** release.", "For current downloads, use the unified **MattMux 1.4.6** release.", 1)
readme.write_text(text)

Path("src/desktop_input_flags.go").write_text('''//go:build windows || linux\n\npackage main\n\nimport "strconv"\n\nfunc appendDesktopRobustInput(args []string, input string) []string {\n\treturn append(args, "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", input)\n}\n\nfunc appendDesktopDVDInput(args []string, title int, input string) []string {\n\treturn append(args, "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", strconv.Itoa(title), "-i", input)\n}\n''')
Path("src/desktop_input_flags_test.go").write_text('''//go:build windows || linux\n\npackage main\n\nimport (\n\t"reflect"\n\t"strings"\n\t"testing"\n)\n\nfunc TestDesktopRobustInputArgs(t *testing.T) {\n\tgot := appendDesktopRobustInput([]string{"-y"}, "input.mkv")\n\twant := []string{"-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-i", "input.mkv"}\n\tif !reflect.DeepEqual(got, want) {\n\t\tt.Fatalf("args = %q; want %q", got, want)\n\t}\n}\n\nfunc TestDesktopDVDInputStartsImmediatelyWithFixedTimestamps(t *testing.T) {\n\tgot := appendDesktopDVDInput([]string{"-hide_banner", "-nostdin", "-y"}, 7, "/dvd")\n\twant := []string{"-hide_banner", "-nostdin", "-y", "-analyzeduration", "100M", "-probesize", "100M", "-fflags", "+genpts", "-f", "dvdvideo", "-title", "7", "-i", "/dvd"}\n\tif !reflect.DeepEqual(got, want) {\n\t\tt.Fatalf("args = %q; want %q", got, want)\n\t}\n\tif strings.Contains(strings.Join(got, " "), "-preindex") {\n\t\tt.Fatalf("DVD remux input unexpectedly pre-indexes: %q", got)\n\t}\n}\n''')
