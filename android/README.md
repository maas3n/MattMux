# MattMux for Android / ChromeOS

This directory contains the experimental Android/ChromeOS frontend for MattMux. Android/ChromeOS follows the unified MattMux release line. GitHub releases publish one persistently signed universal APK named `MattMux-<version>-Android.apk`.

Use `MattMux-<version>-Android.apk` on Android phones/tablets and on Chromebooks with Android app support. The same APK supports both Android and ChromeOS; no separate ChromeOS APK is required.

## Experimental status

Implemented:

- ChromeOS-compatible, resizable Android activity
- Storage Access Framework input/output pickers
- DVD-folder / `VIDEO_TS` input through document-tree providers
- read-only UDF ISO input through libudfread
- native libdvdnav 6.1.1 + libdvdread 6.1.3 title discovery and planning for folder and ISO sources
- longest-title selection
- Advanced Merger with stream, metadata and chapter selection
- GUI BATCH for movie/VIDEO_TS collections and unmounted ISO files
- in-app mattmux-cli: scan, metadata, remux and --batch; remux supports --streams and --no-chapters
- native libavformat/libavcodec/libavutil stream-copy remuxing to MKV
- chapter planning and MKV chapter output
- temporary `.partial` output with commit/abort handling
- cancellation and native progress callbacks
- arm64-v8a and x86_64 native runtimes
- LGPL-only FFmpeg 9.0.1 runtime built from pinned source
- separately linked LGPL libudfread 1.1.2
- Android unit tests, native host parity tests, APK/AAB package verification, and 16 KB page-size checks in CI

Current limitations / remaining gates:

- CSS or other DVD copy protection is not bypassed
- interleaved multi-angle discs are unsupported
- still/shuffle/multi-PGC semantics are not fully supported
- ISO input requires a seekable storage provider
- output providers must support random-access writing and rename
- ISO9660-only images and streaming-only providers are unsupported
- real-device Android and physical Chromebook testing remain release gates
- production Play purchase verification and rollout remain separate from the experimental GitHub APK
- v1.4.0 was debug-signed; v1.4.1 and later GitHub APKs use persistent distribution signing, so upgrading from v1.4.0 may require uninstalling the old APK first

Run and release status should be checked in CI; source implementation alone is not proof of a tested APK. Native test details are in [`native/tests/README.md`](native/tests/README.md).

## BATCH and CLI

The **BATCH** tab processes immediate `Movie/VIDEO_TS` folders and ISO files,
selecting the longest title and copying all streams and chapters into MKV.
Leave the optional output folder blank to write beside each source, or choose a
common destination. Existing output names are skipped. Cancel stops the batch.

The **CLI** tab runs `mattmux-cli` commands inside the app, using the same native
runtime and Android Storage Access Framework permissions as the GUI. Use the
pickers and presets to insert permitted `content://` URIs. This is an in-app
command interface, not a standalone executable installed in Termux or adb PATH.

```text
mattmux-cli scan SOURCE
mattmux-cli metadata --title 1 SOURCE
mattmux-cli remux --title 1 --streams 0,2 --output OUTPUT_ROOT SOURCE
mattmux-cli --batch MOVIES_ROOT OUTPUT_ROOT
```

`--title 0` or an omitted title selects the longest. `metadata` reports stream
indexes for `--streams`; omitting `--streams` copies all tracks. Chapters are
included unless `--no-chapters` is supplied. BATCH supports `--log=batch.log`
for an app-private log. A separate FFmpeg CLI is not required: remuxing and
Advanced Merger perform native libav stream-copy without transcoding.

The APK targets Android 8/API 26+ and includes both arm64-v8a and x86_64.
No libdvdcss or CSS decryption is included.

## Billing

The project contains a Google Play Billing integration and the non-consumable product ID `mattmux_pro`, but purchases are deliberately disabled in the current experimental build through `BuildConfig.ENABLE_BILLING_PURCHASES = false`.

Do not enable charging merely because the native remux engine now exists. Enable production purchases only after the remux path has passed real Chromebook/device testing and the production purchase-verification/signing plan is ready. See [`PLAY_CONSOLE.md`](PLAY_CONSOLE.md).

## Native runtime and licensing

`native/build-ffmpeg-android.sh` builds FFmpeg 9.0.1 as LGPL-only shared libraries, with libudfread 1.1.2 separately linked. libdvdnav 6.1.1 and libdvdread 6.1.3 are GPL libraries statically linked into the JNI runtime; their notices and corresponding source accompany the Android distribution. The complete Android runtime must not be described as LGPL-only.

Generated native libraries and FFmpeg provenance/license assets are build outputs and are intentionally not committed:

```text
app/src/main/jniLibs/
├── arm64-v8a/
│   ├── libavutil.so
│   ├── libavcodec.so
│   ├── libavformat.so
│   ├── libudfread.so
│   └── libmattmux_jni.so
└── x86_64/
    ├── libavutil.so
    ├── libavcodec.so
    ├── libavformat.so
    ├── libudfread.so
    └── libmattmux_jni.so
```

See [`native/FFMPEG_LGPL_POLICY.md`](native/FFMPEG_LGPL_POLICY.md) and the repository-level [`THIRD_PARTY.md`](../THIRD_PARTY.md) for provenance and licensing details.

## Build

Requirements used by CI:

- JDK 17
- Gradle 9.6.0
- Android SDK platform/build-tools 36
- Android NDK `30.0.16248370`
- `curl`, `xz`, `patchelf`, Autotools and the normal native build toolchain

From the repository root:

```bash
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/30.0.16248370"
bash android/native/build-ffmpeg-android.sh
gradle -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug :app:bundleRelease
```

## Google Play

The intended one-time product ID is:

```text
mattmux_pro
```

Price and availability belong in Play Console rather than in the APK. Follow [`PLAY_CONSOLE.md`](PLAY_CONSOLE.md) before enabling purchases or publishing a production Play build.
