package io.github.maas3n.mattmux

internal data class CliDocumentEntry(val id: String, val directory: Boolean)

/** Provider IDs are opaque: establish the parent from provider relationships, not strings. */
internal fun parentInProviderPath(root: String, target: String, path: List<String>): String? =
    path.takeIf { it.size > 1 && it.first() == root && it.last() == target }?.let { it[it.lastIndex - 1] }

internal fun findCliDocumentParent(
    root: String,
    target: String,
    children: (String) -> List<CliDocumentEntry>,
    checkCancelled: () -> Unit = {},
    limit: Int = 2000,
): String? {
    val pending = java.util.ArrayDeque<String>()
    val visited = mutableSetOf<String>()
    pending.add(root)
    while (pending.isNotEmpty() && visited.size < limit) {
        checkCancelled()
        val parent = pending.removeFirst()
        if (!visited.add(parent)) continue
        val entries = children(parent)
        if (entries.any { it.id == target }) return parent
        entries.filter { it.directory && it.id !in visited }.forEach { pending.add(it.id) }
    }
    return null
}

internal fun validateCliStreamSelection(requested: List<Int>, available: List<Int>) {
    require(requested.isNotEmpty()) { "Select at least one stream" }
    val invalid = requested.filter { it !in available }
    require(invalid.isEmpty()) { "Unknown stream indexes: ${invalid.joinToString(",")}. Run metadata for this title to list available streams." }
}
