## MattMux 1.2.0

MattMux is a native Windows utility for remuxing DVD-Video titles to MKV without transcoding.

### Highlights

- DVD folder, `VIDEO_TS`, and ISO input support
- Lossless stream-copy remuxing to MKV
- Automatic longest-title selection after scanning
- Optional chapter preservation
- Native Go IFO chapter parsing with FFprobe fallback
- Metadata/chapter viewer
- Cancelable scans, downloads, metadata reads, and remuxes
- Safe `*.partial.mkv` output followed by rename on success
- Pinned and SHA-256-verified FFmpeg and MediaInfo downloads
- Portable/offline bundle builder

### Downloads

- **MattMux-1.2.0-Setup.exe** — standard Windows installer
- **MattMux-1.2.0-Offline-Builder.zip** — portable builder that downloads and verifies the pinned FFmpeg/FFprobe and MediaInfo packages, then creates a fully offline bundle
- **SHA256SUMS.txt** — SHA-256 checksums for the release assets

### Build provenance

The GitHub release assets are built from the tagged source by GitHub Actions using Go 1.27.1. The binaries are not Authenticode-signed, so Windows may display a publisher/security warning.

MattMux does not bypass DVD copy protection such as CSS. Use it only with media you are authorized to process.
