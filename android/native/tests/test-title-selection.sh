#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
SOURCES="${1:?Pass directory containing pinned libdvdread, libdvdnav and libudfread sources}"
WORK="${2:?Pass output diagnostic directory}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
PREFIX="$WORK/tools"
JDK="${JAVA_HOME:-/usr/lib/jvm/java-17-openjdk-amd64}"
for lib in libdvdread libdvdnav; do
    mkdir -p "$WORK/build-$lib"
    (cd "$WORK/build-$lib"
     PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig" CFLAGS='-O2 -fPIC' \
       "$SOURCES/$lib/configure" --prefix="$PREFIX" --disable-shared --enable-static
     make -j2
     make install)
done
cc -shared -fPIC -std=c11 -D_POSIX_C_SOURCE=200809L -DHAVE_UNISTD_H=1 -DHAVE_FCNTL_H=1 -DMATTMUX_DVDNAV=1 \
  -Wall -Wextra -Werror=implicit-function-declaration \
  -I"$ROOT/android/native/tests/host/include" -I"$PREFIX/include" \
  -I"$SOURCES/libudfread/src" -I"$ROOT/android/native" -I"$JDK/include" -I"$JDK/include/linux" \
  "$SOURCES/libudfread/src/udfread.c" "$SOURCES/libudfread/src/ecma167.c" "$SOURCES/libudfread/src/default_blockinput.c" \
  "${TITLE_JNI_SOURCE:-$ROOT/android/native/mattmux_jni.c}" "$ROOT/android/native/udf_source.c" "$ROOT/android/native/tests/host_fd.c" \
  -L"$PREFIX/lib" -ldvdnav -ldvdread -ldl -lpthread -lm \
  -lavformat -lavcodec -lavutil -o "$WORK/libmattmux_titles.so"

# Distinct movies expose source reuse, and both title orders expose off-by-one indexing.
for name in long-first long-last single; do
    color=red
    tone=440
    if [ "$name" = long-last ]; then color=blue; tone=880; fi
    ffmpeg -v error -f lavfi -i "color=$color:size=720x576:rate=25" \
      -f lavfi -i "sine=frequency=$tone:sample_rate=48000" -t 6 -target pal-dvd -y "$WORK/$name-main.vob"
    ffmpeg -v error -f lavfi -i 'color=green:size=720x576:rate=25' \
      -f lavfi -i 'sine=frequency=220:sample_rate=48000' -t 2 -target pal-dvd -y "$WORK/$name-extra.vob"
    main="<pgc><vob file=\"$WORK/$name-main.vob\" chapters=\"0,3\" /></pgc>"
    extra="<pgc><vob file=\"$WORK/$name-extra.vob\" /></pgc>"
    titles="$main$extra"
    if [ "$name" = long-last ]; then titles="$extra$main"; fi
    if [ "$name" = single ]; then titles="$main"; fi
    cat > "$WORK/$name.xml" <<XML
<dvdauthor dest="$WORK/$name">
  <vmgm><menus><video format="pal" /></menus></vmgm>
  <titleset><titles><video format="pal" /><audio lang="en" />$titles</titles></titleset>
</dvdauthor>
XML
    dvdauthor -x "$WORK/$name.xml"
    genisoimage -quiet -dvd-video -udf -o "$WORK/$name.iso" "$WORK/$name"
done
javac -d "$WORK/classes" \
  "$ROOT/android/native/tests/java/io/github/maas3n/mattmux/AndroidNativeRemuxEngine.java" \
  "$ROOT/android/native/tests/java/io/github/maas3n/mattmux/DvdTitleRegression.java"
java -cp "$WORK/classes" io.github.maas3n.mattmux.DvdTitleRegression "$WORK/libmattmux_titles.so" "$WORK"
for name in long-first long-last single; do
    python3 "$ROOT/android/native/tests/remux_fingerprint.py" "$WORK/$name-folder.mkv" "$WORK/$name-iso.mkv"
done
python3 "$ROOT/android/native/tests/verify-main-movie.py" "$WORK"
