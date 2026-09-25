package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Test

class BatchDiscoveryTest {
    @Test fun isoAndMovieFolderKeepDistinctSourcesAndOutputParents() {
        // Deliberately identical display stems; provider IDs, not names, identify sources.
        val tree = mapOf(
            "root" to listOf(BatchDocument("Movie.iso", "iso:17", false), BatchDocument("Movie", "folder:42", true)),
            "folder:42" to listOf(BatchDocument("video_ts", "dvd:99", true)),
            "dvd:99" to listOf(BatchDocument("video_ts.ifo", "ifo:8", false)),
        )
        val movies = discoverBatchDocuments("root") { tree[it].orEmpty() }
        assertEquals(2, movies.size)
        assertEquals(setOf("iso:17", "folder:42"), movies.map { it.sourceId }.toSet())
        assertEquals("root", movies.single { it.sourceId == "iso:17" }.outputParentId)
        assertEquals("folder:42", movies.single { it.sourceId == "folder:42" }.outputParentId)
    }

    @Test fun nestedIsoStaysInItsOwnMovieFolder() {
        val tree = mapOf(
            "root" to listOf(BatchDocument("Title", "movie", true)),
            "movie" to listOf(BatchDocument("Disc.ISO", "image", false), BatchDocument("old.mkv", "output", false)),
        )
        assertEquals(listOf(BatchDocumentMovie("Disc", "image", "movie")), discoverBatchDocuments("root") { tree[it].orEmpty() })
    }

    @Test fun invalidVideoTsDoesNotBecomeAMovie() {
        val tree = mapOf(
            "root" to listOf(BatchDocument("Broken", "movie", true)),
            "movie" to listOf(BatchDocument("VIDEO_TS", "dvd", true)),
            "dvd" to listOf(BatchDocument("VTS_01_1.VOB", "vob", false)),
        )
        assertEquals(emptyList<BatchDocumentMovie>(), discoverBatchDocuments("root") { tree[it].orEmpty() })
    }
}
