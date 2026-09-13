# MattMux

**MattMux** remuxes DVD-Video titles and combines selected media streams into MKV **without transcoding**. Windows, Linux, and Android/ChromeOS are developed together from the single `main` branch and released under one shared product version.

## Download MattMux 1.4.8

**MattMux 1.4.8** is a desktop hotfix/refactor release for Windows and Linux. It fixes Windows tab repaint ghosting, shortens the BATCH action label to **ONECLICK BATCH**, and makes the Windows/Linux DVD title-discovery path explicitly FFmpeg `dvdvideo`-based with the legacy MattMux `scanTitles()` / `probeDuration()` wrappers removed. Android/ChromeOS behavior is unchanged from 1.4.7.

[**Download MattMux 1.4.8**](https://github.com/maas3n/MattMux/releases/tag/v1.4.8)

| Platform | Recommended package | Other options |
| --- | --- | --- |
| Windows x64 | `MattMux-1.4.8-Windows-All-in-One.exe` | Setup EXE or Portable ZIP; Setup/Portable include `mattmux-cli.exe` |
| Linux amd64 | `MattMux-1.4.8-Linux-amd64Standalone` | Self-contained `.deb` or tarball; packaged CLI included |
| Android phone / tablet | `MattMux-1.4.8-Android.apk` | Signed universal APK; behavior unchanged from 1.4.7 |
| ChromeOS | `MattMux-1.4.8-ChromeOS.apk` | Byte-identical alias of the Android APK |

The release also includes source archives, third-party source/provenance files, per-platform SHA-256 manifests, and one combined `SHA256SUMS.txt`.

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless stream-copy remuxing: no video or audio re-encoding
- Desktop **BATCH** tab on Windows and Linux
  - **ONECLICK BATCH** processes every immediate `Movie Title/VIDEO_TS` folder in the selected collection
  - title discovery uses FFmpeg `dvdvideo` backed by `libdvdread`/`libdvdnav`; MattMux does not restore its old `ReadDVDTitleCount()` scanner
  - automatically selects the longest readable DVD title
  - includes all streams and chapters with `-c copy`
  - always applies `-analyzeduration 100M -probesize 100M -fflags +genpts`
  - completed MKVs go into their matching movie-title folders by default, or into one optional common output folder
  - Windows and Linux CLI batch mode supports an optional log file
- **Advanced Merger** for combining selected streams from multiple containers or elementary media files into MKV
  - **CHOOSE MOVIE FILES** exposes every discovered stream plus a selectable embedded chapter set when present
  - **CHOOSE AUDIO FILES FROM MKV or RAW** exposes only audio streams
  - **CHOOSE SUBTITLE FILES FROM MKV or RAW** exposes only subtitle streams
  - **CHOOSE CHAPTER FILE FROM MKV or RAW** accepts MKV chapters or valid `FFMETADATA1`
  - exact per-stream checkboxes, explicit mapping, attachment/data support, and chapter-title preservation
  - see [`docs/ADVANCED_MERGER.md`](docs/ADVANCED_MERGER.md)
- **Selectable video, audio, and subtitle tracks** from **Show Metadata** for DVD remuxing
  - every detected track is selected by default
  - deselect anything you do not want in the MKV
  - Windows and Linux use explicit FFmpeg `-map` stream selection
  - Android/ChromeOS performs the equivalent selection directly through native libav
- Longest-title auto-selection after scanning
- Windows/Linux **Scan Titles** discovers candidate DVD titles through FFprobe `dvdvideo` (`libdvdread` + `libdvdnav`) for both folders and ISOs; the legacy MattMux `scanTitles()` / `probeDuration()` wrappers are removed from title discovery
- Windows/Linux DVD remux and Advanced Merger inputs always use `-analyzeduration 100M -probesize 100M -fflags +genpts`; DVD **Start Remux** does not pre-index before muxing
- Optional DVD chapter preservation
- Native Go IFO chapter parser on desktop with FFprobe fallback
- Cancelable scans, metadata reads, remuxes, Advanced Merger jobs, and desktop batch jobs
- Unique operation-owned temporary outputs with validated, no-overwrite finalization on desktop
- Pinned and SHA-256-verified third-party runtime tools
- Windows installer, portable ZIP, one-file All-in-One EXE, and packaged batch CLI
- Linux self-contained `.deb`, tarball, CLI, GUI, and one-file standalone executable
- Android/ChromeOS native FFmpeg/libudfread remux and merger paths with dedicated parity tests

## BATCH

Open the **BATCH** tab on Windows or Linux and choose a collection folder laid out like this:

```text
Movies/
├── Movie One/
│   └── VIDEO_TS/
│       ├── VIDEO_TS.IFO
│       └── ...
├── Movie Two/
│   └── VIDEO_TS/
│       ├── VIDEO_TS.IFO
│       └── ...
└── Movie Three/
    └── VIDEO_TS/
        ├── VIDEO_TS.IFO
        └── ...
```

Click **ONECLICK BATCH** after choosing the collection. MattMux processes each immediate movie folder independently. It scans candidate titles through FFmpeg's `dvdvideo` demuxer, which uses `libdvdread`/`libdvdnav`, chooses the longest readable title, then performs a lossless all-stream MKV remux. The batch remux input always receives `-analyzeduration 100M -probesize 100M -fflags +genpts`.

Leave the BATCH output field blank to place the completed MKV inside the corresponding movie-title folder. Choose an output folder to collect all completed MKVs in one destination instead. Existing output files are not overwritten.

The Linux CLI and the Windows `mattmux-cli.exe` support the same batch interface:

```bash
mattmux-cli --batch --log=/my/folder/for/mattmux-batch.log /folder/containing/Movietitles /folder/for/finished/remuxes
```

`--log` is optional. The output-root argument is also optional; when it is omitted, each completed MKV is written into its corresponding movie-title folder.

## Advanced Merger

Open the **Advanced Merger** tab to combine tracks from multiple sources into one MKV without transcoding. Inputs may be normal containers such as MKV, MP4, AVI, and others supported by the bundled FFmpeg runtime, or supported elementary media such as H.264, MPEG-2/VOB, AAC, AC-3, MP3, DTS, SRT, WebVTT, SUP, and similar formats.

**CHOOSE MOVIE FILES** adds every probed stream from each selected movie/container to **Select Streams**, including video, audio, subtitle, attachment/data streams and an embedded chapter-set row when chapters are present. The Audio and Subtitle buttons remain filtered so adding the same MKV through **CHOOSE AUDIO FILES FROM MKV or RAW** exposes only its audio streams, while **CHOOSE SUBTITLE FILES FROM MKV or RAW** exposes only subtitle streams.

Embedded chapters can be selected from a movie input. Alternatively, **CHOOSE CHAPTER FILE FROM MKV or RAW** accepts either an MKV containing chapters or valid `FFMETADATA1`; a dedicated chapter source overrides selected movie chapters. Select the exact streams you want, choose the destination, and press **MUX TO MKV**.

Android/ChromeOS stages selected documents and the in-progress output in private temporary storage because Storage Access Framework documents are not always directly seekable. For VobSub subtitles on Android/ChromeOS, select the matching `.idx` and `.sub` files together. See [`docs/ADVANCED_MERGER.md`](docs/ADVANCED_MERGER.md) for details.

## Track selection

Click **Show Metadata** after selecting a DVD title to inspect its streams. MattMux presents detected video, audio, and subtitle tracks with checkboxes.

For example:

```text
Video
☑ #0  MPEG-2 Video   720×576

Audio
☑ #1  AC-3 5.1      English
☐ #2  AC-3 2.0      Commentary

Subtitles
☑ #3  DVD Subtitle  English
☐ #4  DVD Subtitle  Norwegian
```

All tracks start selected, preserving the traditional MattMux behavior unless you change the selection. At least one media stream must remain selected before remuxing. Chapter preservation remains a separate option.

Changing the DVD source or selected title clears the previous track selection so stream indexes from one title cannot accidentally be reused for another.

## Requirements

### Windows

- Windows x64
- Self-contained published packages are recommended
- Current binaries are not Authenticode-signed

### Linux

- amd64 / x86_64
- Published Linux binaries currently require **glibc 2.38 or newer**
- `.deb` targets Debian/Ubuntu-family systems that meet that runtime requirement
- Standalone build expects a normal 64-bit desktop Linux runtime
- Bundled FFmpeg/FFprobe/MediaInfo remain private to MattMux and do not replace system tools

### Android / ChromeOS

- Experimental
- Android 8.0 / API 26 or newer
- arm64-v8a and x86_64 are targeted
- The v1.4.0 GitHub APK was debug-signed; v1.4.1 and later GitHub APKs use persistent distribution signing, so upgrading from v1.4.0 may require uninstalling v1.4.0 first
- The `Android.apk` and `ChromeOS.apk` release assets are the same signed universal APK under device-friendly names
- Advanced Merger requires temporary free space for staged inputs plus the in-progress MKV
- See [`android/README.md`](android/README.md) for current native-remux details and limitations

Use unencrypted DVD-Video sources or media you are authorized to process. MattMux does **not** bypass CSS or other DVD copy protection.

## Quick start

### Windows

Download one of these from the [MattMux 1.4.8 release](https://github.com/maas3n/MattMux/releases/tag/v1.4.8):

- `MattMux-1.4.8-Windows-All-in-One.exe` — easiest single-file GUI option
- `MattMux-1.4.8-Windows-Setup.exe` — normal installer; includes `mattmux-cli.exe`
- `MattMux-1.4.8-Windows-Portable.zip` — portable GUI + `mattmux-cli.exe` with bundled tools and portable data directory

### Linux standalone

```bash
chmod +x MattMux-1.4.8-Linux-amd64Standalone
./MattMux-1.4.8-Linux-amd64Standalone
```

### Debian / Ubuntu

```bash
sudo apt install ./MattMux-1.4.8-Linux-amd64.deb
mattmux
mattmux-cli --version
```

### Android / ChromeOS

For Android phones/tablets, download `MattMux-1.4.8-Android.apk`. For Chromebooks, download `MattMux-1.4.8-ChromeOS.apk` from the [MattMux 1.4.8 release](https://github.com/maas3n/MattMux/releases/tag/v1.4.8). They are the same persistently signed universal APK; 1.4.8 does not change Android/ChromeOS product behavior from 1.4.7.

### Output location behavior

The Windows and Linux desktop GUIs default to the user's Videos directory (or home) and remember the chosen output folder. `mattmux-cli remux` instead writes to the current working directory when `--output` is omitted. BATCH has separate output behavior: omit its output root to write each MKV into its corresponding movie-title folder, or supply an output root to collect completed MKVs in one folder. The Windows All-in-One launcher may use its extraction directory as the child working directory, so the GUI output field remains authoritative.

## Build from source

Everything is built from `main`.

### Windows

```powershell
powershell -ExecutionPolicy Bypass -File .\src\build.ps1
```

### Linux

For a development build:

```bash
bash packaging/linux/build-linux-release.sh dev
bash packaging/linux/build-linux-standalone.sh dev
```

### Android / ChromeOS

Build the native runtime first, then run the Android tests/lint/package build:

```bash
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/30.0.16248370"
bash android/native/build-ffmpeg-android.sh
gradle -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
```

GitHub Actions uses the same native build step before Gradle; see [`android/native/README.md`](android/native/README.md) for the full native build process.

## Unified development and release model

`main` is the only long-lived source branch. Windows, Linux, and Android/ChromeOS changes are integrated into the same trunk and tested together.

- `.github/workflows/build.yml` validates the Windows/Go path.
- `.github/workflows/linux.yml` builds and validates Linux packages.
- `.github/workflows/android.yml` runs Android/native tests and package validation.
- `.github/workflows/release.yml` is the single GitHub Release publisher for all product versions.
- `.github/workflows/android-play.yml` builds a signed Play bundle when production signing inputs are supplied; it does not create a separate GitHub Release.

New public versions use one shared tag:

- stable: `vMAJOR.MINOR.PATCH`
- preview: `vMAJOR.MINOR.PATCH-alpha.N`, `-beta.N`, or `-rc.N`

One tag produces one release containing all applicable platform packages from the same source commit. Published tags and release assets are treated as immutable; fixes are released under a new version rather than replacing an old one.

See [`RELEASING.md`](RELEASING.md) for the full release policy.

## Release history

MattMux **1.4.0** was the first unified release. MattMux **1.4.1** introduced persistent Android distribution signing plus the release-audit fixes. MattMux **1.4.2** added explicit Android phone/tablet and ChromeOS APK asset names for the same signed universal build. MattMux **1.4.3** introduced the cross-platform Advanced Merger. MattMux **1.4.4** expanded it with all-stream movie imports, embedded chapter selection, MKV/FFMETADATA1 chapter overrides, and metadata preservation. MattMux **1.4.5** changes Windows/Linux title scanning to rely entirely on FFmpeg `dvdvideo` with `libdvdread`/`libdvdnav` for title discovery. MattMux **1.4.6** makes robust 100M analyze/probe limits plus generated timestamps unconditional for Windows/Linux mux inputs and removes DVD remux pre-indexing while retaining the 1.4.5 libdvdread/libdvdnav title scanner. MattMux **1.4.7** adds desktop one-click BATCH processing plus the matching Windows/Linux CLI batch interface. MattMux **1.4.8** fixes Windows tab repaint ghosting, renames the BATCH action to **ONECLICK BATCH**, and removes the legacy desktop `scanTitles()` / `probeDuration()` wrapper structure while keeping title discovery on FFmpeg `dvdvideo` with `libdvdread`/`libdvdnav`.

MattMux previously used separate platform-specific development release lines. Those obsolete release entries and tags have been retired now that the unified release model is active.

Their development remains preserved in the Git history. The repository also retains the `archive/pre-single-trunk-history` archive tag for earlier history.

For current downloads, use the unified **MattMux 1.4.8** release. Future public releases will continue to use one shared version and one GitHub Release for all supported platforms.

## Third-party runtime tools

Desktop builds use FFmpeg/FFprobe and MediaInfo CLI. Android/ChromeOS uses native FFmpeg and libudfread. Exact pinned versions, hashes, source revisions, and licensing notes are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## License

MattMux is licensed under the [MIT License](LICENSE). Third-party components remain governed by their own licenses.
