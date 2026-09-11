package io.github.maas3n.mattmux

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
import android.os.ParcelFileDescriptor
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.locks.ReentrantLock

data class RemuxResult(val outputUri: Uri, val title: Int, val durationMs: Long, val planJson: String)

data class TrackInfo(
    val index: Int, val kind: String, val codec: String, val language: String?, val title: String?,
    val width: Int, val height: Int, val channels: Int, val channelLayout: String?,
) {
    fun displayLabel(): String {
        val typeName = when (kind) { "video" -> "Video"; "audio" -> "Audio"; "subtitle" -> "Subtitle"; else -> kind }
        val details = mutableListOf<String>()
        if (width > 0 && height > 0) details += "${width}x${height}"
        if (channels > 0) details += if (channelLayout.isNullOrBlank()) "$channels ch" else "$channels ch ($channelLayout)"
        language?.takeIf { it.isNotBlank() }?.let { details += "[$it]" }
        title?.takeIf { it.isNotBlank() }?.let { details += it }
        return buildString {
            append(typeName); append("  #"); append(index); append("  "); append(codec)
            if (details.isNotEmpty()) { append("  "); append(details.joinToString("  ")) }
        }
    }
}

data class TrackProbeResult(val title: Int, val tracks: List<TrackInfo>)

interface RemuxEngine {
    val isAvailable: Boolean
    val unavailableReason: String?
    val runtimeInfo: String?
fun probeTracks(context: Context, sourceUri: Uri): TrackProbeResult
    fun remux(context: Context, sourceUri: Uri, outputTreeUri: Uri, selectedStreamIndexes: IntArray? = null): RemuxResult
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

    override fun probeTracks(context: Context, sourceUri: Uri): TrackProbeResult {
        check(isAvailable) { unavailableReason ?: "Remux engine unavailable" }
        check(remuxLock.tryLock()) { "Another native operation is still stopping. Try again shortly." }
        try {
            check(!cancelled.get()) { "Metadata read cancelled" }
            openTitle(context, sourceUri).use { title ->
                val fds = IntArray(title.vobs.size) { title.vobs[it].fd }
                val starts = LongArray(title.plan.cells.size) { title.plan.cells[it].startSector }
                val ends = LongArray(title.plan.cells.size) { title.plan.cells[it].endSectorExclusive }
                val records = nativeProbeTracks(fds, starts, ends, title.isoHandle, title.plan.titleSet)
                    ?: error("Could not probe DVD streams")
                return TrackProbeResult(title.plan.globalTitle, records.map(::parseTrackRecord))
            }
        } finally {
            cancelled.set(false)
            remuxLock.unlock()
        }
    }

    private fun parseTrackRecord(record: String): TrackInfo {
        val fields = record.split('\t', limit = 9)
        require(fields.size == 9) { "Invalid native track metadata" }
        fun optional(value: String): String? = value.takeUnless { it == "-" || it.isBlank() }
        return TrackInfo(
            index = fields[0].toInt(), kind = fields[1], codec = fields[2],
            language = optional(fields[3]), title = optional(fields[4]),
            width = fields[5].toInt(), height = fields[6].toInt(), channels = fields[7].toInt(),
            channelLayout = optional(fields[8]),
        )
    }

    override fun remux(context: Context, sourceUri: Uri, outputTreeUri: Uri, selectedStreamIndexes: IntArray?): RemuxResult {
        check(isAvailable) { unavailableReason ?: "Remux engine unavailable" }
        require(DocumentsContract.isTreeUri(outputTreeUri)) { "Output must be a document-tree folder" }
        require(selectedStreamIndexes == null || selectedStreamIndexes.isNotEmpty()) { "Select at least one video, audio, or subtitle track before remuxing" }
        check(remuxLock.tryLock()) { "Another remux is still stopping. Try again shortly." }
        try {
            check(!cancelled.get()) { "Remux cancelled" }
            return remuxLocked(context, sourceUri, outputTreeUri, selectedStreamIndexes)
        } finally {
            cancelled.set(false)
            remuxLock.unlock()
        }
    }

    private fun remuxLocked(context: Context, sourceUri: Uri, outputTreeUri: Uri, selectedStreamIndexes: IntArray?): RemuxResult {
        val resolver = context.contentResolver
        openTitle(context, sourceUri).use { title ->
            check(!cancelled.get()) { "Remux cancelled" }
            android.util.Log.i("MattMuxPlan", title.plan.diagnosticJson())
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
                    selectedStreamIndexes,
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
    private external fun nativeProbeTracks(vobFds: IntArray, cellStartSectors: LongArray, cellEndSectors: LongArray, isoHandle: Long, titleSet: Int): Array<String>?
    private external fun nativeRemux(
        vobFds: IntArray,
        cellStartSectors: LongArray,
        cellEndSectors: LongArray,
        outputFd: Int,
        chapterStartsMs: LongArray,
        chapterEndsMs: LongArray,
        selectedStreamIndexes: IntArray?,
        isoHandle: Long,
        titleSet: Int,
    ): String?
}
