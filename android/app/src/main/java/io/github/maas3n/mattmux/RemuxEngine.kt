package io.github.maas3n.mattmux

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
import android.os.ParcelFileDescriptor
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.locks.ReentrantLock

data class RemuxResult(val outputUri: Uri, val title: Int, val durationMs: Long, val planJson: String)

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
        private val remuxLock = ReentrantLock()
        private val loadFailure: Throwable? = runCatching {
            System.loadLibrary("avutil")
            System.loadLibrary("avcodec")
            System.loadLibrary("avformat")
            System.loadLibrary("udfread")
            System.loadLibrary("mattmux_jni")
        }.exceptionOrNull()
    }

    private val cancelled = AtomicBoolean(false)

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
        cancelled.set(true)
    }

    override fun remux(context: Context, sourceUri: Uri, outputTreeUri: Uri): RemuxResult {
        check(isAvailable) { unavailableReason ?: "Remux engine unavailable" }
        require(DocumentsContract.isTreeUri(outputTreeUri)) { "Output must be a document-tree folder" }
        check(remuxLock.tryLock()) { "Another remux is still stopping. Try again shortly." }
        try {
            check(!cancelled.get()) { "Remux cancelled" }
            return remuxLocked(context, sourceUri, outputTreeUri)
        } finally {
            cancelled.set(false)
            remuxLock.unlock()
        }
    }

    private fun remuxLocked(context: Context, sourceUri: Uri, outputTreeUri: Uri): RemuxResult {
        val resolver = context.contentResolver
        openTitle(context, sourceUri).use { title ->
            check(!cancelled.get()) { "Remux cancelled" }
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
                    title.isoHandle,
                    title.plan.titleSet,
                )
                if (nativeError != null) {
                    throw IllegalStateException(nativeError)
                }
                check(!cancelled.get()) { "Remux cancelled" }
                val finalUri = output.commit(pending)
                return RemuxResult(finalUri, title.plan.globalTitle, title.plan.durationMs, title.plan.diagnosticJson())
            } catch (t: Throwable) {
                output.abort(pending)
                throw t
            }
        }
    }

    private class NativeTitle(
        val plan: DvdTitlePlan,
        val vobs: List<ParcelFileDescriptor>,
        val isoHandle: Long = 0,
        val cleanup: () -> Unit,
    ) : AutoCloseable {
        override fun close() = cleanup()
    }

    private fun openTitle(context: Context, uri: Uri): NativeTitle {
        val resolver = context.contentResolver
        if (DocumentsContract.isTreeUri(uri)) {
            val title = DvdDocumentSource(resolver, uri).openLongestTitle()
            return NativeTitle(title.plan, title.vobs, cleanup = { title.close() })
        }
        val handle = resolver.openFileDescriptor(uri, "r")?.use { nativeOpenIso(it.fd) }
            ?: error("Could not open ISO image")
        check(handle != 0L) { "Could not open UDF filesystem" }
        try {
            val vmg = nativeReadIsoIfo(handle, 0) ?: error("ISO has no VIDEO_TS/VIDEO_TS.IFO")
            val plan = DvdIfoParser.selectLongestTitle(vmg) { titleSet ->
                check(!cancelled.get()) { "Remux cancelled" }
                nativeReadIsoIfo(handle, titleSet)
            }
            return NativeTitle(plan, emptyList(), handle, cleanup = { nativeCloseIso(handle) })
        } catch (t: Throwable) {
            nativeCloseIso(handle)
            throw t
        }
    }

    @Suppress("unused")
    private fun isNativeCancelled(): Boolean = cancelled.get()

    @Suppress("unused")
    private fun onNativeProgress(percent: Int) {
        progressListener?.invoke(percent.coerceIn(0, 100))
    }

    private external fun nativeVersionSummary(): String
    private external fun nativeOpenIso(fd: Int): Long
    private external fun nativeReadIsoIfo(handle: Long, titleSet: Int): ByteArray?
    private external fun nativeCloseIso(handle: Long)
    private external fun nativeRemux(
        vobFds: IntArray,
        cellStartSectors: LongArray,
        cellEndSectors: LongArray,
        outputFd: Int,
        chapterStartsMs: LongArray,
        chapterEndsMs: LongArray,
        isoHandle: Long,
        titleSet: Int,
    ): String?
}
