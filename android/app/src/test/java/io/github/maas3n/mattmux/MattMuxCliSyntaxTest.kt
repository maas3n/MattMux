package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
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
        assertEquals(null, parsed.outputRoot)
    }

    @Test fun parsesVersion() {
        assertTrue(MattMuxCliSyntax.parse("mattmux-cli --version") === MattMuxCliCommand.Version)
    }
}
