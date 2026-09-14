#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
WORK="${SCRIPT_DIR}/.work"
JNI_ROOT="${REPO_ROOT}/android/app/src/main/jniLibs"
ASSET_ROOT="${REPO_ROOT}/android/app/src/main/assets/ffmpeg"
ANDROID_API="${ANDROID_API:-26}"
: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to the Android NDK directory.}"

DVDREAD_VERSION=6.1.3
DVDNAV_VERSION=6.1.1
READ_SRC="$WORK/libdvdread"
NAV_SRC="$WORK/libdvdnav"
rm -rf "$READ_SRC" "$NAV_SRC"
git clone --depth 1 --branch "$DVDREAD_VERSION" https://code.videolan.org/videolan/libdvdread.git "$READ_SRC"
git clone --depth 1 --branch "$DVDNAV_VERSION" https://code.videolan.org/videolan/libdvdnav.git "$NAV_SRC"
READ_COMMIT="$(git -C "$READ_SRC" rev-parse HEAD)"
NAV_COMMIT="$(git -C "$NAV_SRC" rev-parse HEAD)"
test "$READ_COMMIT" = 0e020921726ee812e633959d9ad6315ff58b902b
test "$NAV_COMMIT" = 49f36c397a31e663d9e59b909379808a08b80b8f
git -C "$READ_SRC" archive --format=tar --prefix="libdvdread-${DVDREAD_VERSION}/" HEAD | gzip -n > "$WORK/libdvdread-${DVDREAD_VERSION}-source.tar.gz"
git -C "$NAV_SRC" archive --format=tar --prefix="libdvdnav-${DVDNAV_VERSION}/" HEAD | gzip -n > "$WORK/libdvdnav-${DVDNAV_VERSION}-source.tar.gz"
cp "$READ_SRC/COPYING" "$ASSET_ROOT/DVDREAD_COPYING.txt"
cp "$NAV_SRC/COPYING" "$ASSET_ROOT/DVDNAV_COPYING.txt"
(cd "$READ_SRC" && autoreconf -fi)
(cd "$NAV_SRC" && autoreconf -fi)

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) HOST_TAG=linux-x86_64 ;;
  Darwin-x86_64|Darwin-arm64) HOST_TAG=darwin-x86_64 ;;
  *) echo "Unsupported build host" >&2; exit 1 ;;
esac
TOOLCHAIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$HOST_TAG"
AR="$TOOLCHAIN/bin/llvm-ar"
RANLIB="$TOOLCHAIN/bin/llvm-ranlib"
STRIP="$TOOLCHAIN/bin/llvm-strip"
NM="$TOOLCHAIN/bin/llvm-nm"

build_one() {
    local abi="$1" target="$2"
    local cc="$TOOLCHAIN/bin/${target}${ANDROID_API}-clang"
    local prefix="$WORK/install-${abi}"
    local jni="$JNI_ROOT/${abi}"
    local read_build="$WORK/build-dvdread-${abi}"
    local nav_build="$WORK/build-dvdnav-${abi}"
    rm -rf "$read_build" "$nav_build"
    mkdir -p "$read_build" "$nav_build"

    test -d "$prefix/include" && test -d "$jni"

    (
        cd "$read_build"
        CC="$cc" AR="$AR" RANLIB="$RANLIB" STRIP="$STRIP" \
          CFLAGS='-O2 -fPIC' LDFLAGS='-Wl,-z,max-page-size=16384' \
          "$READ_SRC/configure" --host="$target" --prefix="$prefix" --enable-static --disable-shared
        make -j2
        make install
    )

    (
        cd "$nav_build"
        CC="$cc" AR="$AR" RANLIB="$RANLIB" STRIP="$STRIP" \
          DVDREAD_CFLAGS="-I$prefix/include" DVDREAD_LIBS="-L$prefix/lib -ldvdread" \
          CFLAGS='-O2 -fPIC' LDFLAGS='-Wl,-z,max-page-size=16384' \
          "$NAV_SRC/configure" --host="$target" --prefix="$prefix" --enable-static --disable-shared
        make -j2
        make install
    )

    "$cc" \
        -shared -fPIC -O2 -DMATTMUX_DVDNAV=1 \
        -I"$prefix/include" -I"$prefix/include/udfread" \
        "$SCRIPT_DIR/mattmux_jni.c" "$SCRIPT_DIR/udf_source.c" \
        -L"$jni" -L"$prefix/lib" -Wl,--no-as-needed \
        -lavformat -lavcodec -lavutil -ludfread \
        -ldvdnav -ldvdread -ldl -lm \
        -Wl,-z,max-page-size=16384 -llog -Wl,--no-undefined \
        -Wl,-soname,libmattmux_jni.so -o "$jni/libmattmux_jni.so"

    while read -r dep; do
        case "$dep" in
            libavutil.so.*) patchelf --replace-needed "$dep" libavutil.so "$jni/libmattmux_jni.so" ;;
            libavcodec.so.*) patchelf --replace-needed "$dep" libavcodec.so "$jni/libmattmux_jni.so" ;;
            libavformat.so.*) patchelf --replace-needed "$dep" libavformat.so "$jni/libmattmux_jni.so" ;;
        esac
    done < <(patchelf --print-needed "$jni/libmattmux_jni.so")

    # llvm-nm can exit 74 on a closed stdout pipe when grep -q exits early.
    # Keep producer errors meaningful and inspect the complete symbol table.
    "$NM" "$jni/libmattmux_jni.so" > "$WORK/symbols-${abi}.txt"
    grep -q 'dvdnav_get_number_of_titles' "$WORK/symbols-${abi}.txt"
    grep -q 'dvdnav_describe_title_chapters' "$WORK/symbols-${abi}.txt"
    grep -q 'DVDOpen' "$WORK/symbols-${abi}.txt"
    patchelf --print-needed "$jni/libmattmux_jni.so" | grep -Eiq 'dvdcss' && {
        echo 'libdvdcss must not be bundled' >&2; exit 1;
    } || true
    "$STRIP" --strip-unneeded "$jni/libmattmux_jni.so"
}

build_one arm64-v8a aarch64-linux-android
build_one x86_64 x86_64-linux-android

cat >> "$ASSET_ROOT/ffmpeg-build-info.txt" <<EOF

DVD TITLE DISCOVERY
Title discovery: libdvdnav ${DVDNAV_VERSION} + libdvdread ${DVDREAD_VERSION}
Integration: GPL DVD libraries statically linked into libmattmux_jni.so for title discovery
libdvdread commit: ${READ_COMMIT}
libdvdnav commit: ${NAV_COMMIT}
CSS decryption/circumvention: not included
EOF

# The unified release workflow uploads every file already present in
# dist/android-release. Stage the GPL DVD source archives and notices here so
# future Android/ChromeOS releases automatically publish the corresponding
# source and license material alongside the APK.
RELEASE_STAGE="${REPO_ROOT}/dist/android-release"
mkdir -p "$RELEASE_STAGE"
cp "$WORK/libdvdread-${DVDREAD_VERSION}-source.tar.gz" "$RELEASE_STAGE/"
cp "$WORK/libdvdnav-${DVDNAV_VERSION}-source.tar.gz" "$RELEASE_STAGE/"
cp "$ASSET_ROOT/DVDREAD_COPYING.txt" "$RELEASE_STAGE/"
cp "$ASSET_ROOT/DVDNAV_COPYING.txt" "$RELEASE_STAGE/"
test -s "$RELEASE_STAGE/libdvdread-${DVDREAD_VERSION}-source.tar.gz"
test -s "$RELEASE_STAGE/libdvdnav-${DVDNAV_VERSION}-source.tar.gz"
test -s "$RELEASE_STAGE/DVDREAD_COPYING.txt"
test -s "$RELEASE_STAGE/DVDNAV_COPYING.txt"

echo "libdvdnav/libdvdread Android title scanner built successfully."
