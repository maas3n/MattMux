package io.github.maas3n.mattmux

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import android.view.View
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import java.io.File
import java.util.concurrent.atomic.AtomicBoolean

/** Uses private temporary files so all demuxers can seek and resolve subtitle sidecars. */
class AdvancedMergerPanel(private val activity: Activity) {
    companion object { const val FIRST_REQUEST = 8100 }
    private val native = AdvancedMergerNative()
    private val root = File(activity.cacheDir, "merger-${System.nanoTime()}").apply { mkdirs() }
    private val controls = mutableListOf<View>()
    private val sources = linkedMapOf<Uri, File>()
    private data class Selection(val file: File, val track: TrackInfo, val check: CheckBox)
    private val selections = mutableListOf<Selection>()
    private var chapters: File? = null
    private var output: Uri? = null
    @Volatile private var destroyed = false
    @Volatile private var busy = false
    private val status = TextView(activity).apply { text = "Choose files and select streams. Temporary space is needed for input copies and the output MKV." }
    private val chapterLabel = TextView(activity).apply { text = "No chapter override (optional MKV or FFMETADATA1)" }
    private val outputLabel = TextView(activity).apply { text = "Choose output folder" }
    private val filename = EditText(activity).apply { setSingleLine(); setText("merged.mkv") }
    private val streamList = LinearLayout(activity).apply { orientation = LinearLayout.VERTICAL }
    private val cancelButton = Button(activity).apply { text = "Cancel"; isEnabled = false; setOnClickListener { native.cancelled.set(true) } }
    val view: View

    init {
        val padding = (24 * activity.resources.displayMetrics.density).toInt()
        val content = LinearLayout(activity).apply { orientation = LinearLayout.VERTICAL; setPadding(padding, padding, padding, padding) }
        fun button(label: String, action: () -> Unit) { content.addView(Button(activity).apply { text = label; setOnClickListener { action() }; controls += this }) }
        button("CHOOSE MOVIE FILES") { choose(0) }
        button("CHOOSE AUDIO FILES FROM MKV or RAW") { choose(1) }
        button("CHOOSE SUBTITLE FILES FROM MKV or RAW") { choose(2) }
        content.addView(TextView(activity).apply { text = "Select Streams — movie inputs include every stream and embedded chapters" })
        content.addView(streamList)
        button("Clear streams") { selections.clear(); streamList.removeAllViews() }
        button("CHOOSE CHAPTER FILE FROM MKV or RAW") { choose(3) }
        content.addView(chapterLabel)
        button("Clear chapter override") { chapters = null; chapterLabel.text = "No chapter override" }
        button("CHOOSE OUTPUT FOLDER") { activity.startActivityForResult(Intent(Intent.ACTION_OPEN_DOCUMENT_TREE), FIRST_REQUEST + 4) }
        content.addView(outputLabel)
        content.addView(filename); controls += filename
        button("MUX TO MKV") { mux() }
        content.addView(cancelButton)
        content.addView(status)
        view = ScrollView(activity).apply { addView(content) }
    }

    private fun choose(kind: Int) {
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
            addCategory(Intent.CATEGORY_OPENABLE); type = "*/*"
            putExtra(Intent.EXTRA_ALLOW_MULTIPLE, kind != 3)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        activity.startActivityForResult(intent, FIRST_REQUEST + kind)
    }

    fun onResult(request: Int, result: Int, data: Intent?): Boolean {
        if (request !in FIRST_REQUEST..FIRST_REQUEST + 4) return false
        if (result != Activity.RESULT_OK || data == null || busy) return true
        if (request == FIRST_REQUEST + 4) {
            output = data.data; outputLabel.text = output.toString(); return true
        }
        val uris = mutableListOf<Uri>()
        data.clipData?.let { clip -> repeat(clip.itemCount) { uris += clip.getItemAt(it).uri } }
        if (uris.isEmpty()) data.data?.let { uris += it }
        if (uris.isEmpty()) return true
        val kind = request - FIRST_REQUEST
        run("Preparing input copies…") {
            // Copy the whole batch before probing: IDX can then find a selected SUB sidecar.
            val files = uris.map { uri -> sources[uri] ?: copyInput(uri).also { sources[uri] = it } }
            if (kind == 3) {
                val file = files.single()
                native.validateChapters(file.absolutePath)?.let { error(it) }
                return@run { chapters = file; chapterLabel.text = file.name; status.text = "Chapter override ready" }
            }
            val type = when (kind) { 0 -> null; 1 -> "audio"; else -> "subtitle" }
            val added = files.flatMap { file ->
                // A VobSub .sub is data for the matching .idx, not another selectable subtitle source.
                if (kind == 2 && file.extension.equals("sub", true) && files.any { it.nameWithoutExtension == file.nameWithoutExtension && it.extension.equals("idx", true) }) emptyList()
                else {
                    val tracks = native.probe(file.absolutePath).map(::parseTrack).filter { type == null || it.kind == type }
                    require(tracks.isNotEmpty()) { "${file.name} contains no ${type ?: "streams"}" }
                    tracks.map { file to it }
                }
            }
            return@run {
                added.forEach { (file, track) ->
                    if (selections.none { it.file == file && it.track.index == track.index && it.track.kind == track.kind }) {
                        val label = if (track.kind == "chapters") "Chapters  ${track.title ?: "embedded chapter set"}" else track.displayLabel()
                        val check = CheckBox(activity).apply { text = "${file.name} — $label"; isChecked = true }
                        selections += Selection(file, track, check); streamList.addView(check)
                    }
                }
                status.text = "${selections.size} streams / chapter sets available"
            }
        }
        return true
    }

    private fun copyInput(uri: Uri): File {
        val name = activity.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use {
            if (it.moveToFirst()) it.getString(0) else null
        } ?: "input-${sources.size}"
        require(name == File(name).name && name != "." && name != "..") { "Invalid input filename" }
        // Avoid silently pairing two unrelated sources with the same basename.
        val file = File(root, name)
        require(!file.exists()) { "Two inputs have the same filename: $name. Rename one before adding it." }
        try {
            activity.contentResolver.openInputStream(uri)?.use { input -> file.outputStream().use { output ->
                val buffer = ByteArray(256 * 1024)
                while (true) { check(!native.cancelled.get()) { "Cancelled" }; val n = input.read(buffer); if (n < 0) break; output.write(buffer, 0, n) }
            } } ?: error("Cannot read $name")
        } catch (error: Throwable) { file.delete(); throw error }
        return file
    }

    private fun parseTrack(record: String): TrackInfo {
        val f = record.split('\t', limit = 9)
        require(f.size == 9) { "Invalid stream metadata" }
        return TrackInfo(f[0].toInt(), f[1], f[2], f[3].takeUnless { it == "-" }, f[4].takeUnless { it == "-" }, f[5].toInt(), f[6].toInt(), f[7].toInt(), f[8].takeUnless { it == "-" })
    }

    private fun mux() {
        val selected = selections.filter { it.check.isChecked }
        val media = selected.filter { it.track.kind != "chapters" }
        val selectedChapterFiles = selected.filter { it.track.kind == "chapters" }.map { it.file }.distinct()
        val folder = output
        val name = filename.text.toString().trim()
        if (media.isEmpty() || folder == null || name != File(name).name || !name.endsWith(".mkv", true)) {
            AlertDialog.Builder(activity).setMessage("Select at least one media or attachment stream, choose an output folder, and enter a filename ending in .mkv.").setPositiveButton("OK", null).show(); return
        }
        if (chapters == null && selectedChapterFiles.size > 1) {
            AlertDialog.Builder(activity).setMessage("Select only one movie chapter set, or choose a chapter file to override them.").setPositiveButton("OK", null).show(); return
        }
        val inputs = media.map { it.file }.distinct()
        val chapter = chapters?.absolutePath ?: selectedChapterFiles.singleOrNull()?.absolutePath
        run("Muxing selected streams…") {
            val temporary = File.createTempFile("merged-", ".mkv", root)
            var destination: Uri? = null
            try {
                native.mux(inputs.map { it.absolutePath }.toTypedArray(), media.map { inputs.indexOf(it.file) }.toIntArray(), media.map { it.track.index }.toIntArray(), chapter, temporary.absolutePath)?.let { error(it) }
                check(!native.cancelled.get()) { "Cancelled" }
                val parent = DocumentsContract.buildDocumentUriUsingTree(folder, DocumentsContract.getTreeDocumentId(folder))
                // createDocument creates a new document; it never opens an existing movie for replacement.
                destination = DocumentsContract.createDocument(activity.contentResolver, parent, "video/x-matroska", name) ?: error("Cannot create output document")
                activity.contentResolver.openOutputStream(destination, "w")?.use { out -> temporary.inputStream().use { input ->
                    val buffer = ByteArray(256 * 1024)
                    while (true) { check(!native.cancelled.get()) { "Cancelled" }; val n = input.read(buffer); if (n < 0) break; out.write(buffer, 0, n) }
                } } ?: error("Cannot write output document")
                val completed = destination
                return@run { status.text = "Completed: $completed" }
            } catch (error: Throwable) {
                destination?.let { runCatching { DocumentsContract.deleteDocument(activity.contentResolver, it) } }
                throw error
            } finally { temporary.delete() }
        }
    }

    private fun run(label: String, work: () -> (() -> Unit)) {
        if (busy) return
        busy = true; native.cancelled.set(false); status.text = label
        controls.forEach { it.isEnabled = false }; selections.forEach { it.check.isEnabled = false }; cancelButton.isEnabled = true
        Thread {
            val result = runCatching(work)
            activity.runOnUiThread {
                busy = false
                if (!destroyed) {
                    controls.forEach { it.isEnabled = true }; selections.forEach { it.check.isEnabled = true }; cancelButton.isEnabled = false
                    result.fold({ it() }, { status.text = it.message ?: "Merge failed" })
                }
            }
            if (destroyed) root.deleteRecursively()
        }.start()
    }

    fun destroy() { destroyed = true; native.cancelled.set(true); if (!busy) root.deleteRecursively() }
}

class AdvancedMergerNative {
    val cancelled = AtomicBoolean(false)
    @Suppress("unused") private fun isNativeCancelled(): Boolean = cancelled.get()
    external fun probe(path: String): Array<String>
    external fun validateChapters(path: String): String?
    external fun mux(paths: Array<String>, inputIndexes: IntArray, streamIndexes: IntArray, chapters: String?, output: String): String?
}
