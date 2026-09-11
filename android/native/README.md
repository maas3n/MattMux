# Android native remux engine

MattMux on Windows currently orchestrates external FFmpeg/FFprobe executables. The Google Play Android/ChromeOS build instead **bundles FFmpeg/libav as native shared libraries inside the APK/AAB**.

The current build uses FFmpeg 9.0.1 in an **LGPL-only** configuration for demuxing/muxing and MattMux-owned code for DVD structure/navigation. It must not bundle `libdvdnav`, `libdvdread`, `libdvdcss`, GPL-enabled FFmpeg components, or `--enable-nonfree` components.

`libavformat`, `libavcodec`, and `libavutil` are FFmpeg libraries; using the `libav*` APIs does not by itself change the license. The FFmpeg build configuration determines whether the resulting native libraries remain LGPL.

Native libraries are built for:

- `arm64-v8a`
- `x86_64`

The Kotlin frontend loads these libraries through `AndroidNativeRemuxEngine`, and `libmattmux_jni.so` provides the JNI boundary.

## Build the bundled runtime

The reproducible build entry point is:

```bash
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/30.0.16248370"
bash android/native/build-ffmpeg-android.sh
```

The script:

- downloads the official FFmpeg 9.0.1 source archive
- verifies SHA-256 `cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635`
- builds dynamically linked LGPL-only libraries with GPL/nonfree disabled
- explicitly disables FFmpeg's `dvdvideo` demuxer
- builds both Chromebook-relevant 64-bit ABIs
- normalizes versioned FFmpeg SONAME/DT_NEEDED names to Android-friendly `lib*.so`
- rejects forbidden GPL/DVD dependencies
- packages FFmpeg license/build-provenance files as app assets

The generated `.so` files are deliberately ignored by Git and are recreated in CI before Gradle builds the APK/AAB.

## DVD strategy without GPL libraries

MattMux will own the DVD-reading logic needed by the Android app:

- reuse/extend the existing conservative IFO parser for title, chapter, PGC, program, cell, angle, and timing metadata
- extend the parser to map selected title cells to the relevant VOB ranges
- feed selected MPEG program-stream data to FFmpeg/libav through custom I/O rather than asking libdvdnav/libdvdread to navigate the disc
- add an internal ISO/UDF reader for ISO sources so the app does not depend on a normal filesystem path or a GPL DVD library
- support unencrypted user-accessible DVD data only; do not add CSS circumvention/decryption code

Complex authoring that MattMux cannot interpret safely should fail clearly instead of silently selecting the wrong cells.

## LGPL build rule

The Android FFmpeg build must remain LGPL-only:

- never pass `--enable-gpl`
- never pass `--enable-nonfree`
- do not link GPL external libraries such as `libx264`, `libx265`, `libxvid`, `libdvdnav`, or `libdvdread`
- prefer the smallest stream-copy configuration needed for DVD MPEG-PS input and Matroska output
- dynamically link the FFmpeg libraries from the MattMux JNI layer
- preserve FFmpeg license notices and provide the source/build material required by the applicable LGPL version

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
- complete LGPL notices and source/build-material availability for the shipped FFmpeg version

Do not enable the paid `mattmux_pro` purchase button until the native engine passes real Chromebook tests.
