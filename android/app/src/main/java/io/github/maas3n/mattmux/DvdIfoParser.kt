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
    /** Stable source-independent diagnostic contract (sector ends are exclusive). */
    fun diagnosticJson(): String = "{\"global_title\":$globalTitle,\"title_set\":$titleSet,\"duration_ms\":$durationMs," +
        "\"cells\":[" + cells.joinToString(",") {
            "{\"start_sector\":${it.startSector},\"end_sector_exclusive\":${it.endSectorExclusive}}"
        } + "],\"chapters\":[" + chapterStartsMs.indices.joinToString(",") {
            "{\"start_ms\":${chapterStartsMs[it]},\"end_ms\":${chapterEndsMs[it]}}"
        } + "]}"
}

/** Conservative DVD-Video IFO parser used by the Android LGPL build. */
internal object DvdIfoParser {
    private const val SECTOR_SIZE = 2048

    private data class Location(val global: Int, val vts: Int, val vtsTitle: Int, val chapters: Int)
    private data class Ptt(val pgcn: Int, val pgn: Int)
    private data class Pgc(
        val programs: Int,
        val cells: Int,
        val playbackMode: Int,
        val stillTime: Int,
        val audioControl: IntArray,
        val subpControl: LongArray,
        val palette: IntArray,
        val programMap: IntArray,
        val cellData: ByteArray,
    )

    fun selectLongestTitle(vmg: ByteArray, vtsLoader: (Int) -> ByteArray?): DvdTitlePlan {
        requireMagic(vmg, "DVDVIDEO-VMG")
        val locations = parseLocations(vmg)
        require(locations.isNotEmpty()) { "DVD contains no titles" }
        val cache = mutableMapOf<Int, ByteArray>()
        var best: DvdTitlePlan? = null
        var lastError: Throwable? = null
        locations.forEach { location ->
            try {
                val vts = cache.getOrPut(location.vts) {
                    vtsLoader(location.vts) ?: error("VTS_%02d_0.IFO is missing".format(location.vts))
                }
                val plan = buildPlan(location, vts)
                if (best == null || plan.durationMs > best!!.durationMs) best = plan
            } catch (t: IllegalArgumentException) {
                lastError = t
            }
        }
        return best ?: throw IllegalArgumentException(
            "No safely readable DVD title was found${lastError?.message?.let { ": $it" } ?: ""}",
            lastError,
        )
    }

    private fun parseLocations(vmg: ByteArray): List<Location> {
        val (base, end) = sectorTable(vmg, 0xC4)
        val count = u16(vmg, base)
        require(count in 1..999) { "TT_SRPT has invalid title count $count" }
        require(base + 8 + count * 12 <= end) { "TT_SRPT is truncated" }
        return List(count) { i ->
            val off = base + 8 + i * 12
            val vts = u8(vmg, off + 6)
            val vtsTitle = u8(vmg, off + 7)
            require(vts in 1..99 && vtsTitle > 0) { "DVD title ${i + 1} has invalid VTS mapping" }
            Location(i + 1, vts, vtsTitle, u16(vmg, off + 2))
        }
    }

    private fun buildPlan(location: Location, vts: ByteArray): DvdTitlePlan {
        requireMagic(vts, "DVDVIDEO-VTS")
        val allPtts = parsePtts(vts)
        require(location.vtsTitle in 1..allPtts.size) { "VTS title is outside PTT table" }
        val ptts = allPtts[location.vtsTitle - 1]
        require(ptts.isNotEmpty()) { "Selected title has no chapters" }
        val pgcn = ptts.first().pgcn
        require(pgcn > 0) { "Selected title has PGCN 0" }
        var prev = 0
        ptts.forEach {
            require(it.pgcn == pgcn) { "Title chapters span multiple PGCs" }
            require(it.pgn > prev) { "Title has non-monotonic program mapping" }
            prev = it.pgn
        }

        val pgc = parsePgc(vts, pgcn)
        require(pgc.playbackMode == 0) { "Title uses random/shuffle playback" }
        require(pgc.stillTime == 0) { "Title uses still-time semantics" }
        validateCells(pgc)
        val starts = programStartsMs(pgc)
        val total = totalDurationMs(pgc)
        val firstPgn = ptts.first().pgn
        require(firstPgn in 1..pgc.programs) { "First chapter references invalid program" }
        val baseMs = starts[firstPgn - 1]

        var endProgramExclusive = pgc.programs + 1
        val lastPgn = ptts.last().pgn
        allPtts.forEachIndexed { index, other ->
            if (index != location.vtsTitle - 1 && other.isNotEmpty() && other.first().pgcn == pgcn) {
                val candidate = other.first().pgn
                if (candidate > lastPgn && candidate < endProgramExclusive) endProgramExclusive = candidate
            }
        }
        val titleEndMs = if (endProgramExclusive <= pgc.programs) starts[endProgramExclusive - 1] else total
        require(titleEndMs > baseMs) { "Title duration is zero" }

        val chapterStarts = LongArray(ptts.size)
        val chapterEnds = LongArray(ptts.size)
        ptts.forEachIndexed { i, ptt ->
            require(ptt.pgn in firstPgn until endProgramExclusive) { "Chapter ${i + 1} is outside selected title" }
            chapterStarts[i] = starts[ptt.pgn - 1] - baseMs
            chapterEnds[i] = if (i + 1 < ptts.size) starts[ptts[i + 1].pgn - 1] - baseMs else titleEndMs - baseMs
            require(chapterEnds[i] > chapterStarts[i]) { "Chapter ${i + 1} has non-positive duration" }
        }
        if (location.chapters > 0) require(location.chapters == ptts.size) { "DVD chapter count mismatch" }

        val cells = mutableListOf<DvdCellRange>()
        for (program in firstPgn until endProgramExclusive) {
            val firstCell = pgc.programMap[program - 1]
            val lastCell = if (program < pgc.programs) pgc.programMap[program] - 1 else pgc.cells
            for (cell in firstCell..lastCell) {
                val off = (cell - 1) * 24
                val type = u8(pgc.cellData, off) ushr 6
                if (type == 2 || type == 3) continue
                require(type == 0 || type == 1) { "Cell $cell has invalid angle type" }
                require(u8(pgc.cellData, off + 2) == 0) { "Cell $cell uses still-time semantics" }
                val firstSector = u32(pgc.cellData, off + 8)
                val lastSector = u32(pgc.cellData, off + 20)
                require(lastSector >= firstSector) { "Cell $cell has invalid sector span" }
                cells += DvdCellRange(firstSector, lastSector + 1)
            }
        }
        require(cells.isNotEmpty()) { "Selected title contains no readable cells" }

        val streamLanguages = parseStreamLanguages(vts, pgc)
        val subtitlePalette = IntArray(16) { dvdClutYuvToRgb(pgc.palette[it]) }
        return DvdTitlePlan(location.global, location.vts, titleEndMs - baseMs, cells, chapterStarts, chapterEnds, streamLanguages, subtitlePalette)
    }

    private fun parsePtts(vts: ByteArray): List<List<Ptt>> {
        val (base, end) = sectorTable(vts, 0xC8)
        val count = u16(vts, base)
        require(count in 1..999) { "VTS_PTT_SRPT has invalid title count" }
        val offsetsEnd = base + 8 + count * 4
        require(offsetsEnd <= end) { "VTS_PTT_SRPT offset table is truncated" }
        val offsets = IntArray(count) { u32(vts, base + 8 + it * 4).toInt() }
        return List(count) { i ->
            val start = base + offsets[i]
            val stop = if (i + 1 < count) base + offsets[i + 1] else end
            require(start >= offsetsEnd && stop >= start && stop <= end && (stop - start) % 4 == 0) { "Invalid PTT offsets" }
            List((stop - start) / 4) { j ->
                val off = start + j * 4
                Ptt(u16(vts, off), u16(vts, off + 2))
            }
        }
    }

    private fun parsePgc(vts: ByteArray, pgcn: Int): Pgc {
        val (base, end) = sectorTable(vts, 0xCC)
        val count = u16(vts, base)
        require(pgcn in 1..count) { "PGC $pgcn is outside PGCI table" }
        val srp = base + 8 + (pgcn - 1) * 8
        require(srp + 8 <= end) { "PGCI entry is truncated" }
        val pgcBase = base + u32(vts, srp + 4).toInt()
        require(pgcBase >= base && pgcBase + 0xEC <= end) { "PGC header is outside PGCI" }
        val programs = u8(vts, pgcBase + 2)
        val cells = u8(vts, pgcBase + 3)
        require(programs in 1..cells) { "PGC has invalid program/cell counts" }
        val programMapRel = u16(vts, pgcBase + 0xE6)
        val cellRel = u16(vts, pgcBase + 0xE8)
        require(programMapRel > 0 && cellRel > 0) { "PGC is missing tables" }
        val mapStart = pgcBase + programMapRel
        val cellStart = pgcBase + cellRel
        require(mapStart + programs <= end && cellStart + cells * 24 <= end) { "PGC tables are truncated" }
        val map = IntArray(programs) { u8(vts, mapStart + it) }
        var prev = 0
        map.forEach { cell ->
            require(cell in 1..cells && cell > prev) { "PGC program map is invalid" }
            prev = cell
        }
        val audioControl = IntArray(8) { u16(vts, pgcBase + 12 + it * 2) }
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

    private fun validateCells(pgc: Pgc) {
        var angleBlock = false
        for (cell in 0 until pgc.cells) {
            val flags = u8(pgc.cellData, cell * 24)
            val mode = flags ushr 6
            val type = (flags ushr 4) and 3
            require(flags and 4 == 0) { "Interleaved angle VOBs require NAV/VOBU selection, which this build does not support" }
            when (type) {
                0 -> require(mode == 0 && !angleBlock) { "Malformed DVD angle block" }
                1 -> when (mode) {
                    1 -> { require(!angleBlock); angleBlock = true }
                    2 -> require(angleBlock) { "Orphan middle angle cell" }
                    3 -> { require(angleBlock); angleBlock = false }
                    else -> error("Invalid angle block mode")
                }
                else -> throw IllegalArgumentException("Unsupported DVD cell block type")
            }
            if (cell + 1 in pgc.programMap && cell > 0) {
                require(mode == 0 || mode == 1) { "Program starts inside an angle block" }
            }
        }
        require(!angleBlock) { "Unterminated DVD angle block" }
    }

    private fun programStartsMs(pgc: Pgc): LongArray {
        val starts = LongArray(pgc.programs)
        var total = 0L
        for (program in 0 until pgc.programs) {
            starts[program] = total
            total += programDurationMs(pgc, program)
        }
        return starts
    }

    private fun totalDurationMs(pgc: Pgc): Long = (0 until pgc.programs).sumOf { programDurationMs(pgc, it) }

    private fun programDurationMs(pgc: Pgc, program: Int): Long {
        val firstCell = pgc.programMap[program]
        val lastCell = if (program + 1 < pgc.programs) pgc.programMap[program + 1] - 1 else pgc.cells
        var total = 0L
        for (cell in firstCell..lastCell) {
            val off = (cell - 1) * 24
            val type = u8(pgc.cellData, off) ushr 6
            if (type == 2 || type == 3) continue
            require(type == 0 || type == 1) { "Invalid cell type" }
            require(u8(pgc.cellData, off + 2) == 0) { "Cell uses still-time semantics" }
            total += decodeDvdTimeMs(pgc.cellData, off + 4)
        }
        return total
    }

    internal fun decodeDvdTimeMs(data: ByteArray, off: Int = 0): Long {
        val hh = bcd(u8(data, off))
        val mm = bcd(u8(data, off + 1))
        val ss = bcd(u8(data, off + 2))
        require(mm < 60 && ss < 60) { "Invalid DVD time" }
        val frame = u8(data, off + 3)
        val rate = frame ushr 6
        val frames = bcd(frame and 0x3f)
        val base = (hh * 3600L + mm * 60L + ss) * 1000L
        return when (rate) {
            1 -> { require(frames < 25); base + frames * 1000L / 25L }
            3 -> { require(frames < 30); base + frames * 1001L / 30L }
            0, 2 -> { require(frames == 0); base }
            else -> error("Invalid frame rate")
        }
    }

    private fun sectorTable(data: ByteArray, pointerOff: Int): Pair<Int, Int> {
        val sector = u32(data, pointerOff)
        require(sector > 0 && sector <= Int.MAX_VALUE / SECTOR_SIZE) { "Invalid IFO sector pointer" }
        val base = sector.toInt() * SECTOR_SIZE
        require(base + 8 <= data.size) { "IFO table points outside file" }
        val endAddr = u32(data, base + 4)
        require(endAddr < Int.MAX_VALUE && base.toLong() + endAddr + 1 <= data.size) { "IFO table end is outside file" }
        return base to (base + endAddr.toInt() + 1)
    }

    private fun requireMagic(data: ByteArray, magic: String) {
        require(data.size >= magic.length && String(data, 0, magic.length, Charsets.US_ASCII) == magic) { "Invalid DVD IFO magic" }
    }

    private fun bcd(value: Int): Int {
        val hi = value ushr 4
        val lo = value and 15
        require(hi <= 9 && lo <= 9) { "Invalid BCD value" }
        return hi * 10 + lo
    }

    private fun u8(data: ByteArray, off: Int): Int { require(off in data.indices); return data[off].toInt() and 0xff }
    private fun u16(data: ByteArray, off: Int): Int = (u8(data, off) shl 8) or u8(data, off + 1)
    private fun u32(data: ByteArray, off: Int): Long = (u8(data, off).toLong() shl 24) or (u8(data, off + 1).toLong() shl 16) or (u8(data, off + 2).toLong() shl 8) or u8(data, off + 3).toLong()
}
