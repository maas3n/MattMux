package io.github.maas3n.mattmux

import android.database.Cursor
import android.database.MatrixCursor
import android.os.CancellationSignal
import android.os.ParcelFileDescriptor
import android.provider.DocumentsContract.Document
import android.provider.DocumentsProvider
import java.io.File

/** Debug-only real file descriptors behind SAF, for end-to-end DVD export tests. */
class DemuxDocumentsProvider : DocumentsProvider() {
    private val root get() = File(context!!.cacheDir, "dvd-saf-test")
    private fun file(id: String): File = File(root, id).canonicalFile.also {
        val base = root.canonicalFile
        require(it == base || it.path.startsWith(base.path + File.separator))
    }
    override fun onCreate() = true
    override fun isChildDocument(parentDocumentId: String, documentId: String) = file(documentId).path.startsWith(file(parentDocumentId).path + File.separator)
    override fun queryRoots(projection: Array<out String>?): Cursor = MatrixCursor(projection ?: emptyArray())
    private fun rows(projection: Array<out String>?, files: List<File>): Cursor {
        val columns = projection ?: arrayOf(Document.COLUMN_DOCUMENT_ID, Document.COLUMN_DISPLAY_NAME, Document.COLUMN_MIME_TYPE, Document.COLUMN_SIZE, Document.COLUMN_FLAGS)
        return MatrixCursor(columns).apply {
            for (f in files) addRow(columns.map { column -> when(column) {
                Document.COLUMN_DOCUMENT_ID -> f.relativeTo(root).path
                Document.COLUMN_DISPLAY_NAME -> f.name
                Document.COLUMN_MIME_TYPE -> if (f.isDirectory) Document.MIME_TYPE_DIR else "application/octet-stream"
                Document.COLUMN_SIZE -> f.length()
                Document.COLUMN_FLAGS -> Document.FLAG_SUPPORTS_WRITE or Document.FLAG_SUPPORTS_DELETE or (if (f.isDirectory) Document.FLAG_DIR_SUPPORTS_CREATE else 0)
                else -> null
            } }.toTypedArray())
        }
    }
    override fun queryDocument(documentId: String, projection: Array<out String>?) = rows(projection, listOf(file(documentId)))
    override fun queryChildDocuments(parentDocumentId: String, projection: Array<out String>?, sortOrder: String?) = rows(projection, file(parentDocumentId).listFiles()!!.toList())
    override fun openDocument(documentId: String, mode: String, signal: CancellationSignal?) = ParcelFileDescriptor.open(file(documentId), ParcelFileDescriptor.parseMode(mode))
    override fun createDocument(parentDocumentId: String, mimeType: String, displayName: String): String {
        val output = File(file(parentDocumentId), displayName)
        check(if (mimeType == Document.MIME_TYPE_DIR) output.mkdir() else output.createNewFile())
        return output.relativeTo(root).path
    }
    override fun deleteDocument(documentId: String) { check(file(documentId).deleteRecursively()) }
}
