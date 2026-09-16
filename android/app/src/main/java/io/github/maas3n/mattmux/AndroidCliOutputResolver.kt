package io.github.maas3n.mattmux

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Process
import android.provider.DocumentsContract
import android.provider.DocumentsContract.Document

internal data class CliOutputDestination(val folder: Uri, val filename: String?)

internal class AndroidCliOutputResolver(
    private val context: Context,
    private val checkCancelled: () -> Unit = {},
) {
    private val resolver = context.contentResolver
    private data class Info(val name: String, val mime: String, val flags: Int)

    fun resolve(source: Uri, explicit: Uri?): CliOutputDestination {
        val info = info(source)
        val iso = info.mime != Document.MIME_TYPE_DIR && info.name.endsWith(".iso", true)
        val name = if (iso) BatchNaming.outputName(info.name.dropLast(4)) else null
        if (explicit != null) {
            require(info(explicit).mime == Document.MIME_TYPE_DIR) { "OUTPUT_ROOT must be a folder" }
            return CliOutputDestination(explicit, name)
        }
        if (!iso) {
            require(info.mime == Document.MIME_TYPE_DIR && DocumentsContract.isTreeUri(source)) { missingOutput() }
            return CliOutputDestination(source, null)
        }
        val target = DocumentsContract.getDocumentId(source)
        val grants = resolver.persistedUriPermissions.filter {
            it.isReadPermission && it.isWritePermission && DocumentsContract.isTreeUri(it.uri) && it.uri.authority == source.authority
        }.map { it.uri }.toMutableList()
        if (DocumentsContract.isTreeUri(source)) {
            val tree = DocumentsContract.buildTreeDocumentUri(requireNotNull(source.authority), DocumentsContract.getTreeDocumentId(source))
            val flags = Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION
            if (context.checkUriPermission(tree, Process.myPid(), Process.myUid(), flags) == PackageManager.PERMISSION_GRANTED) grants.add(tree)
        }
        for (grant in grants.distinct()) {
            checkCancelled()
            val root = DocumentsContract.getTreeDocumentId(grant)
            val scopedSource = DocumentsContract.buildDocumentUriUsingTree(grant, target)
            val path = runCatching { DocumentsContract.findDocumentPath(resolver, scopedSource)?.path }.getOrNull()
            val parent = path?.let { parentInProviderPath(root, target, it) }
            if (parent != null && children(grant, parent).any { it.id == target }) {
                writableFolder(grant, parent)?.let { return CliOutputDestination(it, name) }
            }
            // Some providers don't implement findDocumentPath. Traverse only a
            // granted tree, with a bound and cancellation, to prove the parent.
            val found = findCliDocumentParent(root, target, { children(grant, it) }, checkCancelled)
            if (found != null) writableFolder(grant, found)?.let { return CliOutputDestination(it, name) }
        }
        throw IllegalArgumentException(missingOutput())
    }

    private fun missingOutput() = "Output folder required / no write access to ISO folder. Choose its parent folder or specify --output OUTPUT_ROOT."

    private fun document(uri: Uri): Uri = if (DocumentsContract.isTreeUri(uri)) {
        DocumentsContract.buildDocumentUriUsingTree(uri, documentTreeRootId(uri))
    } else uri

    private fun info(uri: Uri): Info = resolver.query(
        document(uri), arrayOf(Document.COLUMN_DISPLAY_NAME, Document.COLUMN_MIME_TYPE, Document.COLUMN_FLAGS), null, null, null,
    )?.use { cursor ->
        check(cursor.moveToFirst()) { "Document is unavailable: $uri" }
        Info(cursor.getString(0), cursor.getString(1), cursor.getInt(2))
    } ?: error("Document provider did not return metadata: $uri")

    private fun writableFolder(grant: Uri, id: String): Uri? {
        val folder = DocumentsContract.buildDocumentUriUsingTree(grant, id)
        val metadata = runCatching { info(folder) }.getOrNull() ?: return null
        return folder.takeIf { metadata.mime == Document.MIME_TYPE_DIR && metadata.flags and Document.FLAG_DIR_SUPPORTS_CREATE != 0 }
    }

    private fun children(grant: Uri, id: String): List<CliDocumentEntry> {
        checkCancelled()
        return runCatching {
            resolver.query(DocumentsContract.buildChildDocumentsUriUsingTree(grant, id),
                arrayOf(Document.COLUMN_DOCUMENT_ID, Document.COLUMN_MIME_TYPE), null, null, null)?.use { cursor ->
                buildList { while (cursor.moveToNext()) { checkCancelled(); add(CliDocumentEntry(cursor.getString(0), cursor.getString(1) == Document.MIME_TYPE_DIR)) } }
            } ?: emptyList()
        }.getOrElse { checkCancelled(); emptyList() }
    }
}
