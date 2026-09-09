# MattMux

**MattMux** is a lightweight DVD-Video remuxing utility for **Windows, Linux, and experimental ChromeOS/Android builds**. It remuxes a DVD-Video title to an MKV file **without transcoding**.

It accepts a DVD folder / `VIDEO_TS` structure or an ISO image, scans the available titles, and uses FFmpeg to copy the selected title's streams into MKV. Chapter preservation is optional and enabled by default.

## Downloads

Prebuilt packages are published on [GitHub Releases](https://github.com/maas3n/MattMux/releases).

| Platform | Status | Package | What you get |
| --- | --- | --- | --- |
| Windows x64 | Stable | `.exe` installer | MattMux desktop GUI |
| Debian / Ubuntu amd64 | Preview | `.deb` package | MattMux GUI + `mattmux-cli` |
| Linux amd64 | Preview | `.tar.gz` portable bundle | MattMux GUI + `mattmux-cli` |
| ChromeOS / Android | Alpha | `.apk` | Self-contained experimental app build |

Current release lines include:

- **Windows:** `v1.2.0` with `MattMux-1.2.0-Setup.exe`
- **Linux preview:** `v1.3.0-dev1` with `mattmux_1.3.0.dev1_amd64.deb` and `MattMux-1.3.0-dev1-Linux-amd64.tar.gz`
- **ChromeOS alpha:** `v1.2.0-chromeos-alpha1` with `MattMux-1.2.0-ChromeOS-Alpha1.apk`

The Linux and ChromeOS builds are currently prerelease/experimental builds. Check the release notes for platform-specific limitations before installing.

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless remuxing: no video or audio re-encoding
- Longest-title auto-selection after scanning
- Optional DVD chapter preservation
- Native Go IFO chapter parser with FFprobe fallback
- Metadata viewer with detected chapter start times and durations
- Cancelable scans, downloads, metadata reads, and remuxes
- Writes to `*.partial.mkv` and renames only after a successful FFmpeg exit
- Pinned and SHA-256-verified FFmpeg downloads when a suitable system FFmpeg is unavailable
- Windows desktop GUI
- Linux desktop GUI plus `mattmux-cli`
- Experimental ChromeOS / Android APK build

## Requirements

### Windows

- Windows x64
- An unencrypted DVD-Video folder/ISO, or media you are authorized to process
- Internet access on first use if FFmpeg/MediaInfo are not already cached

### Linux preview

- Debian / Ubuntu amd64 for the packaged `.deb`, or a compatible amd64 Linux system for the portable tarball
- An unencrypted DVD-Video folder/ISO, or media you are authorized to process
- A suitable system `ffmpeg` / `ffprobe`, or internet access so MattMux can download and verify its pinned fallback build

The Linux release contains both:

- `mattmux` — desktop GUI
- `mattmux-cli` — command-line interface

For Linux build and packaging details, see the [Linux packaging README](https://github.com/maas3n/MattMux/blob/linux-support/packaging/linux/README.md).

### ChromeOS / Android alpha

The APK is an early test build for Chromebook/Android installation, UI, and native-runtime testing. It is self-contained and bundles its FFmpeg runtime. It is **not** the final Play Store build.

MattMux does **not** bypass DVD copy protection such as CSS.

## How it works

1. Choose a DVD folder or ISO.
2. Scan the available titles.
3. MattMux selects the longest title by default.
4. Choose an output folder.
5. Leave chapter preservation enabled if you want chapters retained.
6. Start the remux.

FFmpeg remains the component that writes the final MKV. MattMux orchestrates source detection, title selection, dependency verification, metadata/chapter inspection, progress reporting, cancellation, and safe output handling.

## Chapter handling

For `VIDEO_TS` folders, MattMux first tries its conservative native Go IFO parser. ISO images and DVD layouts outside that parser automatically fall back to FFprobe's `dvdvideo` demuxer with pre-indexing where supported by the platform build.

When chapter preservation is enabled, MattMux uses FFmpeg's DVD pre-indexing and maps the source chapters. When disabled, chapter mapping is explicitly turned off.

No mkvmerge, ChapterGrabber, .NET runtime, or separate chapter utility is required.

## Third-party tools

MattMux uses FFmpeg / FFprobe for DVD input and MKV output, with platform-specific runtime handling:

- **Windows:** downloads pinned FFmpeg / FFprobe and MediaInfo builds on demand and verifies them before use.
- **Linux:** prefers suitable system `ffmpeg` / `ffprobe`; if they are missing or unsuitable, MattMux can download and verify its pinned Linux fallback. System MediaInfo is optional.
- **ChromeOS / Android alpha:** the current test APK bundles an LGPL-only FFmpeg runtime and does not require a separate FFmpeg download after installation.

The exact pinned versions and verification values are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md) and in the relevant release assets/notes.

## Build from source

### Windows

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

### Linux preview

Linux GUI/CLI development currently lives on the `linux-support` branch. The Linux build produces both `mattmux` and `mattmux-cli`, and the packaging script can create the `.deb`, portable tarball, source tarball, and checksums.

See [`packaging/linux/README.md` on the linux-support branch](https://github.com/maas3n/MattMux/blob/linux-support/packaging/linux/README.md) for build prerequisites and commands.

## Repository layout

```text
MattMux/
├── src/               # Go source, tests, manifest, build scripts
├── .github/workflows/ # CI and release workflows
├── CHANGELOG.md
├── THIRD_PARTY.md
└── README.md
```

Cross-platform preview work may also include platform packaging files on development branches.

Prebuilt installers and portable bundles are distributed through **GitHub Releases** rather than committed into normal repository history.

## Release status

- **Windows:** stable release line
- **Linux:** preview release line with GUI and CLI builds
- **ChromeOS / Android:** alpha test APK

See [GitHub Releases](https://github.com/maas3n/MattMux/releases) for the newest packages, checksums, and release notes.

## Support MattMux

If MattMux is useful to you and you'd like to support its development, Bitcoin donations are appreciated but entirely optional.

<img src="https://upload.wikimedia.org/wikipedia/commons/4/46/Bitcoin.svg" width="14" height="14"> **BTC:** `bc1q79hj2zukfmm75278a7wssjmexanuhvs5nequel`

## License

MattMux is licensed under the [MIT License](LICENSE).

FFmpeg, MediaInfo, and other third-party projects remain governed by their own licenses.
