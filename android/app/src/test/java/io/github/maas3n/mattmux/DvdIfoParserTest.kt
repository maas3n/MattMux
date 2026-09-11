package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Assert.assertArrayEquals
import org.junit.Test

class DvdIfoParserTest {
    @Test
    fun decodesNtscAndPalTimes() {
        assertEquals(5_500L, DvdIfoParser.decodeDvdTimeMs(byteArrayOf(0, 0, 5, 0xD5.toByte())))
        assertEquals(40L, DvdIfoParser.decodeDvdTimeMs(byteArrayOf(0, 0, 0, 0x41)))
    }

    @Test
    fun selectsLongestTitleAndAngleOneCells() {
        val (vmg, vts) = fixture()
        val plan = DvdIfoParser.selectLongestTitle(vmg) { if (it == 1) vts else null }
        assertEquals(1, plan.globalTitle)
        assertEquals(1, plan.titleSet)
        assertEquals(15_000L, plan.durationMs)
        assertArrayEquals(longArrayOf(0, 5_000), plan.chapterStartsMs)
        assertArrayEquals(longArrayOf(5_000, 15_000), plan.chapterEndsMs)
        assertEquals(listOf(DvdCellRange(10, 20), DvdCellRange(30, 50)), plan.cells)
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsInvalidFrameBcd() {
        DvdIfoParser.decodeDvdTimeMs(byteArrayOf(0, 0, 0, 0xCF.toByte()))
    }

    @Test
    fun diagnosticPlanIsStable() {
        val plan = DvdTitlePlan(2, 1, 1000, listOf(DvdCellRange(3, 5)), longArrayOf(0), longArrayOf(1000))
        assertEquals("""{"global_title":2,"title_set":1,"duration_ms":1000,"cells":[{"start_sector":3,"end_sector_exclusive":5}],"chapters":[{"start_ms":0,"end_ms":1000}]}""", plan.diagnosticJson())
    }

    @Test
    fun selectsSecondTitleWhenLonger() {
        val (vmg, vts) = fixture()
        put16(vmg, 2048, 2)
        put32(vmg, 2052, 31)
        put16(vmg, 2048 + 20 + 2, 2)
        vmg[2048 + 20 + 6] = 2
        vmg[2048 + 20 + 7] = 1
        val longer = vts.copyOf()
        longer[4096 + 16 + 0xF0 + 6] = 0x08
        longer[4096 + 16 + 0xF0 + 48 + 6] = 0x12
        val plan = DvdIfoParser.selectLongestTitle(vmg) { if (it == 1) vts else longer }
        assertEquals(2, plan.globalTitle)
        assertEquals(2, plan.titleSet)
        assertEquals(20_000L, plan.durationMs)
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsInterleavedAnglesInsteadOfCopyingWrongVobus() {
        val (vmg, vts) = fixture()
        vts[4096 + 16 + 0xF0] = 0x54
        DvdIfoParser.selectLongestTitle(vmg) { vts }
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsOrphanAngleCell() {
        val (vmg, vts) = fixture()
        vts[4096 + 16 + 0xF0] = 0x90.toByte()
        DvdIfoParser.selectLongestTitle(vmg) { vts }
    }

    @Test(expected = IllegalArgumentException::class)
    fun rejectsTruncatedIfo() {
        val (vmg, vts) = fixture()
        DvdIfoParser.selectLongestTitle(vmg) { vts.copyOf(128) }
    }

    private fun fixture(): Pair<ByteArray, ByteArray> {
        val vmg = ByteArray(4096)
        "DVDVIDEO-VMG".toByteArray().copyInto(vmg)
        put32(vmg, 0xC4, 1)
        val tt = 2048
        put16(vmg, tt, 1)
        put32(vmg, tt + 4, 19)
        put16(vmg, tt + 8 + 2, 2)
        vmg[tt + 8 + 6] = 1
        vmg[tt + 8 + 7] = 1

        val vts = ByteArray(8192)
        "DVDVIDEO-VTS".toByteArray().copyInto(vts)
        put32(vts, 0xC8, 1)
        put32(vts, 0xCC, 2)

        val ptt = 2048
        put16(vts, ptt, 1)
        put32(vts, ptt + 4, 19)
        put32(vts, ptt + 8, 12)
        put16(vts, ptt + 12, 1)
        put16(vts, ptt + 14, 1)
        put16(vts, ptt + 16, 1)
        put16(vts, ptt + 18, 2)

        val pgci = 4096
        put16(vts, pgci, 1)
        val pgcRel = 16
        val pgc = pgci + pgcRel
        val pgcEnd = pgcRel + 0xF0 + 3 * 24 - 1
        put32(vts, pgci + 4, pgcEnd.toLong())
        put32(vts, pgci + 12, pgcRel.toLong())
        vts[pgc + 2] = 2
        vts[pgc + 3] = 3
        put16(vts, pgc + 0xE6, 0xEC)
        put16(vts, pgc + 0xE8, 0xF0)
        vts[pgc + 0xEC] = 1
        vts[pgc + 0xED] = 3
        setCell(vts, pgc + 0xF0, category = 0x50, seconds = 5, firstSector = 10, lastSector = 19)
        setCell(vts, pgc + 0xF0 + 24, category = 0xD0, seconds = 7, firstSector = 20, lastSector = 29)
        setCell(vts, pgc + 0xF0 + 48, category = 0, seconds = 10, firstSector = 30, lastSector = 49)

        return vmg to vts
    }

    private fun setCell(data: ByteArray, off: Int, category: Int, seconds: Int, firstSector: Long, lastSector: Long) {
        data[off] = category.toByte()
        data[off + 4] = 0
        data[off + 5] = 0
        data[off + 6] = (((seconds / 10) shl 4) or (seconds % 10)).toByte()
        data[off + 7] = 0xC0.toByte()
        put32(data, off + 8, firstSector)
        put32(data, off + 20, lastSector)
    }

    private fun put16(data: ByteArray, off: Int, value: Int) {
        data[off] = (value ushr 8).toByte()
        data[off + 1] = value.toByte()
    }

    private fun put32(data: ByteArray, off: Int, value: Long) {
        data[off] = (value ushr 24).toByte()
        data[off + 1] = (value ushr 16).toByte()
        data[off + 2] = (value ushr 8).toByte()
        data[off + 3] = value.toByte()
    }
}
