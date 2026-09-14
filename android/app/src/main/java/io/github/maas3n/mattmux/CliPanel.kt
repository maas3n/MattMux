package io.github.maas3n.mattmux

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.ScrollView
import android.widget.TextView

class CliPanel(private val activity: Activity) {
    companion object { private const val REQUEST_INPUT = 8300; private const val REQUEST_OUTPUT = 8301 }
    private val runner = MattMuxCliRunner(activity)
    private val controls = mutableListOf<View>()
    private var input: Uri? = null
    private var output: Uri? = null
    @Volatile private var busy = false
    @Volatile private var destroyed = false

    private val command = EditText(activity).apply { setText("mattmux-cli --help"); minLines = 2 }
    private val progress = ProgressBar(activity, null, android.R.attr.progressBarStyleHorizontal).apply { max = 100 }
    private val console = TextView(activity).apply { text = "Android mattmux-cli ready. Folder pickers insert Storage Access Framework content:// URIs into the command." }
    private val cancel = Button(activity).apply { text = "Cancel"; isEnabled = false; setOnClickListener { runner.cancel(); append("Cancelling…") } }
    val view: View

    init {
        val padding = (24 * activity.resources.displayMetrics.density).toInt()
        val content = LinearLayout(activity).apply { orientation = LinearLayout.VERTICAL; setPadding(padding, padding, padding, padding) }
        content.addView(TextView(activity).apply { text = "mattmux-cli"; textSize = 20f })
        content.addView(TextView(activity).apply { text = "Same BATCH command model as desktop. Android storage is URI-based, so use the buttons below or paste persisted content:// tree URIs." })
        fun button(label: String, action: () -> Unit) = Button(activity).apply { text = label; setOnClickListener { action() }; controls += this; content.addView(this) }
        button("CHOOSE MOVIES_ROOT") { choose(REQUEST_INPUT) }
        button("CHOOSE OUTPUT_ROOT (OPTIONAL)") { choose(REQUEST_OUTPUT) }
        button("CLEAR OUTPUT_ROOT") { output = null; refreshCommand() }
        content.addView(command); controls += command
        button("RUN mattmux-cli") { runCommand() }
        content.addView(cancel)
        content.addView(progress)
        content.addView(console)
        view = ScrollView(activity).apply { addView(content) }
    }

    fun onResult(request: Int, result: Int, data: Intent?): Boolean {
        if (request != REQUEST_INPUT && request != REQUEST_OUTPUT) return false
        if (result != Activity.RESULT_OK || data == null || busy) return true
        val uri = data.data ?: return true
        persist(uri, data.flags)
        if (request == REQUEST_INPUT) input = uri else output = uri
        refreshCommand()
        return true
    }

    fun destroy() { destroyed = true; if (busy) runner.cancel() }

    private fun choose(request: Int) {
        activity.startActivityForResult(Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
        }, request)
    }

    private fun persist(uri: Uri, flags: Int) {
        val wanted = flags and (Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
        if (wanted != 0) runCatching { activity.contentResolver.takePersistableUriPermission(uri, wanted) }
    }

    private fun refreshCommand() {
        val root = input ?: run { command.setText("mattmux-cli --help"); return }
        val text = buildString {
            append("mattmux-cli --batch \""); append(root); append('"')
            output?.let { append(" \""); append(it); append('"') }
        }
        command.setText(text)
    }

    private fun runCommand() {
        if (busy) return
        busy = true
        console.text = ""
        progress.progress = 0
        controls.forEach { it.isEnabled = false }
        cancel.isEnabled = true
        val text = command.text.toString()
        Thread {
            val result = runCatching {
                runner.execute(text, emit = { line -> activity.runOnUiThread { if (!destroyed) append(line) } }, progress = { percent, message ->
                    activity.runOnUiThread { if (!destroyed) { progress.progress = percent; append(message) } }
                })
            }
            activity.runOnUiThread {
                if (destroyed) return@runOnUiThread
                busy = false
                controls.forEach { it.isEnabled = true }
                cancel.isEnabled = false
                result.onSuccess { code -> append("mattmux-cli exit code: $code") }
                    .onFailure { append("mattmux-cli error: ${it.message ?: it.javaClass.simpleName}") }
            }
        }.apply { name = "MattMux-Android-CLI" }.start()
    }

    private fun append(line: String) {
        console.text = if (console.text.isEmpty()) line else "${console.text}\n$line"
    }
}
