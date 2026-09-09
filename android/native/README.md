# Android native remux engine

MattMux on Windows currently orchestrates external FFmpeg/FFprobe executables. That design should not be copied directly to the Google Play Android build.

The Android/ChromeOS implementation will use an **LGPL-only FFmpeg/libav\*** native build for demuxing/muxing and MattMux-owned code for DVD structure/navigation. It must not bundle `libdvdnav`, `libdvdread`, `libdvdcss`, GPL-enabled FFmpeg components, or `--enable-nonfree` components.

`libavformat`, `libavcodec`, and `libavutil` are FFmpeg libraries; using the `libav*` APIs does not by itself change the license. The FFmpeg build configuration determines whether the resulting native libraries remain LGPL.

Native libraries should be provided for at least:

- `arm64-v8a`
- `x86_64`

The Kotlin frontend talks to that native layer through JNI (or a small Go-mobile binding if the reusable MattMux core is moved into an importable Go package).

## DVD strategy without GPL libraries

MattMux will own the DVD-reading logic needed by the Android app:

- reuse/extend the existing conservative IFO parser for title, chapter, PGC, program, cell, angle, and timing metadata
- extend the parser to map selected title cells to the relevant VOB ranges
- feed the selected MPEG program-stream data to FFmpeg/libav through custom I/O rather than asking libdvdnav/libdvdread to navigate the disc
- add an internal ISO/UDF reader for ISO sources so the app does not depend on a normal filesystem path or a GPL DVD library
- support unencrypted user-accessible DVD data only; do not add CSS circumvention/decryption code

Complex authoring that MattMux cannot interpret safely should fail clearly instead of silently selecting the wrong cells.

## LGPL build rule

The Android FFmpeg build must remain LGPL-only:

- never pass `--enable-gpl`
- never pass `--enable-nonfree`
- do not link GPL external libraries such as `libx264`, `libx265`, `libxvid`, `libdvdnav`, or `libdvdread`
- prefer the smallest stream-copy configuration needed for DVD MPEG-PS input and Matroska output
- preserve FFmpeg license notices and provide the notices/source/relinking material required by the applicable LGPL version

See `FFMPEG_LGPL_POLICY.md` for the build/compliance guardrails.

## Required behavior before enabling billing

- scan `VIDEO_TS`/DVD sources through Android Storage Access Framework URIs
- ISO access without assuming a normal filesystem path
- longest-title selection
- stream-copy MKV output with no transcoding
- chapter preservation
- cancellation
- `*.partial.mkv`-equivalent safe output semantics using a temporary document
- deterministic native dependency versions and reproducible checksums
- complete LGPL notices and corresponding-source/relinking obligations for the chosen FFmpeg build configuration

Do not enable the paid `mattmux_pro` purchase button until the native engine passes real Chromebook tests.
