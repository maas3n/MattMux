# MattMux Android FFmpeg LGPL policy

MattMux for Android/ChromeOS uses FFmpeg's `libavformat`, `libavcodec`, and `libavutil` libraries in an **LGPL-only, dynamically linked** configuration.

This file is an engineering policy, not legal advice. Before a commercial Play release, verify the exact shipped FFmpeg version, configure flags, linked libraries, notices, and source/build materials against that version's license files.

## Pinned production baseline

- FFmpeg: `9.0.1`
- upstream source: `https://ffmpeg.org/releases/ffmpeg-9.0.1.tar.xz`
- source SHA-256: `cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635`
- Android NDK: `30.0.16248370` (r30)
- ABIs: `arm64-v8a`, `x86_64`
- linkage: FFmpeg shared libraries loaded by the MattMux JNI layer

Changing any of these requires re-reviewing the generated build configuration and release notices.

## Mandatory build constraints

A distributable MattMux Android native build must satisfy all of these:

1. FFmpeg is configured **without** `--enable-gpl`.
2. FFmpeg is configured **without** `--enable-nonfree`.
3. No GPL-only external library is linked into the shipped FFmpeg libraries.
4. MattMux does not bundle `libdvdnav`, `libdvdread`, or `libdvdcss`.
5. FFmpeg's `dvdvideo` demuxer is disabled; DVD navigation stays in MattMux-owned code.
6. The build is stream-copy focused; do not add GPL encoders merely for convenience.
7. The exact FFmpeg source revision/archive hash and configure command are recorded in release provenance.
8. Required FFmpeg/LGPL copyright and license notices ship with the app.
9. The applicable FFmpeg source and build/relinking material required by the LGPL is made available with each commercial release.
10. CI must reject versioned/forbidden native dependencies before the APK/AAB is publishable.

## Forbidden examples

Do not enable or link these in the Android commercial build:

- `--enable-gpl`
- `--enable-nonfree`
- `libx264`
- `libx265`
- `libxvid`
- `libdvdnav`
- `libdvdread`
- `libdvdcss`

This list is intentionally conservative and is not exhaustive. Any new native dependency must have its license reviewed before it enters the production build.

## Intended native surface

Use only the FFmpeg/libav pieces MattMux needs for remuxing DVD program streams into Matroska without transcoding, plus MattMux-owned code for DVD structure and ISO/UDF access.

The native build exposes a small JNI surface to Kotlin. The first implemented call reports the loaded FFmpeg/libav versions; the next implementation stage will add:

- opening Android content URIs through custom I/O
- probing MPEG program streams
- enumerating streams
- stream-copy remuxing to Matroska
- writing chapters/metadata
- reporting progress
- cancellation

DVD title/cell selection stays outside FFmpeg in MattMux-owned code.

## Release gate

Billing must remain disabled until CI proves the production native build uses the approved configuration and real Chromebook tests confirm correct remux output.
