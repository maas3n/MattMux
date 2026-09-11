# Changelog

## 1.4.1 — 2026-09-11

- Preserve completed desktop and Android remuxes when final publication fails, and add Linux no-overwrite publication fallbacks for filesystems without hard links.
- Use consistent 100M probe/analyze limits for desktop metadata probing and remuxing.
- Clean Linux CLI partial outputs on Ctrl+C/SIGTERM and add a process-level interrupt regression test.
- Correct Debian preview ordering and declare the measured `libc6 (>= 2.38)` runtime floor.
- Give additional DVD titles distinct `-title-NN` desktop output filenames.
- Sign GitHub Android APKs with persistent distribution credentials starting with 1.4.1 and reject debug certificates; users coming from debug-signed v1.4.0 may need to uninstall/reinstall.
- Carry DVD IFO language mappings and the selected PGC subtitle palette into Android native stream metadata.
- Align Android/release documentation and remove tracked Python bytecode.

MattMux uses one `main` source branch and one unified product-version namespace. Published builds are identified by immutable unified release tags.

## Android / ChromeOS 1.2.0 Alpha 4 — `v1.2.0-chromeos-alpha4`

- Added the Android/ChromeOS native remux path for DVD folders and read-only UDF ISO input.
- Added one IFO/title/cell planner for folder and ISO sources, longest-title selection, chapter planning, native stream-copy remuxing, cancellation, and progress reporting.
- Added arm64-v8a and x86_64 LGPL FFmpeg 9.0.1 + libudfread 1.1.2 runtimes with package/provenance verification.
- Added Android unit tests, host/native parity tests, APK/AAB validation, and 16 KB page-size checks.
- Google Play Billing integration is present, but purchases remain disabled in this alpha.

## Linux 1.3.0-dev5 — `v1.3.0-dev5`

- Added Linux GUI and CLI builds from the shared desktop source.
- Added portable amd64 tarball, self-contained `.deb`, source archive, and one-file standalone executable.
- Bundled FFmpeg/FFprobe and MediaInfo privately for the self-contained packages without replacing system multimedia tools.
- Pinned FFmpeg and MediaInfo-related sources/checksums for reproducible release construction.
- Added release-audit tests and safe no-overwrite output finalization.

## Windows 1.2.0 — `v1.2.0`

- Added a **Preserve chapters in the output MKV** checkbox; enabled by default and persisted.
- Added dependency-free native Go DVD IFO chapter detection for `VIDEO_TS` sources.
- Added automatic FFprobe `dvdvideo`/preindex fallback for ISO and unusual DVD authoring.
- Metadata window now lists detected chapter number, start timestamp, and duration.
- Chapter-preserving remuxes explicitly use FFmpeg `dvdvideo -preindex 1` and `-map_chapters 0`.
- Disabling chapter preservation explicitly uses `-map_chapters -1`.
- Added verified self-contained Setup, Portable and All-in-One packages.
- Added unique operation-owned temporary MKVs, validation/sync, and atomic no-overwrite finalization.

## Windows 1.1.0

- Rebuilt the Windows UI with clearer source/destination/title sections.
- Added DVD-folder and ISO-specific pickers plus drag-and-drop.
- Added cancellation for scans, downloads, metadata reads, and remuxes.
- Added longest-title auto-selection after scanning.
- Added stale-title protection when the source path changes after a scan.
- Added verified, pinned FFmpeg and MediaInfo downloads.
- Added safe ZIP extraction and protected output-folder checks.
- Added FFmpeg progress reporting.
- Kept partial-MKV then rename behavior for safer output.
- Added metadata viewer, persistent output settings, and local operation logs.
- Added DPI-aware external manifest and a dedicated sidecar app icon.
