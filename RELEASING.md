# Releasing MattMux

MattMux uses a single long-lived source branch: `main`. Release history is represented by Git tags and GitHub Releases, not by permanent platform branches.

## Release principles

1. **Published release tags are immutable.** Do not force-move an existing version tag to a newer commit.
2. **Published assets are historical artifacts.** If a released build needs a code or packaging fix, publish a new version instead of silently replacing the old release.
3. Build from the exact commit intended for the release and keep its tag permanently reachable.
4. Generate SHA-256 checksums for release payloads and verify packaged runtime dependencies before publishing.
5. Keep platform-specific release numbering while all source continues to integrate through `main`.

Current tag families are:

- Windows: `v1.2.x`
- Linux preview: `v1.3.0-devN`
- Android / ChromeOS preview: `v1.2.0-chromeos-alphaN`

## Before tagging

- Confirm Windows, Linux, and Android/ChromeOS CI is green for the source changes relevant to the release.
- Confirm version metadata and filenames match the new tag.
- Confirm pinned third-party versions/checksums and licensing/provenance documentation are current.
- Confirm the release workflow refuses to overwrite an existing tag/release.

## Windows

The verified `v1.2.0` release is historical and must not be rebuilt in place. The next Windows packaging/runtime correction should use a new version such as `v1.2.1`.

Windows packages should include:

- Setup EXE
- All-in-One EXE
- Portable ZIP
- `SHA256SUMS.txt`

Portable builds must use the bundled `MattMuxData` directory beside the executable rather than re-downloading tools into the installed-user data location.

## Linux

Linux release builds should pass an explicit version to both packaging scripts and publish their `.deb`, portable tarball, source archive, standalone executable, and checksum manifest under the matching release tag.

## Android / ChromeOS

Each alpha uses a new tag/versionCode/versionName. The release workflow must refuse to overwrite an existing historical alpha tag. Keep FFmpeg/libudfread source and licensing/provenance assets with the APK release.

Do not enable paid Play purchases until real-device/Chromebook validation and the production signing/purchase-verification plan are ready.

## Emergency fixes

If a release is defective after publication, leave its tag and existing historical assets intact, fix the problem on `main`, and publish the next version. Document the superseded release rather than rewriting history.
