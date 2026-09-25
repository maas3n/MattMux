package io.github.maas3n.mattmux

import java.util.Locale

internal data class BatchDocument(val name: String, val documentId: String, val directory: Boolean)
internal data class BatchDocumentMovie(val name: String, val sourceId: String, val outputParentId: String)

/** Keep provider document identity intact; names are only used for display/output. */
internal fun discoverBatchDocuments(rootId: String, children: (String) -> List<BatchDocument>): List<BatchDocumentMovie> {
    val movies = mutableListOf<BatchDocumentMovie>()
    fun addIso(entry: BatchDocument, parent: String) {
        if (!entry.directory && entry.name.endsWith(".iso", true)) {
            movies += BatchDocumentMovie(entry.name.dropLast(4).trim().ifBlank { "DVD" }, entry.documentId, parent)
        }
    }
    for (entry in children(rootId)) {
        if (!entry.directory) {
            addIso(entry, rootId)
            continue
        }
        val entries = children(entry.documentId)
        val videoTs = entries.firstOrNull { it.directory && it.name.equals("VIDEO_TS", true) }
        val dvd = videoTs != null && children(videoTs.documentId).any { !it.directory && it.name.equals("VIDEO_TS.IFO", true) }
        if (dvd) movies += BatchDocumentMovie(entry.name, entry.documentId, entry.documentId)
        else entries.forEach { addIso(it, entry.documentId) }
    }
    return movies.sortedWith(compareBy<BatchDocumentMovie> { it.name.lowercase(Locale.ROOT) }.thenBy { it.sourceId })
}
