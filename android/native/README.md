# Android native remux engine

MattMux on Windows currently orchestrates external FFmpeg/FFprobe executables. That design should not be copied directly to the Google Play Android build.

The Android/ChromeOS implementation should package FFmpeg/libavformat plus the required DVD navigation support as native libraries for at least:

- `arm64-v8a`
- `x86_64`

The Kotlin frontend talks to that native layer through JNI (or a small Go-mobile binding if the reusable MattMux core is moved into an importable Go package).

## Required behavior before enabling billing

- scan `VIDEO_TS`/DVD sources through Android Storage Access Framework URIs
- ISO access without assuming a normal filesystem path
- longest-title selection
- stream-copy MKV output with no transcoding
- chapter preservation
- cancellation
- `*.partial.mkv`-equivalent safe output semantics using a temporary document
- deterministic native dependency versions and reproducible checksums
- complete FFmpeg/libdvdnav license notices and corresponding-source obligations for the chosen build configuration

Do not enable the paid `mattmux_pro` purchase button until the native engine passes real Chromebook tests.
