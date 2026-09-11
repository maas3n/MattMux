# MattMux

**MattMux** remuxes DVD-Video titles to MKV **without transcoding**. Windows, Linux, and Android/ChromeOS are developed together from the single `main` branch and released under one shared product version.

## Download MattMux 1.4.1

**MattMux 1.4.1** is the current unified release. It includes the release-audit fixes for output safety, Linux packaging/versioning, Android DVD metadata, persistent APK signing, and documentation consistency. Windows, Linux, and ChromeOS/Android packages are built from the same tagged source commit and published together in one GitHub Release.

[**Download MattMux 1.4.1**](https://github.com/maas3n/MattMux/releases/tag/v1.4.1)

| Platform | Recommended package | Other options |
| --- | --- | --- |
| Windows x64 | `MattMux-1.4.1-Windows-All-in-One.exe` | Setup EXE or Portable ZIP |
| Linux amd64 | `MattMux-1.4.1-Linux-amd64Standalone` | Self-contained `.deb` or tarball |
| Android / ChromeOS | `MattMux-1.4.1-ChromeOS.apk` | Experimental APK |

The release also includes source archives, third-party source/provenance files, per-platform SHA-256 manifests, and one combined `SHA256SUMS.txt`.

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless stream-copy remuxing: no video or audio re-encoding
- **Selectable video, audio, and subtitle tracks** from **Show Metadata**
  - every detected track is selected by default
  - deselect anything you do not want in the MKV
  - Windows and Linux use explicit FFmpeg `-map` stream selection
  - Android/ChromeOS performs the equivalent selection directly through native libav
- Longest-title auto-selection after scanning
- Optional DVD chapter preservation
- Native Go IFO chapter parser on desktop with FFprobe fallback
- Cancelable scans, metadata reads, and remuxes
- Unique operation-owned temporary outputs with validated, no-overwrite finalization on desktop
- Pinned and SHA-256-verified third-party runtime tools
- Windows installer, portable ZIP, and one-file All-in-One EXE
- Linux self-contained `.deb`, tarball, CLI, GUI, and one-file standalone executable
- Android/ChromeOS native FFmpeg/libudfread remux path with dedicated parity tests

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
- The v1.4.0 GitHub APK was debug-signed; v1.4.1 and later GitHub APKs use persistent distribution signing, so upgrading from v1.4.0 may require uninstalling it first
- See [`android/README.md`](android/README.md) for current native-remux details and limitations

Use unencrypted DVD-Video sources or media you are authorized to process. MattMux does **not** bypass CSS or other DVD copy protection.

## Quick start

### Windows

Download one of these from the [MattMux 1.4.1 release](https://github.com/maas3n/MattMux/releases/tag/v1.4.1):

- `MattMux-1.4.1-Windows-All-in-One.exe` — easiest single-file option
- `MattMux-1.4.1-Windows-Setup.exe` — normal installer
- `MattMux-1.4.1-Windows-Portable.zip` — portable package with bundled tools and portable data directory

### Linux standalone

```bash
chmod +x MattMux-1.4.1-Linux-amd64Standalone
./MattMux-1.4.1-Linux-amd64Standalone
```

### Debian / Ubuntu

```bash
sudo apt install ./MattMux-1.4.1-Linux-amd64.deb
mattmux
```

### Android / ChromeOS

Download `MattMux-1.4.1-ChromeOS.apk` from the [MattMux 1.4.1 release](https://github.com/maas3n/MattMux/releases/tag/v1.4.1) and install it on a compatible Android/ChromeOS device.

### Output location behavior

The Windows and Linux desktop GUIs default to the user's Videos directory (or home) and remember the chosen output folder. `mattmux-cli remux` instead writes to the current working directory when `--output` is omitted. The Windows All-in-One launcher may use its extraction directory as the child working directory, so the GUI output field remains authoritative.

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

MattMux **1.4.0** was the first unified release. **1.4.1** is the current unified release and carries the release-audit fixes.

MattMux previously used separate platform-specific development release lines. Those obsolete release entries and tags have been retired now that the unified release model is active.

Their development remains preserved in the Git history. The repository also retains the `archive/pre-single-trunk-history` archive tag for earlier history.

For current downloads, use the unified **MattMux 1.4.1** release. Future public releases will continue to use one shared version and one GitHub Release for all supported platforms.

## Third-party runtime tools

Desktop builds use FFmpeg/FFprobe and MediaInfo CLI. Android/ChromeOS uses native FFmpeg and libudfread. Exact pinned versions, hashes, source revisions, and licensing notes are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## License

MattMux is licensed under the [MIT License](LICENSE). Third-party components remain governed by their own licenses.
