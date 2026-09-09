#!/usr/bin/env bash
set -euo pipefail

APP_VERSION="${1:-1.3.0-dev1}"
DEB_VERSION="${APP_VERSION/-dev/~dev}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SRC="$ROOT/src"
DIST="$ROOT/dist/linux-release"
WORK="$ROOT/dist/linux-work"

if [[ "$(uname -s)" != "Linux" ]]; then echo "This packaging script must run on Linux." >&2; exit 1; fi
if [[ "$(uname -m)" != "x86_64" ]]; then echo "The managed FFmpeg fallback is currently pinned for amd64/x86_64 only." >&2; exit 1; fi
for cmd in go git tar dpkg-deb sha256sum; do command -v "$cmd" >/dev/null 2>&1 || { echo "Missing build tool: $cmd" >&2; exit 1; }; done

rm -rf "$DIST" "$WORK"
mkdir -p "$DIST" "$WORK/bin"

pushd "$SRC" >/dev/null
export CGO_ENABLED=1
go test -tags cli ./...
go vet -tags cli ./...
go build -trimpath -ldflags "-s -w -X main.appVersion=$APP_VERSION" -o "$WORK/bin/mattmux" .
go build -tags cli -trimpath -ldflags "-s -w -X main.appVersion=$APP_VERSION" -o "$WORK/bin/mattmux-cli" .
popd >/dev/null

PORTABLE="$WORK/MattMux-$APP_VERSION-Linux-amd64"
mkdir -p "$PORTABLE"
install -m 0755 "$WORK/bin/mattmux" "$PORTABLE/mattmux"
install -m 0755 "$WORK/bin/mattmux-cli" "$PORTABLE/mattmux-cli"
cat > "$PORTABLE/README-LINUX.txt" <<TXT
MattMux $APP_VERSION for Debian/Ubuntu Linux (amd64)

mattmux      Desktop GUI
mattmux-cli  Command-line interface

MattMux checks ffmpeg, ffprobe, and mediainfo already installed on PATH first.
System ffmpeg/ffprobe are used only when FFmpeg exposes the dvdvideo demuxer.
If system FFmpeg is missing or incompatible, MattMux downloads its pinned,
SHA-256-verified FFmpeg fallback into the current user's cache.
MediaInfo is optional; metadata viewing works without its extra report section.

MattMux does not bypass DVD copy protection such as CSS.
TXT
tar -C "$WORK" -czf "$DIST/MattMux-$APP_VERSION-Linux-amd64.tar.gz" "$(basename "$PORTABLE")"

DEBROOT="$WORK/deb-root"
mkdir -p "$DEBROOT/DEBIAN" "$DEBROOT/usr/bin" "$DEBROOT/usr/share/applications" "$DEBROOT/usr/share/doc/mattmux"
install -m 0755 "$WORK/bin/mattmux" "$DEBROOT/usr/bin/mattmux"
install -m 0755 "$WORK/bin/mattmux-cli" "$DEBROOT/usr/bin/mattmux-cli"
cat > "$DEBROOT/DEBIAN/control" <<CONTROL
Package: mattmux
Version: $DEB_VERSION
Section: video
Priority: optional
Architecture: amd64
Maintainer: MattMux project <noreply@github.com>
Depends: libc6, ca-certificates, libgl1, libx11-6, libxcursor1, libxrandr2, libxinerama1, libxi6, libxkbcommon0, libwayland-client0
Recommends: ffmpeg, mediainfo
Homepage: https://github.com/maas3n/MattMux
Description: Lossless DVD title remuxing to Matroska
 MattMux scans DVD-Video titles and remuxes the selected title to MKV without
 transcoding. This package installs both the MattMux desktop GUI and mattmux-cli.
 Compatible system FFmpeg/FFprobe are preferred; a verified fallback is used
 when the installed FFmpeg does not expose the dvdvideo demuxer.
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

Use "mattmux-cli tools" to inspect the installed FFmpeg/FFprobe/MediaInfo tools.
The package recommends distro ffmpeg and mediainfo. MattMux checks capability at
runtime and uses a pinned verified FFmpeg fallback only when needed.
TXT

dpkg-deb --build --root-owner-group "$DEBROOT" "$DIST/mattmux_${DEB_VERSION}_amd64.deb" >/dev/null

# Reproducible source snapshot from the exact commit being built.
git -C "$ROOT" archive --format=tar.gz --prefix="MattMux-$APP_VERSION-Source/" -o "$DIST/MattMux-$APP_VERSION-Source.tar.gz" HEAD

(
  cd "$DIST"
  sha256sum ./*.deb ./*.tar.gz > SHA256SUMS.txt
)

echo "Linux release artifacts:"
ls -lh "$DIST"
