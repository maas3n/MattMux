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
- CI build for APK/AAB

Not yet implemented:

- Android-native LGPL-only FFmpeg/libav remux engine
- MattMux-owned DVD cell/navigation and ISO/UDF reading needed to avoid GPL DVD libraries
- source scanning/title selection through Android content URIs
- production purchase verification backend / Play Developer API validation
- production signing and Play Console upload

Purchases are deliberately disabled in `BuildConfig` until the native remux engine is functional. Do not enable charging users for an unfinished remux path.

The Android commercial build must not bundle `libdvdnav`, `libdvdread`, `libdvdcss`, GPL-enabled FFmpeg, or `--enable-nonfree` FFmpeg components. See `native/FFMPEG_LGPL_POLICY.md`.

## Google Play product

Create a one-time, non-consumable product in Play Console with product ID:

```text
mattmux_pro
```

Set the price in Play Console. The app reads and displays the localized Play price; there is no price hardcoded into the APK.

## Build

Requires JDK 17, Gradle 9.6.0, Android SDK platform 36, and build-tools 36.0.0.

```bash
gradle :app:assembleDebug
gradle :app:bundleRelease
```

Run the commands from this `android/` directory.
