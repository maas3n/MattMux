package io.github.maas3n.mattmux

import java.io.File

/** Keep both directions of debug SAF IDs in the same canonical path namespace. */
internal class DemuxDocumentPaths(root: File) {
    // Android's /data/user/0 may alias /data/data. Mixing these paths in
    // relativeTo() emits ../ IDs that no longer resolve beneath the test root.
    private val root = root.canonicalFile

    private fun checked(file: File): File = file.canonicalFile.also {
        require(it == root || it.path.startsWith(root.path + File.separator)) {
            "Document is outside the demux test root: $file"
        }
    }

    fun file(id: String): File = checked(File(root, id))

    fun documentId(file: File): String = checked(file).relativeTo(root).path

    fun isChild(parentId: String, documentId: String): Boolean =
        file(documentId).path.startsWith(file(parentId).path + File.separator)
}
