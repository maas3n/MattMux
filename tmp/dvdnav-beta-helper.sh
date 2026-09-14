#!/usr/bin/env bash
set -euo pipefail

: "${ANDROID_HOME:?ANDROID_HOME is required}"
: "${ANDROID_NDK_VERSION:=30.0.16248370}"

export ANDROID_NDK_HOME="${ANDROID_HOME}/ndk/${ANDROID_NDK_VERSION}"

# Start from the exact Android native runtime used by v1.4.2.
bash android/native/build-ffmpeg-android.sh

cat > android/app/src/main/java/io/github/maas3n/mattmux/DvdNavDebugScanner.kt <<'KOTLIN'
package io.github.maas3n.mattmux

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.DocumentsContract.Document
import java.io.File
import java.util.Locale

internal object DvdNavDebugScanner {
    private data class Entry(val name: String, val documentId: String, val mimeType: String)

    fun stageTreeIfos(context: Context, treeUri: Uri): File {
        val resolver = context.contentResolver
        val rootId = DocumentsContract.getTreeDocumentId(treeUri)
        val rootChildren = listChildren(context, treeUri, rootId)
        val videoTsId = if (rootChildren.any { it.name.equals("VIDEO_TS.IFO", true) }) {
            rootId
        } else {
            rootChildren.firstOrNull {
                it.name.equals("VIDEO_TS", true) && it.mimeType == Document.MIME_TYPE_DIR
            }?.documentId ?: error("VIDEO_TS folder was not found in the selected tree")
        }

        val stageRoot = createStageRoot(context)
        val videoTs = File(stageRoot, "VIDEO_TS").apply { mkdirs() }
        var copied = 0
        for (entry in listChildren(context, treeUri, videoTsId)) {
            val upper = entry.name.uppercase(Locale.ROOT)
            val keep = upper == "VIDEO_TS.IFO" || upper == "VIDEO_TS.BUP" ||
                Regex("VTS_[0-9]{2}_0\\.(IFO|BUP)").matches(upper)
            if (!keep) continue
            val uri = DocumentsContract.buildDocumentUriUsingTree(treeUri, entry.documentId)
            resolver.openInputStream(uri)?.use { input ->
                File(videoTs, upper).outputStream().use { output -> input.copyTo(output) }
            } ?: error("Could not read ${entry.name}")
            copied++
        }
        require(File(videoTs, "VIDEO_TS.IFO").isFile) { "VIDEO_TS.IFO is missing" }
        require(copied > 0) { "No DVD IFO metadata could be staged" }
        return stageRoot
    }

    fun stageIsoIfos(context: Context, vmg: ByteArray, loader: (Int) -> ByteArray?): File {
        val stageRoot = createStageRoot(context)
        val videoTs = File(stageRoot, "VIDEO_TS").apply { mkdirs() }
        File(videoTs, "VIDEO_TS.IFO").writeBytes(vmg)
        for (titleSet in 1..99) {
            val data = loader(titleSet) ?: continue
            File(videoTs, "VTS_%02d_0.IFO".format(Locale.ROOT, titleSet)).writeBytes(data)
        }
        return stageRoot
    }

    private fun createStageRoot(context: Context): File {
        val root = File(context.cacheDir, "mattmux-dvdnav-debug-${System.nanoTime()}")
        require(root.mkdirs()) { "Could not create libdvdnav DEBUG staging directory" }
        return root
    }

    private fun listChildren(context: Context, treeUri: Uri, parentId: String): List<Entry> {
        val childrenUri = DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, parentId)
        return context.contentResolver.query(
            childrenUri,
            arrayOf(Document.COLUMN_DISPLAY_NAME, Document.COLUMN_DOCUMENT_ID, Document.COLUMN_MIME_TYPE),
            null,
            null,
            null,
        )?.use { cursor ->
            val nameCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DISPLAY_NAME)
            val idCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DOCUMENT_ID)
            val mimeCol = cursor.getColumnIndexOrThrow(Document.COLUMN_MIME_TYPE)
            buildList {
                while (cursor.moveToNext()) {
                    add(Entry(cursor.getString(nameCol), cursor.getString(idCol), cursor.getString(mimeCol)))
                }
            }
        } ?: error("Selected document provider did not return directory contents")
    }
}
KOTLIN

python3 - <<'PY'
from pathlib import Path

p = Path('android/app/src/main/java/io/github/maas3n/mattmux/DvdIfoParser.kt')
s = p.read_text()
marker = '    private fun parseLocations(vmg: ByteArray): List<Location> {'
insert = '''    fun selectTitle(vmg: ByteArray, globalTitle: Int, vtsLoader: (Int) -> ByteArray?): DvdTitlePlan {
        requireMagic(vmg, "DVDVIDEO-VMG")
        val location = parseLocations(vmg).firstOrNull { it.global == globalTitle }
            ?: throw IllegalArgumentException("DVD title $globalTitle is outside the VMG title table")
        val vts = vtsLoader(location.vts)
            ?: error("VTS_%02d_0.IFO is missing".format(location.vts))
        return buildPlan(location, vts)
    }

'''
if marker not in s:
    raise SystemExit('DvdIfoParser insertion marker not found')
s = s.replace(marker, insert + marker, 1)
p.write_text(s)

p = Path('android/app/src/main/java/io/github/maas3n/mattmux/DvdDocumentSource.kt')
s = p.read_text()
old = '    fun openLongestTitle(): OpenTitle {'
new = '    fun openLongestTitle(): OpenTitle = openTitle(null)\n\n    fun openTitle(globalTitle: Int?): OpenTitle {'
if old not in s:
    raise SystemExit('DvdDocumentSource method marker not found')
s = s.replace(old, new, 1)
old = '''        val plan = DvdIfoParser.selectLongestTitle(vmg) { titleSet ->
            byName[String.format(Locale.ROOT, "VTS_%02d_0.IFO", titleSet)]?.let(::readEntry)
        }
'''
new = '''        val vtsLoader: (Int) -> ByteArray? = { titleSet ->
            byName[String.format(Locale.ROOT, "VTS_%02d_0.IFO", titleSet)]?.let(::readEntry)
        }
        val plan = if (globalTitle == null) {
            DvdIfoParser.selectLongestTitle(vmg, vtsLoader)
        } else {
            DvdIfoParser.selectTitle(vmg, globalTitle, vtsLoader)
        }
'''
if old not in s:
    raise SystemExit('DvdDocumentSource plan block not found')
s = s.replace(old, new, 1)
p.write_text(s)

p = Path('android/app/src/main/java/io/github/maas3n/mattmux/RemuxEngine.kt')
s = p.read_text()
start = s.find('    private fun openTitle(context: Context, uri: Uri): NativeTitle {')
end = s.find('\n    @Suppress("unused")\n    private fun isNativeCancelled()', start)
if start < 0 or end < 0:
    raise SystemExit('RemuxEngine openTitle block not found')
replacement = '''    private fun scanWithDvdNav(stageRoot: java.io.File): Int {
        val values = nativeScanDvdNav(stageRoot.absolutePath)
            ?: error("libdvdnav/libdvdread could not scan the staged DVD metadata")
        require(values.size >= 3) { "libdvdnav DEBUG scanner returned invalid data" }
        val count = values[0].toInt()
        val bestTitle = values[1].toInt()
        require(count > 0 && bestTitle in 1..count) { "libdvdnav DEBUG scanner found no usable titles" }
        android.util.Log.i(
            "MattMuxDVDNavDEBUG",
            "libdvdnav/libdvdread scanned $count title(s); selected title $bestTitle as longest (duration ticks=${values[2]})",
        )
        return bestTitle
    }

    private fun openTitle(context: Context, uri: Uri): NativeTitle {
        val resolver = context.contentResolver
        if (DocumentsContract.isTreeUri(uri)) {
            val stageRoot = DvdNavDebugScanner.stageTreeIfos(context, uri)
            val bestTitle = try {
                scanWithDvdNav(stageRoot)
            } finally {
                stageRoot.deleteRecursively()
            }
            val title = DvdDocumentSource(resolver, uri).openTitle(bestTitle)
            return NativeTitle(title.plan, title.vobs, cleanup = { title.close() })
        }

        val handle = resolver.openFileDescriptor(uri, "r")?.use { nativeOpenIso(it.fd) }
            ?: error("Could not open ISO image")
        check(handle != 0L) { "Could not open UDF filesystem" }
        try {
            val vmg = nativeReadIsoIfo(handle, 0) ?: error("ISO has no VIDEO_TS/VIDEO_TS.IFO")
            val stageRoot = DvdNavDebugScanner.stageIsoIfos(context, vmg) { titleSet ->
                nativeReadIsoIfo(handle, titleSet)
            }
            val bestTitle = try {
                scanWithDvdNav(stageRoot)
            } finally {
                stageRoot.deleteRecursively()
            }
            val plan = DvdIfoParser.selectTitle(vmg, bestTitle) { titleSet ->
                check(!cancelled.get()) { "Remux cancelled" }
                nativeReadIsoIfo(handle, titleSet)
            }
            return NativeTitle(plan, emptyList(), handle, cleanup = { nativeCloseIso(handle) })
        } catch (t: Throwable) {
            nativeCloseIso(handle)
            throw t
        }
    }
'''
s = s[:start] + replacement + s[end:]
marker = '    private external fun nativeVersionSummary(): String\n'
if marker not in s:
    raise SystemExit('native method marker not found')
s = s.replace(marker, marker + '    private external fun nativeScanDvdNav(path: String): LongArray?\n', 1)
p.write_text(s)

p = Path('android/app/src/main/java/io/github/maas3n/mattmux/MainActivity.kt')
s = p.read_text()
s = s.replace('remuxStatus.text = "Reading title metadata…"', 'remuxStatus.text = "DEBUG: scanning titles with libdvdnav/libdvdread…"', 1)
p.write_text(s)

p = Path('android/native/mattmux_jni.c')
s = p.read_text()
include_marker = '#include <libavutil/mem.h>\n'
if include_marker not in s:
    raise SystemExit('JNI include marker not found')
s = s.replace(include_marker, include_marker + '#include <dvdnav/dvdnav.h>\n#include <android/log.h>\n', 1)
append = r'''

JNIEXPORT jlongArray JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeScanDvdNav(JNIEnv *env, jobject thiz, jstring path_string)
{
    (void)thiz;
    if (!path_string) return NULL;
    const char *path = (*env)->GetStringUTFChars(env, path_string, NULL);
    if (!path) return NULL;

    dvdnav_t *nav = NULL;
    if (dvdnav_open(&nav, path) != DVDNAV_STATUS_OK || !nav) {
        __android_log_print(ANDROID_LOG_ERROR, "MattMuxDVDNavDEBUG", "dvdnav_open failed for %s", path);
        (*env)->ReleaseStringUTFChars(env, path_string, path);
        if (nav) dvdnav_close(nav);
        return NULL;
    }
    (*env)->ReleaseStringUTFChars(env, path_string, path);

    int32_t title_count = 0;
    if (dvdnav_get_number_of_titles(nav, &title_count) != DVDNAV_STATUS_OK || title_count <= 0) {
        __android_log_print(ANDROID_LOG_ERROR, "MattMuxDVDNavDEBUG", "dvdnav_get_number_of_titles failed");
        dvdnav_close(nav);
        return NULL;
    }

    int32_t best_title = 1;
    uint64_t best_duration = 0;
    for (int32_t title = 1; title <= title_count; ++title) {
        uint64_t *chapter_times = NULL;
        uint64_t duration = 0;
        /* libdvdnav chapter API is zero-based; MattMux/UI title numbers are one-based. */
        uint32_t chapters = dvdnav_describe_title_chapters(nav, title - 1, &chapter_times, &duration);
        __android_log_print(ANDROID_LOG_INFO, "MattMuxDVDNavDEBUG",
                            "title %d/%d chapters=%u duration_ticks=%llu",
                            title, title_count, chapters, (unsigned long long)duration);
        free(chapter_times);
        if (duration > best_duration) {
            best_duration = duration;
            best_title = title;
        }
    }

    jlong values[3] = {(jlong)title_count, (jlong)best_title, (jlong)best_duration};
    jlongArray result = (*env)->NewLongArray(env, 3);
    if (result) (*env)->SetLongArrayRegion(env, result, 0, 3, values);
    dvdnav_close(nav);
    return result;
}
'''
p.write_text(s + append)
PY

grep -q 'nativeScanDvdNav' android/app/src/main/java/io/github/maas3n/mattmux/RemuxEngine.kt
grep -q 'dvdnav_get_number_of_titles' android/native/mattmux_jni.c
grep -q 'dvdnav_describe_title_chapters(nav, title - 1' android/native/mattmux_jni.c

WORK="$PWD/android/native/.work"
READ_SRC="$WORK/libdvdread-debug"
NAV_SRC="$WORK/libdvdnav-debug"
rm -rf "$READ_SRC" "$NAV_SRC"
git clone --depth 1 --branch 6.1.3 https://code.videolan.org/videolan/libdvdread.git "$READ_SRC"
git clone --depth 1 --branch 6.1.1 https://code.videolan.org/videolan/libdvdnav.git "$NAV_SRC"
git -C "$READ_SRC" archive --format=tar --prefix=libdvdread-6.1.3/ HEAD | gzip -n > "$WORK/libdvdread-6.1.3-source.tar.gz"
git -C "$NAV_SRC" archive --format=tar --prefix=libdvdnav-6.1.1/ HEAD | gzip -n > "$WORK/libdvdnav-6.1.1-source.tar.gz"
(cd "$READ_SRC" && autoreconf -fi)
(cd "$NAV_SRC" && autoreconf -fi)

TOOLCHAIN="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64"
AR="$TOOLCHAIN/bin/llvm-ar"
RANLIB="$TOOLCHAIN/bin/llvm-ranlib"
STRIP="$TOOLCHAIN/bin/llvm-strip"
NM="$TOOLCHAIN/bin/llvm-nm"
API=26

build_one() {
    local abi="$1" target="$2"
    local cc="$TOOLCHAIN/bin/${target}${API}-clang"
    local prefix="$WORK/install-${abi}"
    local jni="$PWD/android/app/src/main/jniLibs/${abi}"
    local read_build="$WORK/build-dvdread-${abi}"
    local nav_build="$WORK/build-dvdnav-${abi}"
    rm -rf "$read_build" "$nav_build" "$prefix"
    mkdir -p "$read_build" "$nav_build" "$prefix"

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
        -shared -fPIC -O2 \
        -I"$prefix/include" -I"$prefix/include/udfread" \
        "$PWD/android/native/mattmux_jni.c" \
        "$PWD/android/native/udf_source.c" \
        -L"$jni" -L"$prefix/lib" \
        -Wl,--no-as-needed \
        -lavformat -lavcodec -lavutil -ludfread \
        -ldvdnav -ldvdread -ldl -lm \
        -Wl,-z,max-page-size=16384 \
        -llog -Wl,--no-undefined -Wl,-soname,libmattmux_jni.so \
        -o "$jni/libmattmux_jni.so"

    while read -r dep; do
        case "$dep" in
            libavutil.so.*) patchelf --replace-needed "$dep" libavutil.so "$jni/libmattmux_jni.so" ;;
            libavcodec.so.*) patchelf --replace-needed "$dep" libavcodec.so "$jni/libmattmux_jni.so" ;;
            libavformat.so.*) patchelf --replace-needed "$dep" libavformat.so "$jni/libmattmux_jni.so" ;;
        esac
    done < <(patchelf --print-needed "$jni/libmattmux_jni.so")

    "$NM" "$jni/libmattmux_jni.so" | grep -q 'dvdnav_get_number_of_titles'
    "$NM" "$jni/libmattmux_jni.so" | grep -q 'DVDOpen'
    "$STRIP" --strip-unneeded "$jni/libmattmux_jni.so"
}
build_one arm64-v8a aarch64-linux-android
build_one x86_64 x86_64-linux-android

{
    printf '\nDEBUG TITLE SCANNER OVERRIDE\n'
    printf 'GPL debug mode: yes\n'
    printf 'Title discovery: libdvdnav 6.1.1 + libdvdread 6.1.3\n'
    printf 'Integration: statically linked into libmattmux_jni.so for DEBUG title scanning\n'
    printf 'MattMux DvdIfoParser longest-title selection: disabled in RemuxEngine DEBUG path\n'
    printf 'libdvdread commit: %s\n' "$(git -C "$READ_SRC" rev-parse HEAD)"
    printf 'libdvdnav commit: %s\n' "$(git -C "$NAV_SRC" rev-parse HEAD)"
} >> android/app/src/main/assets/ffmpeg/ffmpeg-build-info.txt

# Build the signed v1.4.2 DEBUG APK. Signing env is supplied by the workflow.
gradle -p android \
    -PMATTMUX_VERSION_NAME='1.4.2-DEBUG' \
    -PMATTMUX_VERSION_CODE='10403000' \
    :app:testDebugUnitTest :app:lintRelease :app:assembleRelease

APK="$(find android/app/build/outputs/apk/release -name '*.apk' -print -quit)"
test -n "$APK"
BADGING="$("${ANDROID_HOME}/build-tools/36.0.0/aapt" dump badging "$APK")"
grep -Fq "versionCode='10403000'" <<<"$BADGING"
grep -Fq "versionName='1.4.2-DEBUG'" <<<"$BADGING"
"${ANDROID_HOME}/build-tools/36.0.0/zipalign" -c -P 16 -v 4 "$APK"
"${ANDROID_HOME}/build-tools/36.0.0/apksigner" verify --verbose --print-certs "$APK"
unzip -p "$APK" assets/ffmpeg/ffmpeg-build-info.txt | grep -Fq 'Title discovery: libdvdnav 6.1.1 + libdvdread 6.1.3'

mkdir -p dist
cp "$APK" dist/MattMux-1.4.2-Android-DEBUG.apk
sha256sum dist/MattMux-1.4.2-Android-DEBUG.apk | tee dist/MattMux-1.4.2-Android-DEBUG.apk.sha256
cp "$WORK/libdvdread-6.1.3-source.tar.gz" dist/
cp "$WORK/libdvdnav-6.1.1-source.tar.gz" dist/
