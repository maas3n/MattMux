package io.github.maas3n.mattmux

internal object WindowLayoutPolicy {
    private const val WIDE_WINDOW_DP = 600
    private const val LARGE_FONT_SCALE = 1.30f

    fun stackSourceButtons(screenWidthDp: Int, fontScale: Float): Boolean {
        return screenWidthDp < WIDE_WINDOW_DP || fontScale >= LARGE_FONT_SCALE
    }
}
