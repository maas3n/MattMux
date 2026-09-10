# Third-party software

MattMux relies on FFmpeg/FFprobe and MediaInfo on desktop platforms. Their source or binary distributions are not committed to the normal repository history. Self-contained release packages download/build the pinned versions during CI, verify them, and then bundle the resulting runtime privately with MattMux.

## Windows 1.2.0

### FFmpeg / FFprobe

- Provider: BtbN/FFmpeg-Builds
- Release: `autobuild-2026-09-08-23-15`
- Asset: `ffmpeg-N-126479-g08cd8df29d-win64-gpl-shared.zip`
- Checksum manifest: `checksums.sha256`
- Trusted manifest SHA-256: `f64be162403094773397bfcc299a4a059507028afa7563591fd05c17d56b3214`
- Verified archive SHA-256: `3139da8c0e3d201d16d849d2d6da2744b2b715f8d71184c4196db43da07b9607`

MattMux verifies the pinned checksum manifest first, then verifies the FFmpeg archive against the expected value before using it.

### MediaInfo CLI

- Version: `26.05`
- Asset: `MediaInfo_CLI_26.05_Windows_x64.zip`
- Trusted archive SHA-256: `f7f80620ce6d14f4995f0de6f98e3ef18ad29496db01899571152ee3311229f9`

The Windows Setup EXE, Portable ZIP, and All-in-One EXE bundle these verified runtime tools for normal offline-capable use.

## Linux 1.3.0-dev5

### FFmpeg / FFprobe

- Provider: BtbN/FFmpeg-Builds
- Release: `autobuild-2026-09-08-23-15`
- Asset: `ffmpeg-N-126479-g08cd8df29d-linux64-gpl.tar.xz`
- Trusted archive SHA-256: `635a2d74de852064852e95db5a9c475a86d36e2b6390e3c1ba5e46b2c46dfce0`

The self-contained `.deb` and single-file standalone build keep FFmpeg and FFprobe private to MattMux. They do not install or replace `/usr/bin/ffmpeg` or `/usr/bin/ffprobe` and do not modify the global system `PATH`.

### MediaInfo CLI

Linux MediaInfo is built from an exact pinned source set rather than from moving branches:

- MediaInfo CLI tag: `v26.05`
- MediaInfo CLI commit: `4728f24b666117a19d36515d95b9367fbb37aaf6`
- MediaInfoLib commit: `8bfa658657da9e16470c9fb32035e0fa097c0112`
- ZenLib commit: `2ddc277fe7ecfcbfe45616bb9cd9e23079113ecd`
- MediaArea zlib commit: `eaaf237c8cbc7310170c43202c6ec2cff64fff66`

The self-contained Linux packages keep the resulting MediaInfo binary private to MattMux and do not replace `/usr/bin/mediainfo`.

## ChromeOS / Android Alpha 2

The experimental Android/ChromeOS app uses native FFmpeg libraries rather than launching `ffprobe` or MediaInfo as subprocesses.

- FFmpeg version: `9.0.1`
- FFmpeg source SHA-256: `cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635`
- Current target ABIs: `arm64-v8a`, `x86_64`

The Android build is currently configured around an LGPL-only FFmpeg runtime and remains an experimental app/native-runtime preview while its remux engine is completed.

## Licensing

MattMux itself is licensed under MIT. Third-party projects keep their own licenses and copyright notices.

The Windows and Linux FFmpeg distributions currently used by MattMux are GPL-enabled builds because the desktop DVD workflow depends on FFmpeg's `dvdvideo` support with the relevant DVD libraries. The Android FFmpeg build is handled separately under its own build configuration and licensing requirements.

Anyone redistributing MattMux together with third-party binaries should review and satisfy the corresponding FFmpeg, BtbN/FFmpeg-Builds, MediaInfo, MediaInfoLib, ZenLib, zlib, and other applicable license/source-distribution obligations.
