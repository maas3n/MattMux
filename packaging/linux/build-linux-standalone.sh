#!/usr/bin/env bash
set -euo pipefail

APP_VERSION="${1:-1.3.0-dev5}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DIST="$ROOT/dist/linux-release"
WORK="$ROOT/dist/linux-work"
LAUNCHER_SRC="$ROOT/packaging/linux/standalone/main.go"
STAGE="$WORK/standalone-src"
PAYLOAD="$STAGE/payload"
OUT="$DIST/MattMux-$APP_VERSION-Linux-amd64Standalone"

[[ "$(uname -s)" == "Linux" ]] || { echo "Standalone build requires Linux." >&2; exit 1; }
[[ "$(uname -m)" == "x86_64" ]] || { echo "Standalone build currently supports amd64/x86_64 only." >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "Go is required." >&2; exit 1; }

APP="$WORK/bin/mattmux-bin"
FFMPEG="$(find "$WORK/tools/ffmpeg" -type f -name ffmpeg -perm -u+x | head -n1 || true)"
FFPROBE="$(find "$WORK/tools/ffmpeg" -type f -name ffprobe -perm -u+x | head -n1 || true)"
MEDIAINFO="$WORK/tools/mediainfo-install/bin/mediainfo"

for f in "$LAUNCHER_SRC" "$APP" "$FFMPEG" "$FFPROBE" "$MEDIAINFO"; do
  [[ -f "$f" ]] || { echo "Required standalone payload is missing: $f" >&2; exit 1; }
done

rm -rf "$STAGE"
mkdir -p "$PAYLOAD"
install -m 0644 "$LAUNCHER_SRC" "$STAGE/main.go"
install -m 0755 "$APP" "$PAYLOAD/mattmux-bin"
install -m 0755 "$FFMPEG" "$PAYLOAD/ffmpeg"
install -m 0755 "$FFPROBE" "$PAYLOAD/ffprobe"
install -m 0755 "$MEDIAINFO" "$PAYLOAD/mediainfo"
cat > "$STAGE/go.mod" <<'EOF'
module mattmux-standalone

go 1.27.1
EOF

(
  cd "$STAGE"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags "-s -w -X main.appVersion=$APP_VERSION" \
    -o "$OUT" .
)
chmod 0755 "$OUT"

# Exercise extraction and every embedded tool without opening the GUI.
rm -rf "$WORK/standalone-test-cache"
XDG_CACHE_HOME="$WORK/standalone-test-cache" "$OUT" --standalone-self-test

# The standalone is a release asset, so include it in the same checksum file as
# the .deb and tarballs produced by build-linux-release.sh.
(
  cd "$DIST"
  sha256sum ./*.deb ./*.tar.gz ./*Standalone > SHA256SUMS.txt
)

echo "Standalone Linux executable:"
ls -lh "$OUT"
sha256sum "$OUT"
