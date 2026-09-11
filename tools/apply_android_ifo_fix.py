from pathlib import Path


def repl(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:100]!r}")
    p.write_text(text.replace(old, new, 1))


parser = "android/app/src/main/java/io/github/maas3n/mattmux/DvdIfoParser.kt"
repl(
    parser,
    "package io.github.maas3n.mattmux\n\ninternal data class DvdCellRange",
    "package io.github.maas3n.mattmux\n\nimport java.util.Locale\n\ninternal data class DvdCellRange",
)
repl(
    parser,
    "internal data class DvdCellRange(val startSector: Long, val endSectorExclusive: Long)\n\ninternal data class DvdTitlePlan(",
    "internal data class DvdCellRange(val startSector: Long, val endSectorExclusive: Long)\ninternal data class DvdStreamLanguage(val streamId: Int, val language: String)\n\ninternal data class DvdTitlePlan(",
)
repl(
    parser,
    "    val chapterEndsMs: LongArray,\n) {",
    "    val chapterEndsMs: LongArray,\n    val streamLanguages: List<DvdStreamLanguage> = emptyList(),\n    val subtitlePalette: IntArray = IntArray(0),\n) {",
)
repl(
    parser,
    "        val stillTime: Int,\n        val programMap: IntArray,\n        val cellData: ByteArray,\n",
    "        val stillTime: Int,\n        val audioControl: IntArray,\n        val subpControl: LongArray,\n        val palette: IntArray,\n        val programMap: IntArray,\n        val cellData: ByteArray,\n",
)
repl(
    parser,
    "        return DvdTitlePlan(location.global, location.vts, titleEndMs - baseMs, cells, chapterStarts, chapterEnds)\n",
    "        val streamLanguages = parseStreamLanguages(vts, pgc)\n        val subtitlePalette = IntArray(16) { dvdClutYuvToRgb(pgc.palette[it]) }\n        return DvdTitlePlan(location.global, location.vts, titleEndMs - baseMs, cells, chapterStarts, chapterEnds, streamLanguages, subtitlePalette)\n",
)
repl(
    parser,
    "        return Pgc(programs, cells, u8(vts, pgcBase + 0xA3), u8(vts, pgcBase + 0xA2), map, vts.copyOfRange(cellStart, cellStart + cells * 24))\n    }\n\n    private fun validateCells",
    '''        val audioControl = IntArray(8) { u16(vts, pgcBase + 12 + it * 2) }
        val subpControl = LongArray(32) { u32(vts, pgcBase + 28 + it * 4) }
        val palette = IntArray(16) { u32(vts, pgcBase + 164 + it * 4).toInt() }
        return Pgc(programs, cells, u8(vts, pgcBase + 0xA3), u8(vts, pgcBase + 0xA2), audioControl, subpControl, palette, map, vts.copyOfRange(cellStart, cellStart + cells * 24))
    }

    private fun parseStreamLanguages(vts: ByteArray, pgc: Pgc): List<DvdStreamLanguage> {
        require(vts.size > 0x256) { "VTSI_MAT is truncated" }
        val languages = linkedMapOf<Int, String>()

        val audioCount = u8(vts, 0x203)
        require(audioCount <= 8 && 0x204 + audioCount * 8 <= vts.size) { "VTS audio attributes are invalid" }
        for (i in 0 until audioCount) {
            val control = pgc.audioControl[i]
            if (control and 0x8000 == 0) continue
            val attr = 0x204 + i * 8
            val audioFormat = u8(vts, attr) ushr 5
            val position = (control ushr 8) and 0x7f
            val streamId = audioStreamId(audioFormat, position) ?: continue
            dvdLanguage(vts, attr + 2)?.let { languages.putIfAbsent(streamId, it) }
        }

        val subpCount = u8(vts, 0x255)
        require(subpCount <= 32 && 0x256 + subpCount * 6 <= vts.size) { "VTS subtitle attributes are invalid" }
        for (i in 0 until subpCount) {
            val control = pgc.subpControl[i]
            if (control and 0x80000000L == 0L) continue
            val language = dvdLanguage(vts, 0x256 + i * 6 + 2) ?: continue
            val offsets = intArrayOf(
                ((control ushr 24) and 0x1f).toInt(),
                ((control ushr 16) and 0x1f).toInt(),
                ((control ushr 8) and 0x1f).toInt(),
                (control and 0x1f).toInt(),
            )
            offsets.forEach { languages.putIfAbsent(0x20 + it, language) }
        }
        return languages.map { DvdStreamLanguage(it.key, it.value) }
    }

    private fun audioStreamId(format: Int, position: Int): Int? = when (format) {
        0 -> 0x80 + position
        2, 3 -> 0x1c0 + position
        4 -> 0xa0 + position
        6 -> 0x88 + position
        else -> null
    }

    private fun dvdLanguage(data: ByteArray, off: Int): String? {
        if (off < 0 || off + 1 >= data.size) return null
        val a = u8(data, off)
        val b = u8(data, off + 1)
        fun asciiLetter(v: Int) = v in 'A'.code..'Z'.code || v in 'a'.code..'z'.code
        if (!asciiLetter(a) || !asciiLetter(b)) return null
        val code = "${a.toChar()}${b.toChar()}".lowercase(Locale.ROOT)
        return runCatching { Locale.forLanguageTag(code).getISO3Language().lowercase(Locale.ROOT) }
            .getOrNull()?.takeIf { it.length == 3 }
    }

    private fun dvdClutYuvToRgb(raw: Int): Int {
        val y = (raw ushr 16) and 0xff
        val cr = (raw ushr 8) and 0xff
        val cb = raw and 0xff
        val c = y - 16
        val d = cb - 128
        val e = cr - 128
        val r = ((298 * c + 409 * e + 128) shr 8).coerceIn(0, 255)
        val g = ((298 * c - 100 * d - 208 * e + 128) shr 8).coerceIn(0, 255)
        val b = ((298 * c + 516 * d + 128) shr 8).coerceIn(0, 255)
        return (r shl 16) or (g shl 8) or b
    }

    private fun validateCells''',
)

engine = "android/app/src/main/java/io/github/maas3n/mattmux/RemuxEngine.kt"
repl(
    engine,
    '                val records = nativeProbeTracks(fds, starts, ends, title.isoHandle, title.plan.titleSet)\n                    ?: error("Could not probe DVD streams")\n',
    '                val languageRecords = title.plan.streamLanguages.map { "${it.streamId}\\t${it.language}" }.toTypedArray()\n                val records = nativeProbeTracks(fds, starts, ends, title.isoHandle, title.plan.titleSet, languageRecords, title.plan.subtitlePalette)\n                    ?: error("Could not probe DVD streams")\n',
)
repl(
    engine,
    '                    title.isoHandle,\n                    title.plan.titleSet,\n                )\n',
    '                    title.isoHandle,\n                    title.plan.titleSet,\n                    title.plan.streamLanguages.map { "${it.streamId}\\t${it.language}" }.toTypedArray(),\n                    title.plan.subtitlePalette,\n                )\n',
)
repl(
    engine,
    '    private external fun nativeProbeTracks(vobFds: IntArray, cellStartSectors: LongArray, cellEndSectors: LongArray, isoHandle: Long, titleSet: Int): Array<String>?\n',
    '    private external fun nativeProbeTracks(vobFds: IntArray, cellStartSectors: LongArray, cellEndSectors: LongArray, isoHandle: Long, titleSet: Int, streamLanguages: Array<String>, subtitlePalette: IntArray): Array<String>?\n',
)
repl(
    engine,
    '        isoHandle: Long,\n        titleSet: Int,\n    ): String?\n',
    '        isoHandle: Long,\n        titleSet: Int,\n        streamLanguages: Array<String>,\n        subtitlePalette: IntArray,\n    ): String?\n',
)

native = Path("android/native/mattmux_jni.c")
text = native.read_text()
marker = "JNIEXPORT jobjectArray JNICALL\nJava_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativeProbeTracks(\n"
helper = r'''static int apply_dvd_ifo_metadata(JNIEnv *env, AVFormatContext *input,
                                  jobjectArray language_records, jintArray palette_array)
{
    if (language_records) {
        jsize count = (*env)->GetArrayLength(env, language_records);
        for (jsize i = 0; i < count; ++i) {
            jstring record = (jstring)(*env)->GetObjectArrayElement(env, language_records, i);
            if (!record) continue;
            const char *value = (*env)->GetStringUTFChars(env, record, NULL);
            if (!value) { (*env)->DeleteLocalRef(env, record); return AVERROR_EXTERNAL; }
            char *end = NULL;
            long stream_id = strtol(value, &end, 10);
            if (end && *end == '\t' && end[1] && stream_id >= 0 && stream_id <= INT_MAX) {
                for (unsigned s = 0; s < input->nb_streams; ++s) {
                    if (input->streams[s]->id == stream_id) {
                        av_dict_set(&input->streams[s]->metadata, "language", end + 1, 0);
                        break;
                    }
                }
            }
            (*env)->ReleaseStringUTFChars(env, record, value);
            (*env)->DeleteLocalRef(env, record);
        }
    }

    if (palette_array && (*env)->GetArrayLength(env, palette_array) == 16) {
        jint *colors = (*env)->GetIntArrayElements(env, palette_array, NULL);
        if (!colors) return AVERROR(ENOMEM);
        char palette[192];
        size_t used = 0;
        int n = snprintf(palette, sizeof(palette), "palette: ");
        if (n < 0 || (size_t)n >= sizeof(palette)) { (*env)->ReleaseIntArrayElements(env, palette_array, colors, JNI_ABORT); return AVERROR(EINVAL); }
        used = (size_t)n;
        for (int i = 0; i < 16; ++i) {
            n = snprintf(palette + used, sizeof(palette) - used, "%06x%s",
                         ((unsigned)colors[i]) & 0xffffffU, i == 15 ? "\n" : ", ");
            if (n < 0 || (size_t)n >= sizeof(palette) - used) { (*env)->ReleaseIntArrayElements(env, palette_array, colors, JNI_ABORT); return AVERROR(EINVAL); }
            used += (size_t)n;
        }
        (*env)->ReleaseIntArrayElements(env, palette_array, colors, JNI_ABORT);

        for (unsigned s = 0; s < input->nb_streams; ++s) {
            AVStream *stream = input->streams[s];
            if (stream->codecpar->codec_id != AV_CODEC_ID_DVD_SUBTITLE) continue;
            uint8_t *extra = av_mallocz(used + AV_INPUT_BUFFER_PADDING_SIZE);
            if (!extra) return AVERROR(ENOMEM);
            memcpy(extra, palette, used);
            av_freep(&stream->codecpar->extradata);
            stream->codecpar->extradata = extra;
            stream->codecpar->extradata_size = (int)used;
        }
    }
    return 0;
}

'''
if text.count(marker) != 1:
    raise SystemExit("native probe marker changed")
text = text.replace(marker, helper + marker, 1)
old_probe_sig = """    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jlong iso_handle, jint title_set)
"""
new_probe_sig = """    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jlong iso_handle, jint title_set, jobjectArray language_records, jintArray palette_array)
"""
if text.count(old_probe_sig) != 1:
    raise SystemExit("native probe signature changed")
text = text.replace(old_probe_sig, new_probe_sig, 1)
probe_anchor = '    ret = avformat_find_stream_info(input, NULL);\n    if (ret < 0) { ff_error(error, sizeof(error), "Could not probe DVD streams", ret); goto cleanup_probe; }\n\n'
if text.count(probe_anchor) != 1:
    raise SystemExit("probe metadata anchor changed")
text = text.replace(
    probe_anchor,
    probe_anchor + '    ret = apply_dvd_ifo_metadata(env, input, language_records, palette_array);\n    if (ret < 0) { ff_error(error, sizeof(error), "Could not apply DVD IFO metadata", ret); goto cleanup_probe; }\n\n',
    1,
)
old_remux_sig = """    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jint output_fd, jlongArray chapter_starts, jlongArray chapter_ends, jintArray selected_streams, jlong iso_handle, jint title_set)
"""
new_remux_sig = """    JNIEnv *env, jobject thiz, jintArray fd_array, jlongArray starts_array, jlongArray ends_array,
    jint output_fd, jlongArray chapter_starts, jlongArray chapter_ends, jintArray selected_streams, jlong iso_handle, jint title_set,
    jobjectArray language_records, jintArray palette_array)
"""
if text.count(old_remux_sig) != 1:
    raise SystemExit("native remux signature changed")
text = text.replace(old_remux_sig, new_remux_sig, 1)
remux_anchor = '    ret = avformat_find_stream_info(input, NULL);\n    if (ret < 0) { ff_error(error, sizeof(error), "Could not probe DVD streams", ret); goto cleanup; }\n\n'
if text.count(remux_anchor) != 1:
    raise SystemExit("remux metadata anchor changed")
text = text.replace(
    remux_anchor,
    remux_anchor + '    ret = apply_dvd_ifo_metadata(env, input, language_records, palette_array);\n    if (ret < 0) { ff_error(error, sizeof(error), "Could not apply DVD IFO metadata", ret); goto cleanup; }\n\n',
    1,
)
native.write_text(text)

test = "android/app/src/test/java/io/github/maas3n/mattmux/DvdIfoParserTest.kt"
repl(
    test,
    '        assertEquals(listOf(DvdCellRange(10, 20), DvdCellRange(30, 50)), plan.cells)\n',
    '        assertEquals(listOf(DvdCellRange(10, 20), DvdCellRange(30, 50)), plan.cells)\n        assertEquals(listOf(DvdStreamLanguage(0x80, "eng"), DvdStreamLanguage(0x20, "nor"), DvdStreamLanguage(0x21, "nor"), DvdStreamLanguage(0x22, "nor"), DvdStreamLanguage(0x23, "nor")), plan.streamLanguages)\n        assertEquals(16, plan.subtitlePalette.size)\n        assertEquals(0x000000, plan.subtitlePalette[0])\n        assertEquals(0xffffff, plan.subtitlePalette[1])\n',
)
repl(
    test,
    '        put32(vts, 0xCC, 2)\n\n        val ptt = 2048\n',
    '        put32(vts, 0xCC, 2)\n        vts[0x203] = 1\n        vts[0x206] = \'e\'.code.toByte()\n        vts[0x207] = \'n\'.code.toByte()\n        vts[0x255] = 1\n        vts[0x256] = 1\n        vts[0x258] = \'n\'.code.toByte()\n        vts[0x259] = \'o\'.code.toByte()\n\n        val ptt = 2048\n',
)
repl(
    test,
    '        vts[pgc + 2] = 2\n        vts[pgc + 3] = 3\n',
    '        vts[pgc + 2] = 2\n        vts[pgc + 3] = 3\n        put16(vts, pgc + 12, 0x8000)\n        put32(vts, pgc + 28, 0x80010203L)\n        put32(vts, pgc + 164, 0x00108080L)\n        put32(vts, pgc + 168, 0x00EB8080L)\n',
)

host = "android/native/tests/java/io/github/maas3n/mattmux/AndroidNativeRemuxEngine.java"
repl(
    host,
    '    private native String[] nativeProbeTracks(int[] fds, long[] starts, long[] ends, long iso, int titleSet);\n    private native String nativeRemux(int[] fds, long[] starts, long[] ends, int output,\n        long[] chapterStarts, long[] chapterEnds, int[] selectedStreams, long iso, int titleSet);\n',
    '    private native String[] nativeProbeTracks(int[] fds, long[] starts, long[] ends, long iso, int titleSet, String[] languages, int[] palette);\n    private native String nativeRemux(int[] fds, long[] starts, long[] ends, int output,\n        long[] chapterStarts, long[] chapterEnds, int[] selectedStreams, long iso, int titleSet, String[] languages, int[] palette);\n',
)
repl(
    host,
    '            String[] tracks = engine.nativeProbeTracks(new int[]{first, second}, new long[]{0}, new long[]{sectors}, 0, 1);\n            String[] isoTracks = engine.nativeProbeTracks(new int[0], new long[]{0}, new long[]{sectors}, iso, 1);\n',
    '            String[] languages = new String[]{"128\\teng"};\n            int[] palette = new int[16];\n            String[] tracks = engine.nativeProbeTracks(new int[]{first, second}, new long[]{0}, new long[]{sectors}, 0, 1, languages, palette);\n            String[] isoTracks = engine.nativeProbeTracks(new int[0], new long[]{0}, new long[]{sectors}, iso, 1, languages, palette);\n',
)
repl(
    host,
    '                if (fields[1].equals("video")) videoIndex = Integer.parseInt(fields[0]);\n',
    '                if (fields[1].equals("video")) videoIndex = Integer.parseInt(fields[0]);\n                if (fields[1].equals("audio") && !fields[3].equals("eng")) throw new AssertionError("IFO audio language not applied: " + track);\n',
)
repl(
    host,
    '                        new long[]{0, 1000}, new long[]{1000, 2000}, null, fromIso ? iso : 0, 1);\n',
    '                        new long[]{0, 1000}, new long[]{1000, 2000}, null, fromIso ? iso : 0, 1, languages, palette);\n',
)
repl(
    host,
    '                    new long[]{0, 1000}, new long[]{1000, 2000}, new int[]{videoIndex}, 0, 1);\n',
    '                    new long[]{0, 1000}, new long[]{1000, 2000}, new int[]{videoIndex}, 0, 1, languages, palette);\n',
)
repl(
    host,
    '                    output, new long[]{0}, new long[]{2000}, null, 0, 1);\n',
    '                    output, new long[]{0}, new long[]{2000}, null, 0, 1, languages, palette);\n',
)
