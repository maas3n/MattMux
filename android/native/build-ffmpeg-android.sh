#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
ANDROID_DIR="${REPO_ROOT}/android"
WORK_DIR="${SCRIPT_DIR}/.work"
JNI_ROOT="${ANDROID_DIR}/app/src/main/jniLibs"
ASSET_ROOT="${ANDROID_DIR}/app/src/main/assets/ffmpeg"

FFMPEG_VERSION="${FFMPEG_VERSION:-9.0.1}"
FFMPEG_SHA256="cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635"
FFMPEG_URL="https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"
ANDROID_API="${ANDROID_API:-26}"

if [[ "${FFMPEG_VERSION}" != "9.0.1" ]]; then
  echo "FFmpeg version changed. Update the pinned SHA-256 in this script first." >&2
  exit 1
fi

: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to the Android NDK directory.}"

for tool in curl git sha256sum tar make patchelf autoreconf; do
  command -v "${tool}" >/dev/null 2>&1 || {
    echo "Required build tool not found: ${tool}" >&2
    exit 1
  }
done

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64)
    HOST_TAG="linux-x86_64"
    ;;
  Darwin-x86_64|Darwin-arm64)
    HOST_TAG="darwin-x86_64"
    ;;
  *)
    echo "Unsupported build host: $(uname -s) $(uname -m)" >&2
    exit 1
    ;;
esac

TOOLCHAIN="${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/${HOST_TAG}"
if [[ ! -d "${TOOLCHAIN}" ]]; then
  echo "NDK LLVM toolchain was not found at ${TOOLCHAIN}" >&2
  exit 1
fi

AR="${TOOLCHAIN}/bin/llvm-ar"
NM="${TOOLCHAIN}/bin/llvm-nm"
RANLIB="${TOOLCHAIN}/bin/llvm-ranlib"
STRIP="${TOOLCHAIN}/bin/llvm-strip"

rm -rf "${WORK_DIR}" "${JNI_ROOT}" "${ASSET_ROOT}"
mkdir -p "${WORK_DIR}" "${JNI_ROOT}" "${ASSET_ROOT}"

ARCHIVE="${WORK_DIR}/ffmpeg-${FFMPEG_VERSION}.tar.xz"
curl --fail --location --retry 3 --output "${ARCHIVE}" "${FFMPEG_URL}"
echo "${FFMPEG_SHA256}  ${ARCHIVE}" | sha256sum -c -

tar -xf "${ARCHIVE}" -C "${WORK_DIR}"
SOURCE_DIR="${WORK_DIR}/ffmpeg-${FFMPEG_VERSION}"

# Official VideoLAN 1.1.2 release; verify the peeled commit, not only the tag.
UDFREAD_COMMIT=a35513813819efadca82c4b90edbe1407b1b9e05
UDF_SOURCE="${WORK_DIR}/libudfread"
git clone --depth 1 --branch 1.1.2 https://code.videolan.org/videolan/libudfread.git "${UDF_SOURCE}"
test "$(git -C "${UDF_SOURCE}" rev-parse HEAD)" = "${UDFREAD_COMMIT}"
git -C "${UDF_SOURCE}" archive --format=tar --prefix=libudfread-1.1.2/ HEAD | gzip -n > "${WORK_DIR}/libudfread-1.1.2-source.tar.gz"
cp "${UDF_SOURCE}/COPYING" "${ASSET_ROOT}/LIBUDFREAD_COPYING.txt"
(cd "${UDF_SOURCE}" && autoreconf -fi)


cp "${SOURCE_DIR}/COPYING.LGPLv2.1" "${ASSET_ROOT}/COPYING.LGPLv2.1"
cp "${SOURCE_DIR}/LICENSE.md" "${ASSET_ROOT}/FFMPEG_LICENSE.md"

NDK_REVISION="$(awk -F= '/Pkg.Revision/ {gsub(/[[:space:]]/, "", $2); print $2}' \
  "${ANDROID_NDK_HOME}/source.properties")"

cat > "${ASSET_ROOT}/ffmpeg-build-info.txt" <<EOF
MattMux Android native runtime
FFmpeg version: ${FFMPEG_VERSION}
FFmpeg source: ${FFMPEG_URL}
FFmpeg source SHA-256: ${FFMPEG_SHA256}
Android NDK revision: ${NDK_REVISION}
Android minimum native API: ${ANDROID_API}
License mode: LGPL-only dynamic libraries
GPL enabled: no
nonfree enabled: no
Bundled GPL DVD libraries: none
UDF reader: libudfread 1.1.2, LGPL-2.1-or-later, separate shared library
UDF source: https://code.videolan.org/videolan/libudfread
UDF commit: ${UDFREAD_COMMIT}
Supported ABIs: arm64-v8a, x86_64

The FFmpeg shared libraries are built from the unmodified upstream source archive.
Their Android-friendly SONAME/DT_NEEDED names are normalized after linking so
Google Play can package them as lib*.so files.
EOF

COMMON_FLAGS=(
  --target-os=android
  --enable-cross-compile
  --enable-pic
  --enable-shared
  --disable-static
  --disable-programs
  --disable-doc
  --disable-debug
  --disable-autodetect
  --disable-network
  --disable-avdevice
  --disable-avfilter
  --disable-swscale
  --disable-swresample
  --disable-encoders
  --disable-decoders
  --disable-hwaccels
  --disable-filters
  --disable-demuxer=dvdvideo
  --disable-x86asm
  --disable-symver
  --pkg-config=/bin/false
)

normalize_needed() {
  local file="$1"
  local dep
  while IFS= read -r dep; do
    case "${dep}" in
      libavutil.so.*)
        patchelf --replace-needed "${dep}" libavutil.so "${file}"
        ;;
      libavcodec.so.*)
        patchelf --replace-needed "${dep}" libavcodec.so "${file}"
        ;;
      libavformat.so.*)
        patchelf --replace-needed "${dep}" libavformat.so "${file}"
        ;;
    esac
  done < <(patchelf --print-needed "${file}")
}

copy_android_shared_library() {
  local prefix="$1"
  local abi="$2"
  local base="$3"
  local source_link="${prefix}/lib/lib${base}.so"
  local destination="${JNI_ROOT}/${abi}/lib${base}.so"

  if [[ ! -e "${source_link}" ]]; then
    echo "Expected FFmpeg library was not built: ${source_link}" >&2
    exit 1
  fi

  cp "$(readlink -f "${source_link}")" "${destination}"
  patchelf --set-soname "lib${base}.so" "${destination}"
  normalize_needed "${destination}"
}

build_abi() {
  local abi="$1"
  local arch="$2"
  local target="$3"
  local build_dir="${WORK_DIR}/build-${abi}"
  local prefix="${WORK_DIR}/install-${abi}"
  local jni_dir="${JNI_ROOT}/${abi}"
  local cc="${TOOLCHAIN}/bin/${target}${ANDROID_API}-clang"
  local cxx="${TOOLCHAIN}/bin/${target}${ANDROID_API}-clang++"

  mkdir -p "${build_dir}" "${prefix}" "${jni_dir}"

  local abi_flags=(
    --prefix="${prefix}"
    --arch="${arch}"
    --cc="${cc}"
    --cxx="${cxx}"
    --ar="${AR}"
    --nm="${NM}"
    --ranlib="${RANLIB}"
    --strip="${STRIP}"
    --sysroot="${TOOLCHAIN}/sysroot"
  )

  (
    cd "${build_dir}"

    printf '%q ' "${SOURCE_DIR}/configure" "${COMMON_FLAGS[@]}" "${abi_flags[@]}" \
      > "${ASSET_ROOT}/configure-${abi}.txt"
    printf '\n' >> "${ASSET_ROOT}/configure-${abi}.txt"

    "${SOURCE_DIR}/configure" "${COMMON_FLAGS[@]}" "${abi_flags[@]}"

    grep -q '^#define CONFIG_GPL 0$' config.h
    grep -q '^#define CONFIG_NONFREE 0$' config.h

    if grep -Eq '^#define CONFIG_(LIBDVDNAV|LIBDVDREAD|LIBDVDCSS) 1$' config.h; then
      echo "Forbidden DVD library was detected in the FFmpeg configuration." >&2
      exit 1
    fi

    if grep -q '^#define CONFIG_DVDVIDEO_DEMUXER 1$' config.h; then
      echo "FFmpeg dvdvideo demuxer must remain disabled in the commercial build." >&2
      exit 1
    fi

    make -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 2)"
    make install
  )

  copy_android_shared_library "${prefix}" "${abi}" avutil
  copy_android_shared_library "${prefix}" "${abi}" avcodec
  copy_android_shared_library "${prefix}" "${abi}" avformat

  mkdir -p "${WORK_DIR}/udf-${abi}"
  (
    cd "${WORK_DIR}/udf-${abi}"
    CC="${cc}" AR="${AR}" RANLIB="${RANLIB}" STRIP="${STRIP}" \
      CFLAGS="-O2 -fPIC" LDFLAGS="-Wl,-z,max-page-size=16384" \
      "${UDF_SOURCE}/configure" --host="${target}" --prefix="${prefix}" --enable-shared --disable-static
    make -j2
    make install
  )
  cp "$(readlink -f "${prefix}/lib/libudfread.so")" "${jni_dir}/libudfread.so"
  patchelf --set-soname libudfread.so "${jni_dir}/libudfread.so"

  "${cc}" \
    -shared \
    -fPIC \
    -O2 \
    -I"${prefix}/include" \
    -I"${prefix}/include/udfread" \
    "${SCRIPT_DIR}/mattmux_jni.c" \
    "${SCRIPT_DIR}/udf_source.c" \
    -L"${jni_dir}" \
    -Wl,--no-as-needed \
    -lavformat \
    -lavcodec \
    -lavutil \
    -ludfread \
    -Wl,-z,max-page-size=16384 \
    -llog \
    -Wl,--no-undefined \
    -Wl,-soname,libmattmux_jni.so \
    -o "${jni_dir}/libmattmux_jni.so"

  normalize_needed "${jni_dir}/libmattmux_jni.so"

  local file
  for file in "${jni_dir}"/*.so; do
    if patchelf --print-needed "${file}" | grep -Eq '^libav[^ ]+\.so\.'; then
      echo "Versioned FFmpeg DT_NEEDED entry remains in ${file}" >&2
      exit 1
    fi
    if patchelf --print-needed "${file}" | grep -Eiq '(dvdnav|dvdread|dvdcss|x264|x265|xvid)'; then
      echo "Forbidden GPL/non-approved dependency found in ${file}" >&2
      exit 1
    fi
    "${STRIP}" --strip-unneeded "${file}"
  done

  {
    echo
    echo "[${abi}]"
    echo "architecture: ${arch}"
    echo "compiler: ${cc}"
    for file in "${jni_dir}"/*.so; do
      echo "$(basename "${file}"):"
      patchelf --print-needed "${file}" | sed 's/^/  needs: /'
      sha256sum "${file}" | sed 's/^/  sha256: /'
    done
  } >> "${ASSET_ROOT}/ffmpeg-build-info.txt"
}

build_abi arm64-v8a aarch64 aarch64-linux-android
build_abi x86_64 x86_64 x86_64-linux-android

echo "Bundled LGPL FFmpeg runtime built successfully."
