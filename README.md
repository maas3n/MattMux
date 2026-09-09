# MattMux

**MattMux** is a lightweight native Windows utility for remuxing a DVD-Video title to an MKV file **without transcoding**.

It accepts a DVD folder / `VIDEO_TS` structure or an ISO image, scans the available titles, and uses FFmpeg to copy the selected title's streams into MKV. Chapter preservation is optional and enabled by default.

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless remuxing: no video or audio re-encoding
- Longest-title auto-selection after scanning
- Optional DVD chapter preservation
- Native Go IFO chapter parser with FFprobe fallback
- Metadata viewer with detected chapter start times and durations
- Cancelable scans, downloads, metadata reads, and remuxes
- Writes to `*.partial.mkv` and renames only after a successful FFmpeg exit
- Pinned and SHA-256-verified FFmpeg and MediaInfo downloads
- ZIP path-traversal and oversized-entry protections
- Persistent settings and local logs under `%LOCALAPPDATA%\MattMux`
- Native Win32 UI written in dependency-free Go

## Requirements

- Windows x64
- An unencrypted DVD-Video folder/ISO, or media you are authorized to process
- Internet access on first use if FFmpeg/MediaInfo are not already cached

MattMux does **not** bypass DVD copy protection such as CSS.

## How it works

1. Choose a DVD folder or ISO.
2. Click **Scan Titles**.
3. MattMux scans the DVD and selects the longest title by default.
4. Choose an output folder.
5. Leave **Preserve chapters in the output MKV** enabled if you want chapters retained.
6. Click **Remux**.

FFmpeg remains the component that writes the final MKV. MattMux orchestrates source detection, title selection, dependency verification, metadata/chapter inspection, progress reporting, cancellation, and safe output handling.

## Chapter handling

For `VIDEO_TS` folders, MattMux first tries its conservative native Go IFO parser. ISO images and DVD layouts outside that parser automatically fall back to FFprobe's `dvdvideo` demuxer with pre-indexing.

When chapter preservation is enabled, MattMux uses FFmpeg's DVD pre-indexing and maps the source chapters. When disabled, chapter mapping is explicitly turned off.

No mkvmerge, ChapterGrabber, .NET runtime, or separate chapter utility is required.

## Third-party tools

MattMux downloads third-party tools on demand and verifies the expected files before use:

- **FFmpeg / FFprobe** — BtbN FFmpeg Builds
- **MediaInfo CLI** — MediaArea

The exact pinned versions and verification values are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## Build from source

Release builds require **Go 1.27.1 or newer** on Windows.

```powershell
powershell -ExecutionPolicy Bypass -File .\src\build.ps1
```

The build script runs the test suite before producing `MattMux.exe`.

You can also run the tests directly:

```powershell
cd src
go test ./...
go vet ./...
```

## Repository layout

```text
MattMux/
├── src/                       # Go source, tests, manifest, build script
├── packaging/offline-builder # Fully portable/offline bundle builder
├── .github/workflows/         # CI build/test workflow
├── CHANGELOG.md
├── THIRD_PARTY.md
└── README.md
```

Prebuilt installers and portable bundles should be distributed through **GitHub Releases** rather than committed into normal repository history.

## Version

Current source release: **1.2.0**

See [`CHANGELOG.md`](CHANGELOG.md) for release notes.

## License

No open-source license has been selected for MattMux yet. The source is publicly viewable in this repository, but no additional reuse/distribution rights are granted until a license is added.

FFmpeg, MediaInfo, and other third-party projects remain governed by their own licenses.
