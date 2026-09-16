package io.github.maas3n.mattmux

import org.junit.Assert.*
import org.junit.Test

class CliDocumentTraversalTest {
    @Test fun providerPathMustBelongToGrantedTreeAndExactSource() {
        assertEquals("opaque-parent", parentInProviderPath("root", "disc", listOf("root", "opaque-parent", "disc")))
        assertNull(parentInProviderPath("root", "disc", listOf("elsewhere", "parent", "disc")))
        assertNull(parentInProviderPath("root", "disc", listOf("root", "different-disc")))
        assertNull(parentInProviderPath("root", "root", listOf("root")))
    }

    @Test fun traversalFindsOpaqueParentWithoutInterpretingIds() {
        val tree = mapOf(
            "root" to listOf(CliDocumentEntry("unrelated", true), CliDocumentEntry("opaque:42", true)),
            "opaque:42" to listOf(CliDocumentEntry("disc-without-path", false)),
        )
        assertEquals("opaque:42", findCliDocumentParent("root", "disc-without-path", { tree[it].orEmpty() }))
        assertEquals("root", findCliDocumentParent("root", "opaque:42", { tree[it].orEmpty() }))
        assertNull(findCliDocumentParent("root", "absent", { tree[it].orEmpty() }))
    }

    @Test fun cyclesTerminateAndTraversalHasABound() {
        var queries = 0
        assertNull(findCliDocumentParent("root", "absent", {
            queries++
            listOf(CliDocumentEntry("root", true))
        }))
        assertEquals(1, queries)
        queries = 0
        assertNull(findCliDocumentParent("0", "absent", {
            queries++
            listOf(CliDocumentEntry((it.toInt() + 1).toString(), true))
        }, limit = 3))
        assertEquals(3, queries)
    }

    @Test fun cancellationStopsBeforeProviderQuery() {
        assertThrows(IllegalStateException::class.java) {
            findCliDocumentParent("root", "disc", { error("must not query") }, { throw IllegalStateException("Remux cancelled") })
        }
    }

    @Test fun streamSelectionRejectsAnyUnknownIndex() {
        validateCliStreamSelection(listOf(0, 2), listOf(0, 1, 2))
        assertThrows(IllegalArgumentException::class.java) { validateCliStreamSelection(listOf(0, 9), listOf(0, 1, 2)) }
        assertThrows(IllegalArgumentException::class.java) { validateCliStreamSelection(emptyList(), listOf(0)) }
    }
}
