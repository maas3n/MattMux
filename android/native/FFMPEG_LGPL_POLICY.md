# MattMux Android FFmpeg LGPL policy

MattMux for Android/ChromeOS is intended to use FFmpeg's `libavformat`, `libavcodec`, and `libavutil` libraries in an **LGPL-only** configuration.

This file is an engineering policy, not legal advice. Before a commercial Play release, verify the exact shipped FFmpeg version, configure flags, linked libraries, notices, and source/relinking materials against that version's license files.

## Mandatory build constraints

A distributable MattMux Android native build must satisfy all of these:

1. FFmpeg is configured **without** `--enable-gpl`.
2. FFmpeg is configured **without** `--enable-nonfree`.
3. No GPL-only external library is linked into the shipped FFmpeg libraries.
4. MattMux does not bundle `libdvdnav`, `libdvdread`, or `libdvdcss`.
5. The build is stream-copy focused; do not add GPL encoders merely for convenience.
6. The exact FFmpeg source revision and configure command are recorded in the release provenance.
7. Required FFmpeg/LGPL copyright and license notices ship with the app or its accompanying legal notices.
8. Required LGPL source/relinking material for the shipped libraries is made available with each release.

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

The native build should expose a small API to Kotlin for:

- opening an Android content URI through custom I/O
- probing MPEG program streams
- enumerating streams
- stream-copy remuxing to Matroska
- writing chapters/metadata
- reporting progress
- cancellation

DVD title/cell selection stays outside FFmpeg in MattMux-owned code.

## Release gate

Billing must remain disabled until CI can prove the production native build uses the approved configuration and real Chromebook tests confirm correct remux output.
