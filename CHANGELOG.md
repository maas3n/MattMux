# Changelog

## 1.2.0

- Added a **Preserve chapters in the output MKV** checkbox; enabled by default and persisted.
- Added dependency-free native Go DVD IFO chapter detection for `VIDEO_TS` sources.
- Added automatic FFprobe `dvdvideo`/preindex fallback for ISO and unusual DVD authoring.
- Metadata window now lists detected chapter number, start timestamp, and duration.
- Chapter-preserving remuxes explicitly use FFmpeg `dvdvideo -preindex 1` and `-map_chapters 0`.
- Disabling chapter preservation explicitly uses `-map_chapters -1`.
- Kept FFmpeg as the only component that writes the final MKV chapter table.
- No new runtime dependency: mkvmerge, ChapterGrabber, and .NET are not required.

## 1.1.0

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
