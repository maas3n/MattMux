package io.github.maas3n.mattmux

import android.net.Uri
import android.provider.DocumentsContract

/** Returns the selected document itself for nested tree/document URIs, otherwise the tree root. */
internal fun documentTreeRootId(uri: Uri): String =
    runCatching { DocumentsContract.getDocumentId(uri) }
        .getOrElse { DocumentsContract.getTreeDocumentId(uri) }
