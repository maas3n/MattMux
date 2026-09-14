package io.github.maas3n.mattmux

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
