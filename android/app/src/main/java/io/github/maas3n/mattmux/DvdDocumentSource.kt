package io.github.maas3n.mattmux

import android.content.ContentResolver
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.DocumentsContract.Document
import android.os.ParcelFileDescriptor
import java.util.Locale

internal class DvdDocumentSource(
    private val resolver: ContentResolver,
    private val treeUri: Uri,
) {
    internal data class OpenTitle(
        val plan: DvdTitlePlan,
        val vobs: List<ParcelFileDescriptor>,
    ) : AutoCloseable {
        override fun close() = vobs.forEach { runCatching { it.close() } }
    }

    private data class Entry(val name: String, val documentId: String, val mimeType: String)

    fun openLongestTitle(): OpenTitle {
        val rootId = DocumentsContract.getTreeDocumentId(treeUri)
        val rootChildren = listChildren(rootId)
        val videoTsId = if (rootChildren.any { it.name.equals("VIDEO_TS.IFO", true) }) {
            rootId
        } else {
            rootChildren.firstOrNull {
                it.name.equals("VIDEO_TS", true) && it.mimeType == Document.MIME_TYPE_DIR
            }?.documentId ?: error("VIDEO_TS folder was not found in the selected tree")
        }
        val entries = listChildren(videoTsId)
        val byName = entries.associateBy { it.name.uppercase(Locale.ROOT) }
        val vmg = readEntry(byName["VIDEO_TS.IFO"] ?: error("VIDEO_TS.IFO is missing"))
        val plan = DvdIfoParser.selectLongestTitle(vmg) { titleSet ->
            byName[String.format(Locale.ROOT, "VTS_%02d_0.IFO", titleSet)]?.let(::readEntry)
        }

        val prefix = String.format(Locale.ROOT, "VTS_%02d_", plan.titleSet)
        val vobEntries = entries
            .filter { it.name.uppercase(Locale.ROOT).matches(Regex("${prefix}[1-9]\\.VOB")) }
            .sortedBy { it.name.uppercase(Locale.ROOT) }
        require(vobEntries.isNotEmpty()) { "No title VOB files were found for VTS ${plan.titleSet}" }

        vobEntries.forEachIndexed { index, entry ->
            require(entry.name.equals("${prefix}${index + 1}.VOB", true)) { "Title has a missing VOB part" }
        }
        val opened = mutableListOf<ParcelFileDescriptor>()
        try {
            vobEntries.forEach { entry ->
                opened += resolver.openFileDescriptor(documentUri(entry.documentId), "r")
                    ?: error("Could not open ${entry.name}")
            }
            return OpenTitle(plan, opened)
        } catch (t: Throwable) {
            opened.forEach { runCatching { it.close() } }
            throw t
        }
    }

    private fun listChildren(parentId: String): List<Entry> {
        val childrenUri = DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, parentId)
        return resolver.query(
            childrenUri,
            arrayOf(Document.COLUMN_DISPLAY_NAME, Document.COLUMN_DOCUMENT_ID, Document.COLUMN_MIME_TYPE),
            null,
            null,
            null,
        )?.use { cursor ->
            val nameCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DISPLAY_NAME)
            val idCol = cursor.getColumnIndexOrThrow(Document.COLUMN_DOCUMENT_ID)
            val mimeCol = cursor.getColumnIndexOrThrow(Document.COLUMN_MIME_TYPE)
            buildList {
                while (cursor.moveToNext()) {
                    add(Entry(cursor.getString(nameCol), cursor.getString(idCol), cursor.getString(mimeCol)))
                }
            }
        } ?: error("Selected document provider did not return directory contents")
    }

    private fun readEntry(entry: Entry): ByteArray {
        val uri = documentUri(entry.documentId)
        resolver.openInputStream(uri)?.use { input ->
            val max = 64 * 1024 * 1024
            val out = java.io.ByteArrayOutputStream()
            val buffer = ByteArray(32 * 1024)
            var total = 0
            while (true) {
                val n = input.read(buffer)
                if (n < 0) break
                total += n
                require(total <= max) { "${entry.name} is implausibly large" }
                out.write(buffer, 0, n)
            }
            return out.toByteArray()
        }
        error("Could not read ${entry.name}")
    }

    private fun documentUri(documentId: String): Uri =
        DocumentsContract.buildDocumentUriUsingTree(treeUri, documentId)
}

internal class DvdDocumentOutput(
    private val resolver: ContentResolver,
    private val treeUri: Uri,
) {
    internal data class Pending(val uri: Uri, val descriptor: ParcelFileDescriptor, val finalName: String)

    fun create(title: Int): Pending {
        val parentId = DocumentsContract.getTreeDocumentId(treeUri)
        val parent = DocumentsContract.buildDocumentUriUsingTree(treeUri, parentId)
        val finalName = "MattMux-title-%02d.mkv".format(title)
        val partialName = "$finalName.partial"
        val uri = DocumentsContract.createDocument(resolver, parent, "video/x-matroska", partialName)
            ?: error("The output provider could not create $partialName")
        try {
            val descriptor = resolver.openFileDescriptor(uri, "rw")
                ?: error("The output provider could not open $partialName")
            return Pending(uri, descriptor, finalName)
        } catch (t: Throwable) {
            runCatching { DocumentsContract.deleteDocument(resolver, uri) }
            throw t
        }
    }

    fun commit(pending: Pending): Uri {
        pending.descriptor.close()
        return DocumentsContract.renameDocument(resolver, pending.uri, pending.finalName)
            ?: error("Remux completed, but the output provider could not rename the temporary file. Completed MKV kept at ${pending.uri}")
    }

    fun preserve(pending: Pending) {
        runCatching { pending.descriptor.close() }
    }

    fun abort(pending: Pending) {
        runCatching { pending.descriptor.close() }
        runCatching { DocumentsContract.deleteDocument(resolver, pending.uri) }
    }
}
