package io.github.maas3n.mattmux

/**
 * Boundary for the future Android-native FFmpeg/libdvdnav implementation.
 *
 * The Windows application launches pinned external FFmpeg tools. Android apps
 * distributed through Google Play need a native-library integration instead;
 * this interface keeps that work isolated from the ChromeOS UI and billing.
 */
interface RemuxEngine {
    val isAvailable: Boolean
    val unavailableReason: String?
}

class AndroidNativeRemuxEngine : RemuxEngine {
    override val isAvailable: Boolean = false
    override val unavailableReason: String =
        "Android-native FFmpeg/libdvdnav integration has not been bundled yet."
}
