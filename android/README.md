# Alpha 4 development status

The Android branch now implements folder and read-only UDF ISO input using one
IFO/title/cell planner and native stream-copy loop. ISO access uses a separate
LGPL libudfread shared library through a seekable SAF descriptor; no root, mount,
FUSE or VOB extraction is used. Source files are opened read-only.

Output stays under a temporary `.partial` name until the native muxer has
finished and flushed successfully. Cancellation/errors abort the output. A
provider must support random-access output and rename for this path.

Run and release status must be checked in CI; source implementation alone is
not proof of a tested APK. Generated-source and host-JNI checks are described in
[native/tests/README.md](native/tests/README.md). Full Windows/Linux output parity,
real Android execution and physical Chromebook tests remain separate gates.
Interleaved multi-angle discs, still/shuffle/multi-PGC semantics, CSS decryption,
ISO9660-only images and streaming-only providers are unsupported. Billing stays
disabled. Alpha 3's published tag and assets remain unchanged.

---

# MattMux for Android / Chromebook

This directory contains the Google Play / ChromeOS frontend for MattMux.

## Current milestone

Implemented:

- ChromeOS-compatible Android manifest (`android.hardware.type.pc` and touchscreen both optional)
- resizable desktop-style activity
- Storage Access Framework pickers for ISO/DVD folders and output folders
- Google Play Billing Library 9.1.0
- non-consumable `mattmux_pro` entitlement flow
- purchase restore/query and acknowledgement
- API 36 target/compile SDK for current Google Play submission requirements
- LGPL-only FFmpeg 9.0.1 shared runtime built from source for `arm64-v8a` and `x86_64`
- JNI runtime/version bridge proving the bundled native libraries load
- NDK r30 / modern AGP packaging for 16 KB page-size compatibility
- CI build and verification for APK/AAB

Not yet implemented:

- MattMux-owned DVD cell/navigation and ISO/UDF reading needed to avoid GPL DVD libraries
- actual libav custom-I/O stream-copy remuxing
- source scanning/title selection through Android content URIs
- production purchase verification backend / Play Developer API validation
- production signing and Play Console upload

Purchases are deliberately disabled in `BuildConfig` until the remux engine is functional. Do not enable charging users for an unfinished remux path.

The Android commercial build must not bundle `libdvdnav`, `libdvdread`, `libdvdcss`, GPL-enabled FFmpeg, or `--enable-nonfree` FFmpeg components. See `native/FFMPEG_LGPL_POLICY.md`.

## Bundled FFmpeg runtime

CI runs `native/build-ffmpeg-android.sh` before Gradle. The script downloads the pinned official FFmpeg source archive, verifies its SHA-256, builds LGPL-only shared libraries for both supported ABIs, normalizes their Android SONAMEs, and generates:

```text
app/src/main/jniLibs/
├── arm64-v8a/
│   ├── libavutil.so
│   ├── libavcodec.so
│   ├── libavformat.so
│   └── libmattmux_jni.so
└── x86_64/
    ├── libavutil.so
    ├── libavcodec.so
    ├── libavformat.so
    └── libmattmux_jni.so
```

These files are generated build artifacts and are intentionally not committed to Git. Gradle packages them into the APK/AAB, and Google Play serves the matching ABI to each device.

FFmpeg license text and build provenance are also generated under `app/src/main/assets/ffmpeg/` and packaged with the app.

## Google Play product

Create a one-time, non-consumable product in Play Console with product ID:

```text
mattmux_pro
```

Set the price in Play Console. The app reads and displays the localized Play price; there is no price hardcoded into the APK.

## Build

Requires JDK 17, Gradle 9.6.0, Android SDK platform/build-tools 36, Android NDK r30 (`30.0.16248370`), `curl`, `xz`, and `patchelf`.

From the repository root:

```bash
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/30.0.16248370"
bash android/native/build-ffmpeg-android.sh
gradle -p android :app:assembleDebug :app:bundleRelease
```
