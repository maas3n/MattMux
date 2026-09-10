package io.github.maas3n.mattmux

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WindowLayoutPolicyTest {
    @Test
    fun compactWindowsStackSourceButtons() {
        assertTrue(WindowLayoutPolicy.stackSourceButtons(screenWidthDp = 599, fontScale = 1.0f))
    }

    @Test
    fun wideWindowsKeepSourceButtonsHorizontal() {
        assertFalse(WindowLayoutPolicy.stackSourceButtons(screenWidthDp = 600, fontScale = 1.0f))
    }

    @Test
    fun largeFontsStackButtonsEvenInWideWindows() {
        assertTrue(WindowLayoutPolicy.stackSourceButtons(screenWidthDp = 900, fontScale = 1.30f))
    }
}
