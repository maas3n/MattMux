package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class MattMuxCliSyntaxTest {
    @Test fun parsesBatchWithQuotedUris() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli --batch \"content://provider/tree/movies\" \"content://provider/tree/output\"")
        assertTrue(parsed is MattMuxCliCommand.Batch)
        parsed as MattMuxCliCommand.Batch
        assertEquals("content://provider/tree/movies", parsed.inputRoot)
        assertEquals("content://provider/tree/output", parsed.outputRoot)
    }

    @Test fun parsesLogEqualsForm() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli --batch --log=batch.log content://provider/tree/movies") as MattMuxCliCommand.Batch
        assertEquals("batch.log", parsed.logFile)
        assertNull(parsed.outputRoot)
    }

    @Test fun parsesScan() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli scan content://provider/document/disc.iso") as MattMuxCliCommand.Scan
        assertEquals("content://provider/document/disc.iso", parsed.source)
    }

    @Test fun parsesMetadataTitle() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli metadata --title 3 content://provider/tree/dvd") as MattMuxCliCommand.Metadata
        assertEquals(3, parsed.title)
        assertEquals("content://provider/tree/dvd", parsed.source)
    }

    @Test fun titleZeroSelectsLongest() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli metadata --title=0 content://provider/tree/dvd") as MattMuxCliCommand.Metadata
        assertNull(parsed.title)
    }

    @Test fun parsesRemuxOptions() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli remux --title=2 --output content://provider/tree/out --no-chapters content://provider/document/disc.iso") as MattMuxCliCommand.Remux
        assertEquals(2, parsed.title)
        assertEquals("content://provider/tree/out", parsed.outputRoot)
        assertTrue(parsed.noChapters)
        assertEquals("content://provider/document/disc.iso", parsed.source)
    }

    @Test fun remuxDefaultsToChapters() {
        val parsed = MattMuxCliSyntax.parse("mattmux-cli remux content://provider/tree/dvd") as MattMuxCliCommand.Remux
        assertFalse(parsed.noChapters)
        assertNull(parsed.title)
        assertNull(parsed.outputRoot)
    }

    @Test fun parsesVersion() {
        assertTrue(MattMuxCliSyntax.parse("mattmux-cli --version") === MattMuxCliCommand.Version)
    }

    @Test fun parsesExplicitStreamsAndChapterChoice() {
        for (option in listOf("--streams 0,2,2", "--streams=0,2,2")) {
            val parsed = MattMuxCliSyntax.parse("mattmux-cli remux $option --no-chapters content://provider/tree/dvd") as MattMuxCliCommand.Remux
            assertEquals(listOf(0, 2), parsed.streams)
            assertTrue(parsed.noChapters)
        }
        val defaults = MattMuxCliSyntax.parse("mattmux-cli remux content://provider/tree/dvd") as MattMuxCliCommand.Remux
        assertNull(defaults.streams)
    }

    @Test fun rejectsInvalidStreamIndexes() {
        for (value in listOf("", "-1", "0,", "video", "2147483648")) {
            val result = runCatching { MattMuxCliSyntax.parse("mattmux-cli remux --streams=$value content://provider/tree/dvd") }
            assertTrue("Accepted invalid stream list: $value", result.exceptionOrNull() is IllegalArgumentException)
        }
    }
}
