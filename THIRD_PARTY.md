# Third-party tools

MattMux downloads these tools on demand. Their binaries are not committed to this repository.

## FFmpeg / FFprobe

- Provider: BtbN/FFmpeg-Builds
- Release: `autobuild-2026-09-08-23-15`
- Asset: `ffmpeg-N-126479-g08cd8df29d-win64-gpl-shared.zip`
- Checksum manifest: `checksums.sha256`
- Trusted manifest SHA-256: `d0bc1f689725bead0681dbcc0bfbf6eb12582d3100e6973d32837c6571b2e1dc`

MattMux verifies the pinned checksum manifest first, then verifies the archive against the asset hash contained in that verified manifest.

## MediaInfo CLI

- Version: `26.05`
- Asset: `MediaInfo_CLI_26.05_Windows_x64.zip`
- Trusted archive SHA-256: `f7f80620ce6d14f4995f0de6f98e3ef18ad29496db01899571152ee3311229f9`

The downloaded projects retain their respective licenses. If you redistribute third-party binaries, review and comply with their applicable license terms and source-distribution obligations.
