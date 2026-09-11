## MattMux 1.2.0

MattMux is a native Windows utility for remuxing DVD-Video titles to MKV without transcoding.

### September 11, 2026 hotfix rebuild

The v1.2.0 Windows release was rebuilt after the release audit and its tag/assets were refreshed from the corrected source. The hotfix:

- gives every remux operation a unique operation-owned temporary MKV
- rejects zero-byte/non-regular remux output
- syncs the completed temporary output before commit
- commits the final MKV without replacing a destination created by another writer
- hardens native DVD chapter parsing against malformed angle-block structures while preserving FFmpeg/libdvdnav fallback behavior

The NTSC frame-timing formula was intentionally not changed by this hotfix because the audit found conflicting timing conventions and called for a separately verified timing contract first.

### Highlights

- DVD folder, `VIDEO_TS`, and ISO input support
- Lossless stream-copy remuxing to MKV
- Automatic longest-title selection after scanning
- Optional chapter preservation
- Native Go IFO chapter parsing with FFprobe fallback
- Metadata/chapter viewer
- Cancelable scans, metadata reads, and remuxes
- Unique temporary remux output with validated, no-overwrite finalization
- FFmpeg, FFprobe, and MediaInfo bundled in the installer and portable package
- Ready-to-run portable ZIP
- All-in-One EXE with Run MattMux and Install MattMux choices

### Downloads

- **MattMux-1.2.0-All-in-One.exe** — one-file launcher with Run MattMux and Install MattMux choices
- **MattMux-1.2.0-Setup.exe** — self-contained Windows installer with bundled runtime tools
- **MattMux-1.2.0-Portable.zip** — ready-to-run portable build with bundled runtime tools
- **SHA256SUMS.txt** — SHA-256 checksums for the release assets

### Build provenance

The refreshed GitHub release assets are built from the refreshed `v1.2.0` tag by GitHub Actions using Go 1.27.1. The workflow downloads the pinned third-party runtime archives, verifies their SHA-256 values, and embeds them into the installer, portable ZIP, and All-in-One launcher. The binaries are not Authenticode-signed, so Windows may display a publisher/security warning.

MattMux does not bypass DVD copy protection such as CSS. Use it only with media you are authorized to process.
