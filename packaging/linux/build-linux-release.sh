#!/usr/bin/env bash
set -euo pipefail

APP_VERSION="${1:-1.3.0-dev2}"
DEB_VERSION="${APP_VERSION/-dev/~dev}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="$ROOT/src"
DIST="$ROOT/dist/linux-release"
WORK="$ROOT/dist/linux-work"

# Pinned third-party tools for the Debian package. They are installed below
# /usr/lib/mattmux and never replace distro executables in /usr/bin.
FFMPEG_TAG="autobuild-2026-09-08-23-15"
FFMPEG_ASSET="ffmpeg-N-126479-g08cd8df29d-linux64-gpl.tar.xz"
FFMPEG_SHA256="635a2d74de852064852e95db5a9c475a86d36e2b6390e3c1ba5e46b2c46dfce0"
FFMPEG_URL="https://github.com/BtbN/FFmpeg-Builds/releases/download/$FFMPEG_TAG/$FFMPEG_ASSET"
MEDIAINFO_TAG="v26.05"
MEDIAINFO_REPO="https://github.com/MediaArea/MediaInfo.git"

if [[ "$(uname -s)" != "Linux" ]]; then echo "This packaging script must run on Linux." >&2; exit 1; fi
if [[ "$(uname -m)" != "x86_64" ]]; then echo "The bundled toolchain is currently pinned for amd64/x86_64 only." >&2; exit 1; fi
for cmd in go git tar dpkg-deb sha256sum curl cmake ninja; do command -v "$cmd" >/dev/null 2>&1 || { echo "Missing build tool: $cmd" >&2; exit 1; }; done

rm -rf "$DIST" "$WORK"
mkdir -p "$DIST" "$WORK/bin" "$WORK/tools"

pushd "$SRC" >/dev/null
export CGO_ENABLED=1
go test -tags cli ./...
go vet -tags cli ./...
go build -trimpath -ldflags "-s -w -X main.appVersion=$APP_VERSION" -o "$WORK/bin/mattmux-bin" .
go build -tags cli -trimpath -ldflags "-s -w -X main.appVersion=$APP_VERSION" -o "$WORK/bin/mattmux-cli-bin" .
popd >/dev/null

# The portable tarball remains small and uses the normal MattMux runtime tool
# discovery/fallback behavior. The .deb below additionally embeds private tools.
PORTABLE="$WORK/MattMux-$APP_VERSION-Linux-amd64"
mkdir -p "$PORTABLE"
install -m 0755 "$WORK/bin/mattmux-bin" "$PORTABLE/mattmux"
install -m 0755 "$WORK/bin/mattmux-cli-bin" "$PORTABLE/mattmux-cli"
cat > "$PORTABLE/README-LINUX.txt" <<TXT
MattMux $APP_VERSION for Debian/Ubuntu Linux (amd64)

mattmux      Desktop GUI
mattmux-cli  Command-line interface

This portable archive checks ffmpeg, ffprobe, and mediainfo on PATH first.
System ffmpeg/ffprobe are used only when FFmpeg exposes the dvdvideo demuxer.
If system FFmpeg is missing or incompatible, MattMux can prepare its pinned,
SHA-256-verified FFmpeg fallback in the current user's cache.
MediaInfo is optional for the portable archive.

The .deb release additionally embeds private FFmpeg, FFprobe and MediaInfo
copies under /usr/lib/mattmux and does not overwrite distro tools.

MattMux does not bypass DVD copy protection such as CSS.
TXT
tar -C "$WORK" -czf "$DIST/MattMux-$APP_VERSION-Linux-amd64.tar.gz" "$(basename "$PORTABLE")"

# Download and verify the exact GPL FFmpeg build used as the private .deb
# fallback. dvdvideo requires a GPL-enabled FFmpeg build with libdvdnav/read.
FF_ARCHIVE="$WORK/tools/$FFMPEG_ASSET"
FF_EXTRACT="$WORK/tools/ffmpeg"
echo "Downloading pinned FFmpeg build for .deb bundle..."
curl --fail --location --retry 3 --proto '=https' --tlsv1.2 -o "$FF_ARCHIVE" "$FFMPEG_URL"
printf '%s  %s\n' "$FFMPEG_SHA256" "$FF_ARCHIVE" | sha256sum --check --strict
mkdir -p "$FF_EXTRACT"
tar -xJf "$FF_ARCHIVE" -C "$FF_EXTRACT"
BUNDLED_FFMPEG="$(find "$FF_EXTRACT" -type f -name ffmpeg -perm -u+x | head -n1)"
BUNDLED_FFPROBE="$(find "$FF_EXTRACT" -type f -name ffprobe -perm -u+x | head -n1)"
[[ -n "$BUNDLED_FFMPEG" && -n "$BUNDLED_FFPROBE" ]] || { echo "FFmpeg archive did not contain ffmpeg/ffprobe" >&2; exit 1; }
"$BUNDLED_FFMPEG" -hide_banner -demuxers 2>/dev/null | grep -q 'dvdvideo' || { echo "Pinned FFmpeg lacks dvdvideo demuxer" >&2; exit 1; }

# Build a pinned static-oriented MediaInfo CLI from its immutable release tag.
# MediaInfo's CMake project fetches/builds ZenLib and zlib when requested.
MI_SRC="$WORK/tools/MediaInfo"
MI_BUILD="$WORK/tools/mediainfo-build"
MI_INSTALL="$WORK/tools/mediainfo-install"
echo "Building MediaInfo $MEDIAINFO_TAG for .deb bundle..."
git clone --quiet --depth 1 --branch "$MEDIAINFO_TAG" "$MEDIAINFO_REPO" "$MI_SRC"
cmake -G Ninja \
  -D CMAKE_PREFIX_PATH="$MI_INSTALL" \
  -D CMAKE_INSTALL_PREFIX="$MI_INSTALL" \
  -D CMAKE_BUILD_TYPE=Release \
  -D BUILD_ZENLIB=ON \
  -D BUILD_ZLIB=ON \
  -D ZLIB_BUILD_SHARED=OFF \
  -D ZLIB_BUILD_TESTING=OFF \
  -S "$MI_SRC/Project/CMake/CLI" \
  -B "$MI_BUILD"
cmake --build "$MI_BUILD" --parallel
cmake --install "$MI_BUILD"
BUNDLED_MEDIAINFO="$MI_INSTALL/bin/mediainfo"
[[ -x "$BUNDLED_MEDIAINFO" ]] || { echo "MediaInfo build did not produce a CLI binary" >&2; exit 1; }
"$BUNDLED_MEDIAINFO" --Version | head -n 4

DEBROOT="$WORK/deb-root"
mkdir -p \
  "$DEBROOT/DEBIAN" \
  "$DEBROOT/usr/bin" \
  "$DEBROOT/usr/lib/mattmux/app" \
  "$DEBROOT/usr/lib/mattmux/ffmpeg-bin" \
  "$DEBROOT/usr/lib/mattmux/mediainfo-bin" \
  "$DEBROOT/usr/share/applications" \
  "$DEBROOT/usr/share/doc/mattmux"

install -m 0755 "$WORK/bin/mattmux-bin" "$DEBROOT/usr/lib/mattmux/app/mattmux-bin"
install -m 0755 "$WORK/bin/mattmux-cli-bin" "$DEBROOT/usr/lib/mattmux/app/mattmux-cli-bin"
install -m 0755 "$BUNDLED_FFMPEG" "$DEBROOT/usr/lib/mattmux/ffmpeg-bin/ffmpeg"
install -m 0755 "$BUNDLED_FFPROBE" "$DEBROOT/usr/lib/mattmux/ffmpeg-bin/ffprobe"
install -m 0755 "$BUNDLED_MEDIAINFO" "$DEBROOT/usr/lib/mattmux/mediainfo-bin/mediainfo"

# Launchers preserve the normal system PATH. A compatible system FFmpeg wins;
# otherwise only MattMux's private FFmpeg directory is prepended. The private
# MediaInfo directory is appended so an installed system mediainfo also wins.
cat > "$DEBROOT/usr/bin/mattmux" <<'LAUNCHER'
#!/bin/sh
set -eu
FFDIR=/usr/lib/mattmux/ffmpeg-bin
MIDIR=/usr/lib/mattmux/mediainfo-bin
if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1 && ffmpeg -hide_banner -demuxers 2>/dev/null | grep -q 'dvdvideo'; then
    PATH="$PATH:$MIDIR"
else
    PATH="$FFDIR:$PATH:$MIDIR"
fi
export PATH
exec /usr/lib/mattmux/app/mattmux-bin "$@"
LAUNCHER
chmod 0755 "$DEBROOT/usr/bin/mattmux"

cat > "$DEBROOT/usr/bin/mattmux-cli" <<'LAUNCHER'
#!/bin/sh
set -eu
FFDIR=/usr/lib/mattmux/ffmpeg-bin
MIDIR=/usr/lib/mattmux/mediainfo-bin
if command -v ffmpeg >/dev/null 2>&1 && command -v ffprobe >/dev/null 2>&1 && ffmpeg -hide_banner -demuxers 2>/dev/null | grep -q 'dvdvideo'; then
    PATH="$PATH:$MIDIR"
else
    PATH="$FFDIR:$PATH:$MIDIR"
fi
export PATH
exec /usr/lib/mattmux/app/mattmux-cli-bin "$@"
LAUNCHER
chmod 0755 "$DEBROOT/usr/bin/mattmux-cli"

cat > "$DEBROOT/DEBIAN/control" <<CONTROL
Package: mattmux
Version: $DEB_VERSION
Section: video
Priority: optional
Architecture: amd64
Maintainer: MattMux project <noreply@github.com>
Depends: libc6, libstdc++6, libgcc-s1, ca-certificates, libgl1, libx11-6, libxcursor1, libxrandr2, libxinerama1, libxi6, libxkbcommon0, libwayland-client0
Suggests: ffmpeg, mediainfo
Homepage: https://github.com/maas3n/MattMux
Description: Lossless DVD title remuxing to Matroska
 MattMux scans DVD-Video titles and remuxes the selected title to MKV without
 transcoding. This package installs both the MattMux desktop GUI and mattmux-cli,
 plus private bundled FFmpeg, FFprobe and MediaInfo fallbacks under
 /usr/lib/mattmux. Existing distro multimedia tools are never overwritten.
CONTROL

cat > "$DEBROOT/usr/share/applications/mattmux.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=MattMux
Comment=Lossless DVD title remuxing to Matroska
Exec=mattmux
Icon=video-x-generic
Terminal=false
Categories=AudioVideo;AudioVideoEditing;Utility;
Keywords=DVD;MKV;FFmpeg;Remux;
DESKTOP

cat > "$DEBROOT/usr/share/doc/mattmux/README.Debian" <<TXT
MattMux for Debian/Ubuntu
=========================

Commands installed by this package:
  /usr/bin/mattmux
  /usr/bin/mattmux-cli

Private bundled tools:
  /usr/lib/mattmux/ffmpeg-bin/ffmpeg
  /usr/lib/mattmux/ffmpeg-bin/ffprobe
  /usr/lib/mattmux/mediainfo-bin/mediainfo

MattMux DOES NOT install /usr/bin/ffmpeg, /usr/bin/ffprobe, or
/usr/bin/mediainfo. Existing distro installations are left untouched.

At startup the launchers test the system ffmpeg/ffprobe first. If the system
FFmpeg exposes the dvdvideo demuxer, those tools are preferred. Otherwise the
private MattMux FFmpeg/FFprobe pair is selected. System MediaInfo is preferred
when installed; the private MediaInfo copy is the fallback.

Use "mattmux-cli tools" to see the paths MattMux currently resolves.
TXT

cat > "$DEBROOT/usr/share/doc/mattmux/THIRD-PARTY-NOTICES" <<TXT
Third-party software bundled with MattMux $APP_VERSION
=====================================================

FFmpeg / FFprobe
----------------
Build provider: BtbN/FFmpeg-Builds
Build tag: $FFMPEG_TAG
Asset: $FFMPEG_ASSET
SHA-256: $FFMPEG_SHA256
FFmpeg source revision represented by the build: 08cd8df29d
Build/source information: https://github.com/BtbN/FFmpeg-Builds
FFmpeg project source: https://github.com/FFmpeg/FFmpeg

The bundled build is GPL-enabled because FFmpeg's dvdvideo demuxer requires
libdvdnav and libdvdread with GPL support. See FFmpeg and the build provider for
the applicable copyright notices, license texts, build configuration and source.

MediaInfo
---------
Version/tag: $MEDIAINFO_TAG
Source: https://github.com/MediaArea/MediaInfo
License: BSD-2-Clause (see MediaInfo-LICENSE in this directory).

MattMux keeps these programs private under /usr/lib/mattmux and does not claim
them as part of MattMux itself.
TXT
install -m 0644 "$MI_SRC/LICENSE" "$DEBROOT/usr/share/doc/mattmux/MediaInfo-LICENSE"

# Preserve any FFmpeg license/readme text distributed in the pinned build.
FF_LICENSE="$(find "$FF_EXTRACT" -type f \( -iname 'license*' -o -iname 'copying*' \) | head -n1 || true)"
if [[ -n "$FF_LICENSE" ]]; then install -m 0644 "$FF_LICENSE" "$DEBROOT/usr/share/doc/mattmux/FFmpeg-LICENSE"; fi

dpkg-deb --build --root-owner-group "$DEBROOT" "$DIST/mattmux_${DEB_VERSION}_amd64.deb" >/dev/null

# Source snapshot from the exact MattMux commit being built, plus the module
# metadata resolved by CI so the archive is immediately buildable.
SOURCE="$WORK/MattMux-$APP_VERSION-Source"
mkdir -p "$SOURCE"
git -C "$ROOT" archive HEAD | tar -x -C "$SOURCE"
if [[ -f "$SRC/go.mod" ]]; then cp "$SRC/go.mod" "$SOURCE/src/go.mod"; fi
if [[ -f "$SRC/go.sum" ]]; then cp "$SRC/go.sum" "$SOURCE/src/go.sum"; fi
tar -C "$WORK" -czf "$DIST/MattMux-$APP_VERSION-Source.tar.gz" "$(basename "$SOURCE")"

(
  cd "$DIST"
  sha256sum ./*.deb ./*.tar.gz > SHA256SUMS.txt
)

echo "Linux release artifacts:"
ls -lh "$DIST"
