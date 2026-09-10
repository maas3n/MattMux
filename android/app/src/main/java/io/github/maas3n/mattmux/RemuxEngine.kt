package io.github.maas3n.mattmux

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract

data class RemuxResult(val outputUri: Uri, val title: Int, val durationMs: Long)

interface RemuxEngine {
    val isAvailable: Boolean
    val unavailableReason: String?
    val runtimeInfo: String?
    fun remux(context: Context, sourceUri: Uri, outputTreeUri: Uri): RemuxResult
    fun cancel()
    fun setProgressListener(listener: ((Int) -> Unit)?)
}

class AndroidNativeRemuxEngine : RemuxEngine {
    companion object {
        private val loadFailure: Throwable? = runCatching {
            System.loadLibrary("avutil")
            System.loadLibrary("avcodec")
            System.loadLibrary("avformat")
            System.loadLibrary("mattmux_jni")
        }.exceptionOrNull()
    }

    @Volatile
    private var progressListener: ((Int) -> Unit)? = null

    override val runtimeInfo: String? by lazy {
        if (loadFailure != null) null else runCatching { nativeVersionSummary() }
            .getOrElse { "Bundled FFmpeg libraries loaded (version probe failed)" }
    }

    override val isAvailable: Boolean
        get() = loadFailure == null

    override val unavailableReason: String?
        get() = loadFailure?.let { "Bundled FFmpeg runtime failed to load: ${it.javaClass.simpleName}" }

    override fun setProgressListener(listener: ((Int) -> Unit)?) {
        progressListener = listener
    }

    override fun cancel() {
        if (loadFailure == null) nativeCancel()
    }

    override fun remux(context: Context, sourceUri: Uri, outputTreeUri: Uri): RemuxResult {
        check(isAvailable) { unavailableReason ?: "Remux engine unavailable" }
        require(DocumentsContract.isTreeUri(sourceUri)) {
            "ISO/UDF images are not supported yet. Extract/select the DVD VIDEO_TS folder for this build."
        }
        require(DocumentsContract.isTreeUri(outputTreeUri)) { "Output must be a document-tree folder" }

        val resolver = context.contentResolver
        DvdDocumentSource(resolver, sourceUri).openLongestTitle().use { title ->
            val output = DvdDocumentOutput(resolver, outputTreeUri)
            val pending = output.create(title.plan.globalTitle)
            try {
                val fds = IntArray(title.vobs.size) { title.vobs[it].fd }
                val starts = LongArray(title.plan.cells.size) { title.plan.cells[it].startSector }
                val ends = LongArray(title.plan.cells.size) { title.plan.cells[it].endSectorExclusive }
                val nativeError = nativeRemux(
                    fds,
                    starts,
                    ends,
                    pending.descriptor.fd,
                    title.plan.chapterStartsMs,
                    title.plan.chapterEndsMs,
                )
                if (nativeError != null) {
                    throw IllegalStateException(nativeError)
                }
                val finalUri = output.commit(pending)
                return RemuxResult(finalUri, title.plan.globalTitle, title.plan.durationMs)
            } catch (t: Throwable) {
                output.abort(pending)
                throw t
            }
        }
    }

    @Suppress("unused")
    private fun onNativeProgress(percent: Int) {
        progressListener?.invoke(percent.coerceIn(0, 100))
    }

    private external fun nativeVersionSummary(): String
    private external fun nativeCancel()
    private external fun nativeRemux(
        vobFds: IntArray,
        cellStartSectors: LongArray,
        cellEndSectors: LongArray,
        outputFd: Int,
        chapterStartsMs: LongArray,
        chapterEndsMs: LongArray,
    ): String?
}
