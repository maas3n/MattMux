# Releasing MattMux

MattMux uses one long-lived source branch, `main`, and one product version namespace across Windows, Linux, and Android/ChromeOS.

## Unified release model

Every new public release uses exactly one product tag and one GitHub Release:

- stable: `vMAJOR.MINOR.PATCH` (for example `v1.4.0`)
- preview: `vMAJOR.MINOR.PATCH-alpha.N`, `-beta.N`, or `-rc.N` (for example `v1.4.0-alpha.1`)

Do not create new platform-specific version tags such as `-linux`, `-chromeos`, `-windows`, or separate `devN` release lines. Historical platform-specific tags remain valid historical pointers and are not rewritten.

A unified release contains the platform assets that are ready from the same tagged commit. Typical assets are:

- `MattMux-<version>-Windows-Setup.exe`
- `MattMux-<version>-Windows-All-in-One.exe`
- `MattMux-<version>-Windows-Portable.zip`
- `MattMux-<version>-Linux-amd64.deb`
- `MattMux-<version>-Linux-amd64.tar.gz`
- `MattMux-<version>-Linux-amd64Standalone`
- `MattMux-<version>-Source.tar.gz`
- `MattMux-<version>-ChromeOS.apk`
- third-party source/provenance/license files
- per-platform checksum manifests
- one combined `SHA256SUMS.txt`

GitHub also exposes source ZIP/tar archives automatically for the release tag.

## Release principles

1. **Published release tags are immutable.** Never force-move an existing version tag.
2. **Published release assets are immutable.** Fix a released problem in a new version.
3. **All platform payloads come from the same tag/commit.** Windows, Linux, and Android/ChromeOS must not publish different source commits under the same product version.
4. **One tag creates one GitHub Release.** Platform workflows may build independently, but `.github/workflows/release.yml` is the only workflow that publishes GitHub Releases.
5. Generate and verify SHA-256 checksums for release payloads and verify bundled runtime dependencies before publishing.
6. Keep historical development provenance in Git history; obsolete platform-specific public release entries may remain retired after the unified-release cleanup.

## Before tagging

- Merge the intended source into `main`.
- Confirm Windows, Linux, and Android/ChromeOS CI is green.
- Confirm pinned third-party versions/checksums and licensing/provenance documentation are current.
- Decide whether the release is stable (`v1.4.0`) or a shared preview (`v1.4.0-alpha.1`).
- Do not reuse a tag that already has a GitHub Release.

## Publishing

Create the tag from the exact `main` commit to publish and push it:

```bash
git switch main
git pull --ff-only
git tag v1.4.0
git push origin v1.4.0
```

For a preview:

```bash
git tag v1.4.0-alpha.1
git push origin v1.4.0-alpha.1
```

The **Unified release** workflow then:

1. validates the unified tag format;
2. derives one product version plus the Android `versionCode`;
3. builds the Windows payload;
4. builds the Linux payload;
5. builds the Android/ChromeOS APK;
6. verifies each platform payload;
7. downloads all platform artifacts into one release job;
8. creates a combined `SHA256SUMS.txt`; and
9. publishes one GitHub Release for that tag.

The workflow refuses to overwrite an existing GitHub Release.

The same workflow can be run manually for an **existing** unified tag by using `workflow_dispatch` and supplying that tag. Manual dispatch does not invent or move tags.

## Android / ChromeOS versionCode

The unified workflow derives a monotonically ordered Android versionCode from the product version:

- alpha builds sort before beta builds;
- beta builds sort before release candidates;
- release candidates sort before the stable release;
- the next patch/minor/major version sorts after the previous stable release.

Android/ChromeOS purchases remain disabled until production device validation, signing, and purchase-verification readiness are complete. The separate Play bundle workflow is distribution tooling; it does not create GitHub Releases.

GitHub release APKs after v1.4.0 use the persistent Android upload-signing credentials. The v1.4.0 APK was debug-signed, so an in-place upgrade from that APK must not be promised unless its original debug key is proven compatible. Release notes for the first persistently signed APK must call out the migration requirement. Preserve the same distribution key for subsequent APK releases and verify its certificate before publishing.

## Historical releases

Windows `v1.2.0`, Linux `v1.3.0-dev5`, and the old ChromeOS alpha line are historical development lines from before the unified release model. Their development remains in Git history; obsolete public release entries/tags were retired during the unified-release cleanup. Do not recreate platform-specific release lines. New releases use the unified version namespace only.

## Emergency fixes

If a published unified release is defective, leave its tag and assets unchanged, fix the problem on `main`, and publish the next product version (for example `v1.4.1`). Never rebuild an old release in place.
