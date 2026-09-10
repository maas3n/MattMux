# MattMux

**MattMux** remuxes DVD-Video titles to MKV **without transcoding**. Windows and Linux builds are functional today, and there is also an experimental ChromeOS/Android app preview.

MattMux accepts a DVD folder / `VIDEO_TS` structure or an ISO image, scans the available titles, and uses FFmpeg to copy the selected title's streams into MKV. Chapter preservation is optional and enabled by default.

## Downloads

| Platform | Status | Recommended download |
| --- | --- | --- |
| Windows x64 | **Stable — 1.2.0** | [MattMux 1.2.0 release](https://github.com/maas3n/MattMux/releases/tag/v1.2.0) — All-in-One EXE, installer, or portable ZIP |
| Linux amd64 | **Preview — 1.3.0-dev5** | [MattMux 1.3.0-dev5 release](https://github.com/maas3n/MattMux/releases/tag/v1.3.0-dev5) — standalone executable or self-contained `.deb` |
| ChromeOS / Android | **Experimental — 1.2.0 Alpha 2** | [ChromeOS Alpha 2 release](https://github.com/maas3n/MattMux/releases/tag/v1.2.0-chromeos-alpha2) |

For Linux, `MattMux-1.3.0-dev5-Linux-amd64Standalone` is the simplest no-install option: make it executable and run it directly. The `.deb` is the recommended option if you want normal Debian/Ubuntu installation and menu integration.

The ChromeOS/Android build is currently an **app/native-runtime preview**. Its Android-native remux engine is still in development, so it should not be treated as equivalent to the Windows or Linux builds yet.

All published binaries and checksums are available under [GitHub Releases](https://github.com/maas3n/MattMux/releases).

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless remuxing: no video or audio re-encoding
- Longest-title auto-selection after scanning
- Optional DVD chapter preservation
- Native Go IFO chapter parser with FFprobe fallback
- Metadata viewer with detected chapter start times and durations
- Cancelable scans, metadata reads, and remuxes
- Writes to `*.partial.mkv` and renames only after a successful FFmpeg exit
- Pinned and SHA-256-verified third-party runtime tools
- Windows installer, portable ZIP, and one-file All-in-One EXE
- Linux self-contained `.deb` and one-file standalone executable
- Bundled Linux tools remain private to MattMux and do not replace system FFmpeg, FFprobe, or MediaInfo

## Requirements

### Windows

- Windows x64
- Use one of the published self-contained packages for the easiest setup
- The current binaries are not Authenticode-signed, so Windows may show a publisher/security warning

### Linux

- amd64 / x86_64
- The `.deb` is intended for Debian/Ubuntu-family systems
- The standalone build expects a normal 64-bit desktop Linux runtime with the usual graphical system libraries
- No system-wide FFmpeg, FFprobe, or MediaInfo replacement is performed by the self-contained `.deb` or standalone build

### ChromeOS / Android

- Experimental preview only
- Android 8.0 / API 26 or newer
- arm64-v8a and x86_64 builds are currently targeted

For every platform, use an unencrypted DVD-Video source or media you are authorized to process. MattMux does **not** bypass DVD copy protection such as CSS.

## Quick start

### Windows

Download either the All-in-One EXE, Setup EXE, or Portable ZIP from the [1.2.0 release](https://github.com/maas3n/MattMux/releases/tag/v1.2.0), then launch MattMux normally.

### Linux standalone

```bash
chmod +x MattMux-1.3.0-dev5-Linux-amd64Standalone
./MattMux-1.3.0-dev5-Linux-amd64Standalone
```

The standalone file contains MattMux plus its pinned FFmpeg, FFprobe, and MediaInfo runtime. On first launch it extracts its private runtime into the current user's cache and adjusts `PATH` only for the MattMux process. It does not modify the global system `PATH` or replace `/usr/bin/ffmpeg`, `/usr/bin/ffprobe`, or `/usr/bin/mediainfo`.

### Debian / Ubuntu package

```bash
sudo apt install ./MattMux-1.3.0-dev5-Linux-amd64.deb
mattmux
```

The package installs MattMux normally while keeping its bundled multimedia tools private under `/usr/lib/mattmux`.

## How it works

1. Choose a DVD folder or ISO.
2. Click **Scan Titles**.
3. MattMux scans the DVD and selects the longest title by default.
4. Choose an output folder.
5. Leave **Preserve chapters in the output MKV** enabled if you want chapters retained.
6. Click **Remux**.

FFmpeg remains the component that writes the final MKV. MattMux orchestrates source detection, title selection, dependency/runtime verification, metadata and chapter inspection, progress reporting, cancellation, and safe output handling.

## Chapter handling

For `VIDEO_TS` folders, MattMux first tries its conservative native Go IFO parser. ISO images and DVD layouts outside that parser automatically fall back to FFprobe's `dvdvideo` demuxer with pre-indexing.

When chapter preservation is enabled, MattMux uses FFmpeg's DVD pre-indexing and maps the source chapters. When disabled, chapter mapping is explicitly turned off.

No mkvmerge, ChapterGrabber, .NET runtime, or separate chapter utility is required.

## Third-party runtime tools

MattMux uses **FFmpeg / FFprobe** and **MediaInfo CLI**. The source repository does not commit their binary distributions, but the self-contained Windows and Linux release packages bundle verified copies so normal users do not need to install or replace those tools system-wide.

The Linux portable tarball is intentionally lighter and may use runtime discovery/fallback behavior instead of carrying the full self-contained bundle.

Exact pinned versions, source revisions, hashes, and licensing notes are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## Build from source

### Windows (`main` branch)

Release builds require **Go 1.27.1 or newer** on Windows.

```powershell
powershell -ExecutionPolicy Bypass -File .\src\build.ps1
```

You can run the tests directly with:

```powershell
cd src
go test ./...
go vet ./...
```

### Linux (`linux-support` branch)

The Linux release builder produces the GUI, CLI, self-contained `.deb`, portable tarball, source archive, and single-file standalone build:

```bash
git switch linux-support
bash packaging/linux/build-linux-release.sh 1.3.0-dev5
```

The script checks for its required build tools, downloads only the pinned third-party sources/assets used for packaging, and verifies the FFmpeg archive before use.

### ChromeOS / Android (`android-chromeos` branch)

Android development lives on the `android-chromeos` branch while the native remux engine is being completed.

## Repository branches

- **`main`** — stable Windows source and Windows release tooling
- **`linux-support`** — current Linux GUI/CLI and packaging work
- **`android-chromeos`** — experimental ChromeOS/Android app

Prebuilt binaries belong in **GitHub Releases** rather than normal repository history.

See [`CHANGELOG.md`](CHANGELOG.md) for the stable Windows release history.

## License

MattMux is licensed under the [MIT License](LICENSE).

FFmpeg, MediaInfo, and other third-party projects remain governed by their own licenses. See [`THIRD_PARTY.md`](THIRD_PARTY.md) for the runtime components currently used by each platform.
