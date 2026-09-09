package io.github.maas3n.mattmux

/**
 * Boundary for the Android-native remux implementation.
 *
 * FFmpeg/libav is bundled as LGPL-only shared libraries inside the APK/AAB.
 * The actual DVD title/cell navigation and remux orchestration remains behind
 * this interface so billing cannot be enabled before the complete path works.
 */
interface RemuxEngine {
    val isAvailable: Boolean
    val unavailableReason: String?
    val runtimeInfo: String?
}

class AndroidNativeRemuxEngine : RemuxEngine {

    companion object {
        private val loadFailure: Throwable? = runCatching {
            // Load in dependency order. The CI build normalizes FFmpeg SONAMEs
            // to Android-friendly unversioned lib*.so names.
            System.loadLibrary("avutil")
            System.loadLibrary("avcodec")
            System.loadLibrary("avformat")
            System.loadLibrary("mattmux_jni")
        }.exceptionOrNull()
    }

    override val runtimeInfo: String? by lazy {
        if (loadFailure != null) {
            null
        } else {
            runCatching { nativeVersionSummary() }
                .getOrElse { "Bundled FFmpeg libraries loaded (version probe failed)" }
        }
    }

    // The runtime is bundled now, but do not mark remuxing as available until
    // the MattMux DVD/IFO/ISO cell reader and libav custom-I/O remux path land.
    override val isAvailable: Boolean = false

    override val unavailableReason: String
        get() = loadFailure?.let {
            "Bundled FFmpeg runtime failed to load: ${it.javaClass.simpleName}"
        } ?: "${runtimeInfo ?: "Bundled FFmpeg runtime"} is loaded. " +
            "DVD title/cell remux integration is still being implemented."

    private external fun nativeVersionSummary(): String
}
