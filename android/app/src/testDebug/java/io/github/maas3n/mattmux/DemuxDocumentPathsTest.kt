package io.github.maas3n.mattmux

import java.io.File
import java.nio.file.Files
import org.junit.Assert.*
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder

class DemuxDocumentPathsTest {
    @get:Rule val temporary = TemporaryFolder()

    @Test fun symlinkedCacheReturnsReusableChildAndCreatedDocumentIds() {
        val data = temporary.newFolder("data")
        val real = File(data, "data").apply { mkdirs() }
        File(data, "user").mkdirs()
        val alias = File(data, "user/0")
        Files.createSymbolicLink(alias.toPath(), real.toPath())
        val root = File(alias, "app/cache/dvd-saf-test")
        val videoTs = File(root, "movie/VIDEO_TS").apply { mkdirs() }
        val paths = DemuxDocumentPaths(root)

        // Same sequence as queryChildDocuments -> DvdNavScanner -> enforceTree.
        val child = paths.file("movie").listFiles()!!.single()
        val id = paths.documentId(child)
        assertEquals("movie/VIDEO_TS", id)
        assertEquals(videoTs.canonicalFile, paths.file(id))
        assertTrue(paths.isChild("movie", id))

        // createDocument also returns IDs which must survive a later open/query.
        val output = File(paths.file(""), "output").apply { mkdirs() }
        val export = File(output, "track-00.mpeg2").apply { writeText("stream") }
        assertEquals("output/track-00.mpeg2", paths.documentId(export))
        assertEquals("stream", paths.file(paths.documentId(export)).readText())
        assertEquals("", paths.documentId(root))
    }

    @Test fun containmentStillRejectsTraversalSiblingsAndEscapingSymlinks() {
        val root = temporary.newFolder("root")
        val outside = temporary.newFolder("root-sibling")
        val paths = DemuxDocumentPaths(root)
        File(root, "movie/VIDEO_TS").mkdirs()
        File(root, "movie-other").mkdirs()
        assertFalse(paths.isChild("movie", "movie-other"))
        assertFalse(paths.isChild("movie", "movie"))
        assertTrue(paths.isChild("", "movie/VIDEO_TS"))
        assertThrows(IllegalArgumentException::class.java) { paths.file("../root-sibling") }
        assertThrows(IllegalArgumentException::class.java) { paths.documentId(outside) }
        Files.createSymbolicLink(File(root, "escape").toPath(), outside.toPath())
        assertThrows(IllegalArgumentException::class.java) { paths.file("escape") }
    }
}
