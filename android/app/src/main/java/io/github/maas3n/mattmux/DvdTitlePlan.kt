package io.github.maas3n.mattmux

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
