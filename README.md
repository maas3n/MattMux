# MattMux

**MattMux** remuxes DVD-Video titles to MKV **without transcoding**. Windows and Linux builds are functional, with an experimental Android/ChromeOS app preview. All platform source now lives on the single `main` branch; releases are identified by tags and GitHub Releases rather than long-lived platform branches.

## Downloads

| Platform | Status | Recommended download |
| --- | --- | --- |
| Windows x64 | **Stable — 1.2.0** | [MattMux 1.2.0](https://github.com/maas3n/MattMux/releases/tag/v1.2.0) — All-in-One EXE, installer, or portable ZIP |
| Linux amd64 | **Preview — 1.3.0-dev5** | [MattMux 1.3.0-dev5](https://github.com/maas3n/MattMux/releases/tag/v1.3.0-dev5) — standalone executable or self-contained `.deb` |
| Android / ChromeOS | **Experimental — 1.2.0 Alpha 4** | [ChromeOS Alpha 4](https://github.com/maas3n/MattMux/releases/tag/v1.2.0-chromeos-alpha4) — APK plus native-runtime source/provenance assets |

## Highlights

- DVD folder / `VIDEO_TS` / ISO input
- Lossless stream-copy remuxing: no video or audio re-encoding
- Longest-title auto-selection after scanning
- Optional DVD chapter preservation
- Native Go IFO chapter parser on desktop with FFprobe fallback
- Cancelable scans, metadata reads, and remuxes
- Unique operation-owned temporary outputs with validated, no-overwrite finalization on desktop
- Pinned and SHA-256-verified third-party runtime tools
- Windows installer, portable ZIP, and one-file All-in-One EXE
- Linux self-contained `.deb`, tarball, CLI, GUI, and one-file standalone executable
- Experimental Android/ChromeOS native FFmpeg/libudfread remux path with dedicated parity tests

## Requirements

### Windows
- Windows x64
- Self-contained published packages are recommended
- Current binaries are not Authenticode-signed

### Linux
- amd64 / x86_64
- `.deb` targets Debian/Ubuntu-family systems
- Standalone build expects a normal 64-bit desktop Linux runtime
- Bundled FFmpeg/FFprobe/MediaInfo remain private to MattMux and do not replace system tools

### Android / ChromeOS
- Experimental preview
- Android 8.0 / API 26 or newer
- arm64-v8a and x86_64 are targeted
- See [`android/README.md`](android/README.md) for current native-remux limitations

Use unencrypted DVD-Video sources or media you are authorized to process. MattMux does **not** bypass CSS or other DVD copy protection.

## Quick start

### Windows
Download the All-in-One EXE, Setup EXE, or Portable ZIP from the [1.2.0 release](https://github.com/maas3n/MattMux/releases/tag/v1.2.0).

### Linux standalone
```bash
chmod +x MattMux-1.3.0-dev5-Linux-amd64Standalone
./MattMux-1.3.0-dev5-Linux-amd64Standalone
```

### Debian / Ubuntu
```bash
sudo apt install ./MattMux-1.3.0-dev5-Linux-amd64.deb
mattmux
```

## Build from source

Everything is built from `main`.

### Windows
```powershell
powershell -ExecutionPolicy Bypass -File .\src\build.ps1
```

### Linux
```bash
bash packaging/linux/build-linux-release.sh 1.3.0-dev5
bash packaging/linux/build-linux-standalone.sh 1.3.0-dev5
```

### Android / ChromeOS
```bash
gradle -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
```

The Android native FFmpeg runtime is built by the GitHub Actions workflow; see [`android/native/README.md`](android/native/README.md) for the full native build process.

## CI and releases

`main` is the only long-lived source branch. Pull-request branches may be short-lived, but Windows, Linux, and Android/ChromeOS code are reviewed and integrated back into `main`.

- `.github/workflows/build.yml` validates the Windows/Go path.
- `.github/workflows/linux.yml` builds and validates the Linux packages.
- `.github/workflows/android.yml` runs Android/native tests and package validation.
- Release workflows live beside the source on `main`; published releases remain anchored by immutable version tags rather than release branches.

Historical release tags are retained even when old platform/release branches are retired.

## Third-party runtime tools

Desktop builds use FFmpeg/FFprobe and MediaInfo CLI. Android/ChromeOS uses native FFmpeg and libudfread. Exact pinned versions, hashes, source revisions, and licensing notes are documented in [`THIRD_PARTY.md`](THIRD_PARTY.md).

## License

MattMux is licensed under the [MIT License](LICENSE). Third-party components remain governed by their own licenses.
