#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")" && pwd)"
work="$root/.work/mediainfo"
assets="$root/../app/src/main/assets/ffmpeg"
mkdir -p "$work" "$assets"
checkout() {
  local name="$1" commit="$2"
  git init -q "$work/$name"
  git -C "$work/$name" remote add origin "https://github.com/MediaArea/$name.git"
  git -C "$work/$name" fetch --depth 1 origin "$commit"
  git -C "$work/$name" checkout --detach FETCH_HEAD
  test "$(git -C "$work/$name" rev-parse HEAD)" = "$commit"
  git -C "$work/$name" archive --format=tar.gz --prefix="$name/" HEAD > "$root/.work/$name-source.tar.gz"
  if [[ -f "$work/$name/License.html" ]]; then
    cp "$work/$name/License.html" "$assets/$name-License.html"
  else
    cp "$work/$name/License.txt" "$assets/$name-License.txt"
  fi
}
checkout MediaInfoLib 8bfa658657da9e16470c9fb32035e0fa097c0112
checkout ZenLib 2ddc277fe7ecfcbfe45616bb9cd9e23079113ecd
for abi in arm64-v8a x86_64; do
  cmake -S "$root/mediainfo" -B "$work/build-$abi" -G Ninja \
    -DCMAKE_TOOLCHAIN_FILE="$ANDROID_NDK_HOME/build/cmake/android.toolchain.cmake" \
    -DANDROID_ABI="$abi" -DANDROID_PLATFORM=android-26 -DANDROID_STL=c++_static \
    -DCMAKE_BUILD_TYPE=Release -DMEDIAINFO_SOURCE="$work/MediaInfoLib"
  cmake --build "$work/build-$abi" --parallel 2
  cp "$work/build-$abi/libmediainfo_jni.so" "$root/../app/src/main/jniLibs/$abi/"
done
printf '\nMediaInfoLib 26.05: 8bfa658657da9e16470c9fb32035e0fa097c0112\nZenLib: 2ddc277fe7ecfcbfe45616bb9cd9e23079113ecd\n' >> "$assets/ffmpeg-build-info.txt"
