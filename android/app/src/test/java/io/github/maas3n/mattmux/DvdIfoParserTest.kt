package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Assert.assertArrayEquals
import org.junit.Test

class DvdIfoParserTest {
    @Test
    fun decodesNtscAndPalTimes() {
        assertEquals(5_500L, DvdIfoParser.decodeDvdTimeMs(byteArrayOf(0, 0, 5, 0xCF.toByte())))
        assertEquals(40L, DvdIfoParser.decodeDvdTimeMs(byteArrayOf(0, 0, 0, 0x41)))
    }

    @Test
    fun selectsLongestTitleAndAngleOneCells() {
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
        setCell(vts, pgc + 0xF0, category = 0x40, seconds = 5, firstSector = 10, lastSector = 19)
        setCell(vts, pgc + 0xF0 + 24, category = 0xC0, seconds = 7, firstSector = 20, lastSector = 29)
        setCell(vts, pgc + 0xF0 + 48, category = 0, seconds = 10, firstSector = 30, lastSector = 49)

        val plan = DvdIfoParser.selectLongestTitle(vmg) { if (it == 1) vts else null }
        assertEquals(1, plan.globalTitle)
        assertEquals(1, plan.titleSet)
        assertEquals(15_000L, plan.durationMs)
        assertArrayEquals(longArrayOf(0, 5_000), plan.chapterStartsMs)
        assertArrayEquals(longArrayOf(5_000, 15_000), plan.chapterEndsMs)
        assertEquals(listOf(DvdCellRange(10, 20), DvdCellRange(30, 50)), plan.cells)
    }

    private fun setCell(data: ByteArray, off: Int, category: Int, seconds: Int, firstSector: Long, lastSector: Long) {
        data[off] = category.toByte()
        data[off + 4] = 0
        data[off + 5] = 0
        data[off + 6] = seconds.toByte()
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
