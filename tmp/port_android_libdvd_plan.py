from pathlib import Path
import re

ROOT = Path('.')

# 1) Keep the plan data contract, but remove all Kotlin DVD-IFO parsing.
(ROOT / 'android/app/src/main/java/io/github/maas3n/mattmux/DvdTitlePlan.kt').write_text(r'''package io.github.maas3n.mattmux

import java.util.Locale

internal data class DvdCellRange(val startSector: Long, val endSectorExclusive: Long)
internal data class DvdStreamLanguage(val streamId: Int, val language: String)

internal data class DvdTitlePlan(
    val globalTitle: Int,
    val titleSet: Int,
    val durationMs: Long,
    val cells: List<DvdCellRange>,
    val chapterStartsMs: LongArray,
    val chapterEndsMs: LongArray,
    val streamLanguages: List<DvdStreamLanguage> = emptyList(),
    val subtitlePalette: IntArray = IntArray(0),
) {
    fun diagnosticJson(): String = "{\"global_title\":$globalTitle,\"title_set\":$titleSet,\"duration_ms\":$durationMs," +
        "\"cells\":[" + cells.joinToString(",") {
            "{\"start_sector\":${it.startSector},\"end_sector_exclusive\":${it.endSectorExclusive}}"
        } + "],\"chapters\":[" + chapterStartsMs.indices.joinToString(",") {
            "{\"start_ms\":${chapterStartsMs[it]},\"end_ms\":${chapterEndsMs[it]}}"
        } + "]}"
}

/** Decodes JNI records produced from libdvdnav/libdvdread; it does not parse DVD IFO bytes. */
internal object DvdNavPlanCodec {
    fun parse(records: Array<String>): DvdTitlePlan {
        require(records.isNotEmpty()) { "libdvdnav/libdvdread returned no title plan" }
        var globalTitle = 0
        var titleSet = 0
        var durationMs = 0L
        val cells = mutableListOf<DvdCellRange>()
        val starts = mutableListOf<Long>()
        val ends = mutableListOf<Long>()
        val languages = linkedMapOf<Int, String>()
        val palette = IntArray(16)
        var paletteSeen = false

        records.forEach { record ->
            val f = record.split('\t')
            when (f.firstOrNull()) {
                "T" -> {
                    require(f.size == 4) { "Invalid DVD title plan header" }
                    globalTitle = f[1].toInt()
                    titleSet = f[2].toInt()
                    durationMs = f[3].toLong()
                }
                "C" -> {
                    require(f.size == 3) { "Invalid DVD cell record" }
                    cells += DvdCellRange(f[1].toLong(), f[2].toLong())
                }
                "H" -> {
                    require(f.size == 3) { "Invalid DVD chapter record" }
                    starts += f[1].toLong(); ends += f[2].toLong()
                }
                "L" -> {
                    require(f.size == 3) { "Invalid DVD language record" }
                    val code = f[2].lowercase(Locale.ROOT)
                    val iso3 = runCatching { Locale.forLanguageTag(code).isO3Language.lowercase(Locale.ROOT) }.getOrNull()
                    if (!iso3.isNullOrBlank() && iso3.length == 3) languages.putIfAbsent(f[1].toInt(), iso3)
                }
                "P" -> {
                    require(f.size == 3) { "Invalid DVD palette record" }
                    val index = f[1].toInt()
                    require(index in palette.indices) { "Invalid DVD palette index" }
                    palette[index] = f[2].toInt(); paletteSeen = true
                }
                else -> error("Unknown libdvdnav/libdvdread plan record")
            }
        }
        require(globalTitle > 0 && titleSet in 1..99 && durationMs > 0) { "Invalid DVD title plan" }
        require(cells.isNotEmpty()) { "Selected DVD title contains no readable cells" }
        require(starts.isNotEmpty() && starts.size == ends.size) { "Selected DVD title contains no chapters" }
        cells.forEach { require(it.startSector >= 0 && it.endSectorExclusive > it.startSector) { "Invalid DVD cell span" } }
        starts.indices.forEach { require(starts[it] >= 0 && ends[it] > starts[it]) { "Invalid DVD chapter span" } }
        return DvdTitlePlan(
            globalTitle, titleSet, durationMs, cells,
            starts.toLongArray(), ends.toLongArray(),
            languages.map { DvdStreamLanguage(it.key, it.value) },
            if (paletteSeen) palette else IntArray(0),
        )
    }
}
''')

# 2) Folder sources now only open VOB files according to a libdvdread-derived plan.
(ROOT / 'android/app/src/main/java/io/github/maas3n/mattmux/DvdDocumentSource.kt').write_text(r'''package io.github.maas3n.mattmux

import android.content.ContentResolver
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.DocumentsContract.Document
import android.os.ParcelFileDescriptor
import java.util.Locale

internal class DvdDocumentSource(
    private val resolver: ContentResolver,
    private val treeUri: Uri,
) {
    internal data class OpenTitle(
        val plan: DvdTitlePlan,
        val vobs: List<ParcelFileDescriptor>,
    ) : AutoCloseable {
        override fun close() = vobs.forEach { runCatching { it.close() } }
    }

    private data class Entry(val name: String, val documentId: String, val mimeType: String)

    fun openPlan(plan: DvdTitlePlan): OpenTitle {
        val rootId = documentTreeRootId(treeUri)
        val rootChildren = listChildren(rootId)
        val videoTsId = if (rootChildren.any { it.name.equals("VIDEO_TS.IFO", true) }) {
            rootId
        } else {
            rootChildren.firstOrNull {
                it.name.equals("VIDEO_TS", true) && it.mimeType == Document.MIME_TYPE_DIR
            }?.documentId ?: error("VIDEO_TS folder was not found in the selected tree")
        }
        val entries = listChildren(videoTsId)
        val prefix = String.format(Locale.ROOT, "VTS_%02d_", plan.titleSet)
        val vobEntries = entries
            .filter { it.name.uppercase(Locale.ROOT).matches(Regex("${prefix}[1-9]\\.VOB")) }
            .sortedBy { it.name.uppercase(Locale.ROOT) }
        require(vobEntries.isNotEmpty()) { "No title VOB files were found for VTS ${plan.titleSet}" }
        vobEntries.forEachIndexed { index, entry ->
            require(entry.name.equals("${prefix}${index + 1}.VOB", true)) { "Title has a missing VOB part" }
        }
        val opened = mutableListOf<ParcelFileDescriptor>()
        try {
            vobEntries.forEach { entry ->
                opened += resolver.openFileDescriptor(documentUri(entry.documentId), "r")
                    ?: error("Could not open ${entry.name}")
            }
            return OpenTitle(plan, opened)
        } catch (t: Throwable) {
            opened.forEach { runCatching { it.close() } }
            throw t
        }
    }

    private fun listChildren(parentId: String): List<Entry> {
        val childrenUri = DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, parentId)
        return resolver.query(
            childrenUri,
            arrayOf(Document.COLUMN_DISPLAY_NAME, Document.COLUMN_DOCUMENT_ID, Document.COLUMN_MIME_TYPE),
            null, null, null,
        )?.use { cursor ->
            val nameCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DISPLAY_NAME)
            val idCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DOCUMENT_ID)
            val mimeCol = cursor.getColumnIndexOrThrow(Document.COLUMN_MIME_TYPE)
            buildList {
                while (cursor.moveToNext()) add(Entry(cursor.getString(nameCol), cursor.getString(idCol), cursor.getString(mimeCol)))
            }
        } ?: error("Selected document provider did not return directory contents")
    }

    private fun documentUri(documentId: String): Uri = DocumentsContract.buildDocumentUriUsingTree(treeUri, documentId)
}

internal class DvdDocumentOutput(
    private val resolver: ContentResolver,
    private val treeUri: Uri,
) {
    internal data class Pending(val uri: Uri, val descriptor: ParcelFileDescriptor, val finalName: String)

    fun create(title: Int): Pending {
        val parentId = documentTreeRootId(treeUri)
        val parent = DocumentsContract.buildDocumentUriUsingTree(treeUri, parentId)
        val finalName = "MattMux-title-%02d.mkv".format(title)
        val partialName = "$finalName.partial"
        val uri = DocumentsContract.createDocument(resolver, parent, "video/x-matroska", partialName)
            ?: error("The output provider could not create $partialName")
        try {
            val descriptor = resolver.openFileDescriptor(uri, "rw") ?: error("The output provider could not open $partialName")
            return Pending(uri, descriptor, finalName)
        } catch (t: Throwable) {
            runCatching { DocumentsContract.deleteDocument(resolver, uri) }; throw t
        }
    }

    fun commit(pending: Pending): Uri {
        pending.descriptor.close()
        return DocumentsContract.renameDocument(resolver, pending.uri, pending.finalName)
            ?: error("Remux completed, but the output provider could not rename the temporary file. Completed MKV kept at ${pending.uri}")
    }
    fun preserve(pending: Pending) { runCatching { pending.descriptor.close() } }
    fun abort(pending: Pending) {
        runCatching { pending.descriptor.close() }
        runCatching { DocumentsContract.deleteDocument(resolver, pending.uri) }
    }
}
''')

# 3) Route every Android DVD source through libdvdnav/libdvdread planning.
p = ROOT / 'android/app/src/main/java/io/github/maas3n/mattmux/RemuxEngine.kt'
s = p.read_text()
scan = '''    private fun scanWithDvdNav(stageRoot: java.io.File): Int {\n        val values = nativeScanDvdNav(stageRoot.absolutePath)\n            ?: error("libdvdnav/libdvdread could not scan the staged DVD metadata")\n        val result = parseDvdNavScan(values)\n        android.util.Log.i(\n            "MattMuxDVDNav",\n            "libdvdnav/libdvdread scanned ${result.titles.size} title(s); selected title ${result.longestTitle} as longest",\n        )\n        return result.longestTitle\n    }\n'''
if scan not in s:
    raise SystemExit('scanWithDvdNav block not found')
s = s.replace(scan, scan + '''\n    private fun planWithDvdNav(stageRoot: java.io.File, globalTitle: Int): DvdTitlePlan {\n        val records = nativePlanDvdNav(stageRoot.absolutePath, globalTitle)\n            ?: error("libdvdnav/libdvdread could not build the selected DVD title plan")\n        val plan = DvdNavPlanCodec.parse(records)\n        require(plan.globalTitle == globalTitle) { "libdvdnav/libdvdread returned the wrong DVD title" }\n        return plan\n    }\n''', 1)
start = s.index('    private fun openTitle(context: Context, uri: Uri, requestedTitle: Int? = null): NativeTitle {')
end = s.index('\n    @Suppress("unused")\n    private fun isNativeCancelled()', start)
replacement = '''    private fun openTitle(context: Context, uri: Uri, requestedTitle: Int? = null): NativeTitle {\n        require(requestedTitle == null || requestedTitle > 0) { "DVD title must be greater than zero" }\n        val resolver = context.contentResolver\n        if (isDirectorySource(context, uri)) {\n            val stageRoot = DvdNavScanner.stageTreeIfos(context, uri)\n            val plan = try {\n                val title = requestedTitle ?: scanWithDvdNav(stageRoot)\n                planWithDvdNav(stageRoot, title)\n            } finally { stageRoot.deleteRecursively() }\n            val title = DvdDocumentSource(resolver, uri).openPlan(plan)\n            return NativeTitle(title.plan, title.vobs, cleanup = { title.close() })\n        }\n\n        val handle = resolver.openFileDescriptor(uri, "r")?.use { nativeOpenIso(it.fd) }\n            ?: error("Could not open ISO image")\n        check(handle != 0L) { "Could not open UDF filesystem" }\n        try {\n            val vmg = nativeReadIsoIfo(handle, 0) ?: error("ISO has no VIDEO_TS/VIDEO_TS.IFO")\n            val stageRoot = DvdNavScanner.stageIsoIfos(context, vmg) { titleSet -> nativeReadIsoIfo(handle, titleSet) }\n            val plan = try {\n                val title = requestedTitle ?: scanWithDvdNav(stageRoot)\n                planWithDvdNav(stageRoot, title)\n            } finally { stageRoot.deleteRecursively() }\n            return NativeTitle(plan, emptyList(), handle, cleanup = { nativeCloseIso(handle) })\n        } catch (t: Throwable) {\n            nativeCloseIso(handle)\n            throw t\n        }\n    }\n'''
s = s[:start] + replacement + s[end:]
s = s.replace('    private external fun nativeScanDvdNav(path: String): LongArray?\n',
              '    private external fun nativeScanDvdNav(path: String): LongArray?\n    private external fun nativePlanDvdNav(path: String, globalTitle: Int): Array<String>?\n')
p.write_text(s)

p = ROOT / 'android/app/src/main/java/io/github/maas3n/mattmux/DvdNavScanner.kt'
s = p.read_text().replace(
    '/** Stages only DVD IFO metadata so libdvdnav/libdvdread can select the longest title. */',
    '/** Stages only DVD IFO metadata for libdvdnav/libdvdread discovery and title planning. */',
)
p.write_text(s)

# 4) Native planner: libdvdnav chooses the title/chapter timing; libdvdread provides VTS/PGC/cell/stream metadata.
p = ROOT / 'android/native/mattmux_jni.c'
s = p.read_text()
s = s.replace('#include <dvdnav/dvdnav.h>\n#include <android/log.h>\n',
              '#include <dvdnav/dvdnav.h>\n#include <dvdread/dvd_reader.h>\n#include <dvdread/ifo_read.h>\n#include <dvdread/ifo_types.h>\n#include <android/log.h>\n')
planner = r'''

static int mattmux_ascii_language(uint16_t code, char out[3])
{
    unsigned char a = (unsigned char)(code >> 8), b = (unsigned char)(code & 0xff);
    if (!((a >= 'A' && a <= 'Z') || (a >= 'a' && a <= 'z')) ||
        !((b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z'))) return 0;
    out[0] = (char)((a >= 'A' && a <= 'Z') ? a + ('a' - 'A') : a);
    out[1] = (char)((b >= 'A' && b <= 'Z') ? b + ('a' - 'A') : b);
    out[2] = '\0';
    return 1;
}

static int mattmux_audio_stream_id(int format, int position)
{
    switch (format) {
        case 0: return 0x80 + position;
        case 2:
        case 3: return 0x1c0 + position;
        case 4: return 0xa0 + position;
        case 6: return 0x88 + position;
        default: return -1;
    }
}

static int mattmux_yuv_to_rgb(uint32_t raw)
{
    int y = (raw >> 16) & 0xff, cr = (raw >> 8) & 0xff, cb = raw & 0xff;
    int c = y - 16, d = cb - 128, e = cr - 128;
    int r = (298 * c + 409 * e + 128) >> 8;
    int g = (298 * c - 100 * d - 208 * e + 128) >> 8;
    int b = (298 * c + 516 * d + 128) >> 8;
    if (r < 0) r = 0; else if (r > 255) r = 255;
    if (g < 0) g = 0; else if (g > 255) g = 255;
    if (b < 0) b = 0; else if (b > 255) b = 255;
    return (r << 16) | (g << 8) | b;
}

static int mattmux_add_row(JNIEnv *env, jobjectArray array, int index, const char *text)
{
    jstring value = (*env)->NewStringUTF(env, text);
    if (!value) return 0;
    (*env)->SetObjectArrayElement(env, array, index, value);
    (*env)->DeleteLocalRef(env, value);
    return !(*env)->ExceptionCheck(env);
}

JNIEXPORT jobjectArray JNICALL
Java_io_github_maas3n_mattmux_AndroidNativeRemuxEngine_nativePlanDvdNav(JNIEnv *env, jobject thiz, jstring path_string, jint global_title)
{
    (void)thiz;
    if (!path_string || global_title <= 0) return NULL;
    const char *path = (*env)->GetStringUTFChars(env, path_string, NULL);
    if (!path) return NULL;

    dvdnav_t *nav = NULL;
    dvd_reader_t *dvd = NULL;
    ifo_handle_t *vmg = NULL, *vts = NULL;
    uint64_t *chapter_times = NULL, duration = 0;
    jobjectArray result = NULL;
    char **rows = NULL;
    int row_count = 0, row_capacity = 0;

#define ADD_ROW(...) do { \
    if (row_count >= row_capacity) { \
        int next_capacity = row_capacity ? row_capacity * 2 : 64; \
        char **next = realloc(rows, (size_t)next_capacity * sizeof(*rows)); \
        if (!next) goto cleanup_plan; \
        rows = next; row_capacity = next_capacity; \
    } \
    char temp[256]; snprintf(temp, sizeof(temp), __VA_ARGS__); \
    rows[row_count] = strdup(temp); \
    if (!rows[row_count]) goto cleanup_plan; \
    row_count++; \
} while (0)

    if (dvdnav_open(&nav, path) != DVDNAV_STATUS_OK || !nav) goto cleanup_plan;
    uint32_t nav_chapters = dvdnav_describe_title_chapters(nav, global_title - 1, &chapter_times, &duration);
    if (!nav_chapters || !duration) goto cleanup_plan;

    dvd = DVDOpen(path);
    if (!dvd) goto cleanup_plan;
    vmg = ifoOpen(dvd, 0);
    if (!vmg || !vmg->tt_srpt || global_title > vmg->tt_srpt->nr_of_srpts) goto cleanup_plan;
    title_info_t info = vmg->tt_srpt->title[global_title - 1];
    if (info.title_set_nr <= 0 || info.vts_ttn <= 0) goto cleanup_plan;
    vts = ifoOpen(dvd, info.title_set_nr);
    if (!vts || !vts->vts_ptt_srpt || !vts->vts_pgcit || !vts->vtsi_mat) goto cleanup_plan;
    if (info.vts_ttn > vts->vts_ptt_srpt->nr_of_srpts) goto cleanup_plan;

    ttu_t *ttu = &vts->vts_ptt_srpt->title[info.vts_ttn - 1];
    if (!ttu || ttu->nr_of_ptts <= 0 || !ttu->ptt) goto cleanup_plan;
    int pgcn = ttu->ptt[0].pgcn;
    int first_pgn = ttu->ptt[0].pgn;
    int last_pgn = ttu->ptt[ttu->nr_of_ptts - 1].pgn;
    if (pgcn <= 0 || pgcn > vts->vts_pgcit->nr_of_pgci_srp) goto cleanup_plan;
    for (int i = 0, prev = 0; i < ttu->nr_of_ptts; ++i) {
        if (ttu->ptt[i].pgcn != pgcn || ttu->ptt[i].pgn <= prev) goto cleanup_plan;
        prev = ttu->ptt[i].pgn;
    }
    pgc_t *pgc = vts->vts_pgcit->pgci_srp[pgcn - 1].pgc;
    if (!pgc || !pgc->program_map || !pgc->cell_playback || pgc->nr_of_programs <= 0 || pgc->nr_of_cells <= 0) goto cleanup_plan;
    if (first_pgn <= 0 || first_pgn > pgc->nr_of_programs || last_pgn > pgc->nr_of_programs) goto cleanup_plan;
    if (pgc->pg_playback_mode != 0 || pgc->still_time != 0) goto cleanup_plan;

    int end_program_exclusive = pgc->nr_of_programs + 1;
    for (int i = 0; i < vts->vts_ptt_srpt->nr_of_srpts; ++i) {
        if (i == info.vts_ttn - 1) continue;
        ttu_t *other = &vts->vts_ptt_srpt->title[i];
        if (!other || other->nr_of_ptts <= 0 || !other->ptt) continue;
        if (other->ptt[0].pgcn == pgcn && other->ptt[0].pgn > last_pgn && other->ptt[0].pgn < end_program_exclusive)
            end_program_exclusive = other->ptt[0].pgn;
    }

    ADD_ROW("T\t%d\t%d\t%llu", global_title, info.title_set_nr, (unsigned long long)(duration / 90ULL));

    for (int program = first_pgn; program < end_program_exclusive; ++program) {
        int first_cell = pgc->program_map[program - 1];
        int last_cell = program < pgc->nr_of_programs ? pgc->program_map[program] - 1 : pgc->nr_of_cells;
        if (first_cell <= 0 || last_cell < first_cell || last_cell > pgc->nr_of_cells) goto cleanup_plan;
        for (int cell = first_cell; cell <= last_cell; ++cell) {
            cell_playback_t *cp = &pgc->cell_playback[cell - 1];
            if (cp->interleaved || cp->still_time != 0) goto cleanup_plan;
            if (cp->block_type == BLOCK_TYPE_ANGLE_BLOCK) {
                if (cp->block_mode == BLOCK_MODE_IN_BLOCK || cp->block_mode == BLOCK_MODE_LAST_CELL) continue;
                if (cp->block_mode != BLOCK_MODE_FIRST_CELL) goto cleanup_plan;
            } else if (cp->block_mode != BLOCK_MODE_NOT_IN_BLOCK) goto cleanup_plan;
            if (cp->last_sector < cp->first_sector) goto cleanup_plan;
            ADD_ROW("C\t%u\t%u", cp->first_sector, cp->last_sector + 1U);
        }
    }

    uint64_t previous = 0;
    for (uint32_t i = 0; i < nav_chapters; ++i) {
        uint64_t end = chapter_times ? chapter_times[i] : 0;
        if (i + 1 == nav_chapters && duration > end) end = duration;
        if (end <= previous) goto cleanup_plan;
        ADD_ROW("H\t%llu\t%llu", (unsigned long long)(previous / 90ULL), (unsigned long long)(end / 90ULL));
        previous = end;
    }

    for (int i = 0; i < vts->vtsi_mat->nr_of_vts_audio_streams && i < 8; ++i) {
        uint16_t control = pgc->audio_control[i];
        if (!(control & 0x8000)) continue;
        int stream_id = mattmux_audio_stream_id(vts->vtsi_mat->vts_audio_attr[i].audio_format, (control >> 8) & 0x7f);
        char lang[3];
        if (stream_id >= 0 && mattmux_ascii_language(vts->vtsi_mat->vts_audio_attr[i].lang_code, lang)) ADD_ROW("L\t%d\t%s", stream_id, lang);
    }
    for (int i = 0; i < vts->vtsi_mat->nr_of_vts_subp_streams && i < 32; ++i) {
        uint32_t control = pgc->subp_control[i];
        char lang[3];
        if (!(control & 0x80000000U) || !mattmux_ascii_language(vts->vtsi_mat->vts_subp_attr[i].lang_code, lang)) continue;
        int offsets[4] = { (control >> 24) & 0x1f, (control >> 16) & 0x1f, (control >> 8) & 0x1f, control & 0x1f };
        for (int j = 0; j < 4; ++j) ADD_ROW("L\t%d\t%s", 0x20 + offsets[j], lang);
    }
    for (int i = 0; i < 16; ++i) ADD_ROW("P\t%d\t%d", i, mattmux_yuv_to_rgb(pgc->palette[i]));

    result = (*env)->NewObjectArray(env, row_count, (*env)->FindClass(env, "java/lang/String"), NULL);
    if (!result) goto cleanup_plan;
    for (int i = 0; i < row_count; ++i) if (!mattmux_add_row(env, result, i, rows[i])) { result = NULL; break; }

cleanup_plan:
    for (int i = 0; i < row_count; ++i) free(rows[i]);
    free(rows);
    free(chapter_times);
    if (vts) ifoClose(vts);
    if (vmg) ifoClose(vmg);
    if (dvd) DVDClose(dvd);
    if (nav) dvdnav_close(nav);
    (*env)->ReleaseStringUTFChars(env, path_string, path);
    return result;
#undef ADD_ROW
}
'''
if 'nativePlanDvdNav' not in s:
    pos = s.rfind('\n#endif')
    if pos < 0:
        raise SystemExit('MATTMUX_DVDNAV final endif not found')
    s = s[:pos] + planner + s[pos:]
p.write_text(s)

# 5) Remove the custom parser and its parser-specific unit tests.
for rel in [
    'android/app/src/main/java/io/github/maas3n/mattmux/DvdIfoParser.kt',
    'android/app/src/test/java/io/github/maas3n/mattmux/DvdIfoParserTest.kt',
]:
    (ROOT / rel).unlink(missing_ok=True)

(ROOT / 'android/app/src/test/java/io/github/maas3n/mattmux/DvdNavPlanCodecTest.kt').write_text(r'''package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class DvdNavPlanCodecTest {
    @Test fun decodesLibraryPlanRecords() {
        val plan = DvdNavPlanCodec.parse(arrayOf(
            "T\t2\t3\t60000",
            "C\t10\t20", "C\t30\t40",
            "H\t0\t30000", "H\t30000\t60000",
            "L\t128\ten", "P\t0\t16711680",
        ))
        assertEquals(2, plan.globalTitle)
        assertEquals(3, plan.titleSet)
        assertEquals(60000L, plan.durationMs)
        assertEquals(2, plan.cells.size)
        assertEquals("eng", plan.streamLanguages.single().language)
        assertEquals(16711680, plan.subtitlePalette[0])
        assertTrue(plan.diagnosticJson().contains("\"global_title\":2"))
    }
}
''')

p = ROOT / 'android/native/tests/README.md'
s = p.read_text()
s = re.sub(
    r'\n`DvdIfoParserTest` separately verifies IFO selection, angle-1 cell ranges, chapters,\nBCD validation and stable `diagnosticJson\(\)`. The app logs the selected plan under\n`MattMuxPlan`; successful `RemuxResult` also carries it as `planJson`.\n',
    '\nAndroid DVD title discovery and title/cell/chapter planning are provided by the bundled libdvdnav/libdvdread path. Kotlin does not parse DVD IFO structures. The app logs the selected library-derived plan under `MattMuxPlan`; successful `RemuxResult` also carries it as `planJson`.\n',
    s,
)
p.write_text(s)

print('Android libdvdnav/libdvdread planning migration staged.')
