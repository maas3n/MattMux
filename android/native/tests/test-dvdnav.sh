#!/usr/bin/env bash
# Exercise the production JNI scanner/planner against an authored two-title DVD.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SOURCES="${1:?Pass directory containing pinned libdvdread, libdvdnav and libudfread sources}"
WORK="${2:?Pass diagnostic output directory}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
PREFIX="$WORK/tools"
JDK="${JAVA_HOME:-/usr/lib/jvm/java-17-openjdk-amd64}"
test "$(git -C "$SOURCES/libdvdread" rev-parse HEAD)" = 0e020921726ee812e633959d9ad6315ff58b902b
test "$(git -C "$SOURCES/libdvdnav" rev-parse HEAD)" = 49f36c397a31e663d9e59b909379808a08b80b8f
for lib in libdvdread libdvdnav; do
    mkdir -p "$WORK/$lib"
    (cd "$WORK/$lib"
     PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig" CFLAGS='-O2 -fPIC' \
       "$SOURCES/$lib/configure" --prefix="$PREFIX" --disable-shared --enable-static
     make -j2 >/dev/null
     make install >/dev/null)
done
cc -shared -fPIC -std=c11 -D_POSIX_C_SOURCE=200809L -DHAVE_UNISTD_H=1 -DHAVE_FCNTL_H=1 -DMATTMUX_DVDNAV=1 \
  -Wall -Wextra -Werror=implicit-function-declaration \
  -I"$ROOT/android/native/tests/host/include" -I"$PREFIX/include" \
  -I"$SOURCES/libudfread/src" -I"$ROOT/android/native" -I"$JDK/include" -I"$JDK/include/linux" \
  "$SOURCES/libudfread/src/udfread.c" "$SOURCES/libudfread/src/ecma167.c" "$SOURCES/libudfread/src/default_blockinput.c" \
  "$ROOT/android/native/mattmux_jni.c" "$ROOT/android/native/udf_source.c" \
  "$ROOT/android/native/tests/host_fd.c" \
  -L"$PREFIX/lib" -ldvdnav -ldvdread -ldl -lpthread -lm \
  -lavformat -lavcodec -lavutil -o "$WORK/libmattmux_dvdnav_test.so"
for seconds in 1 3; do
    ffmpeg -v error -f lavfi -i 'testsrc2=size=720x576:rate=25' \
      -f lavfi -i 'sine=frequency=440:sample_rate=48000' -t "$seconds" -target pal-dvd -y "$WORK/$seconds.vob"
done
cat > "$WORK/disc.xml" <<XML
<dvdauthor dest="$WORK/disc">
  <vmgm><menus><video format="pal" /></menus></vmgm>
  <titleset><titles><video format="pal" /><audio lang="en" />
    <pgc><vob file="$WORK/1.vob" /></pgc>
    <pgc><vob file="$WORK/3.vob" chapters="0,1.5" /></pgc>
  </titles></titleset>
</dvdauthor>
XML
dvdauthor -x "$WORK/disc.xml"
mkdir -p "$WORK/staged/VIDEO_TS"
cp "$WORK/disc/VIDEO_TS/"*.IFO "$WORK/staged/VIDEO_TS/"
genisoimage -quiet -dvd-video -udf -o "$WORK/disc.iso" "$WORK/disc"
javac -d "$WORK/classes" "$ROOT/android/native/tests/java/io/github/maas3n/mattmux/AndroidNativeRemuxEngine.java"
java -cp "$WORK/classes" io.github.maas3n.mattmux.AndroidNativeRemuxEngine \
  "$WORK/libmattmux_dvdnav_test.so" "$WORK" dvdnav
python3 "$ROOT/android/native/tests/remux_fingerprint.py" "$WORK/folder.mkv" "$WORK/iso.mkv"
