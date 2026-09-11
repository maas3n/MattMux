package io.github.maas3n.mattmux

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.graphics.Typeface
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.Button
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast

class MainActivity : Activity(), BillingManager.Listener {

    companion object {
        private const val REQUEST_SOURCE_ISO = 1001
        private const val REQUEST_SOURCE_FOLDER = 1002
        private const val REQUEST_OUTPUT_FOLDER = 1003
        private const val STATE_SOURCE_URI = "source_uri"
        private const val STATE_OUTPUT_URI = "output_uri"
        private const val STATE_TRACK_SELECTION_SET = "track_selection_set"
        private const val STATE_SELECTED_TRACKS = "selected_tracks"
    }

    private val engine: RemuxEngine = AndroidNativeRemuxEngine()
    private var billing: BillingManager? = null

    private lateinit var sourceValue: TextView
    private lateinit var outputValue: TextView
    private lateinit var proValue: TextView
    private lateinit var billingValue: TextView
    private lateinit var remuxStatus: TextView
    private lateinit var buyButton: Button
    private lateinit var remuxButton: Button
    private lateinit var cancelButton: Button
    private lateinit var tracksButton: Button

    private var sourceUri: Uri? = null
    private var outputUri: Uri? = null
    private var selectedTrackIndexes: Set<Int>? = null
    private var proOwned = false
    @Volatile private var remuxRunning = false
    @Volatile private var metadataBusy = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildUi())
        restoreSelectionState(savedInstanceState)

        engine.setProgressListener { percent ->
            runOnUiThread { remuxStatus.text = "Remuxing… $percent%" }
        }
        if (BuildConfig.ENABLE_BILLING_PURCHASES) {
            billing = BillingManager(this, this).also { it.start() }
        } else {
            proValue.text = "MattMux Pro: purchases disabled in this alpha"
            billingValue.text = "Billing is off until production device validation and release readiness are complete."
            buyButton.visibility = View.GONE
        }
        updateRemuxButton()
    }

    override fun onSaveInstanceState(outState: Bundle) {
        outState.putString(STATE_SOURCE_URI, sourceUri?.toString())
        outState.putString(STATE_OUTPUT_URI, outputUri?.toString())
        outState.putBoolean(STATE_TRACK_SELECTION_SET, selectedTrackIndexes != null)
        selectedTrackIndexes?.let { outState.putIntArray(STATE_SELECTED_TRACKS, it.sorted().toIntArray()) }
        super.onSaveInstanceState(outState)
    }

    override fun onDestroy() {
        if (remuxRunning) engine.cancel()
        engine.setProgressListener(null)
        billing?.close()
        super.onDestroy()
    }

    override fun onBillingState(state: BillingManager.State) {
        runOnUiThread {
            proOwned = state.proOwned
            proValue.text = if (state.proOwned) "MattMux Pro: unlocked" else "MattMux Pro: not unlocked"
            val price = state.price ?: "price loads from Google Play"
            buyButton.text = if (state.proOwned) "MattMux Pro owned" else "Buy MattMux Pro ($price)"
            buyButton.isEnabled = !state.proOwned && BuildConfig.ENABLE_BILLING_PURCHASES
            billingValue.text = state.message ?: if (state.connected) "Google Play connected" else "Google Play unavailable"
            updateRemuxButton()
        }
    }

    @Deprecated("Uses the platform document picker result API for minSdk simplicity.")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (resultCode != RESULT_OK) return
        val resultData = data ?: return
        val uri = resultData.data ?: return
        persistUriPermission(uri, resultData)

        when (requestCode) {
            REQUEST_SOURCE_ISO, REQUEST_SOURCE_FOLDER -> {
                sourceUri = uri
                sourceValue.text = describeUri(uri)
                clearTrackSelection()
            }
            REQUEST_OUTPUT_FOLDER -> {
                outputUri = uri
                outputValue.text = describeUri(uri)
            }
        }
        updateRemuxButton()
    }

    private fun restoreSelectionState(savedInstanceState: Bundle?) {
        sourceUri = savedInstanceState?.getString(STATE_SOURCE_URI)?.takeIf { it.isNotBlank() }?.let(Uri::parse)
        outputUri = savedInstanceState?.getString(STATE_OUTPUT_URI)?.takeIf { it.isNotBlank() }?.let(Uri::parse)
        selectedTrackIndexes = if (savedInstanceState?.getBoolean(STATE_TRACK_SELECTION_SET) == true) savedInstanceState.getIntArray(STATE_SELECTED_TRACKS)?.toSet() ?: emptySet() else null
        sourceValue.text = sourceUri?.let(::describeUri) ?: "No source selected"
        outputValue.text = outputUri?.let(::describeUri) ?: "No output folder selected"
    }

    private fun persistUriPermission(uri: Uri, data: Intent) {
        val readGranted = data.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0
        val writeGranted = data.flags and Intent.FLAG_GRANT_WRITE_URI_PERMISSION != 0
        try {
            when {
                readGranted && writeGranted -> contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
                writeGranted -> contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
                readGranted -> contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
        } catch (_: SecurityException) {
        }
    }

    private fun buildUi(): ViewGroup {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(28), dp(24), dp(28), dp(28))
        }
        root.addView(TextView(this).apply {
            text = "MattMux"
            textSize = 30f
            setTypeface(typeface, Typeface.BOLD)
        })
        root.addView(TextView(this).apply {
            text = "DVD / VIDEO_TS / ISO → MKV without transcoding"
            textSize = 16f
            setPadding(0, dp(4), 0, dp(22))
        })

        root.addView(section("Source"))
        sourceValue = value("No source selected")
        root.addView(sourceValue)
        val sourceButtons = sourceButtonContainer()
        sourceButtons.addView(button("Choose ISO") { chooseIso() })
        sourceButtons.addView(button("Choose DVD folder") { chooseSourceFolder() })
        root.addView(sourceButtons)

        root.addView(section("Output"))
        outputValue = value("No output folder selected")
        root.addView(outputValue)
        root.addView(button("Choose output folder") { chooseOutputFolder() })

        root.addView(section("Google Play"))
        proValue = value("MattMux Pro: checking…")
        billingValue = value("Connecting to Google Play…")
        root.addView(proValue)
        root.addView(billingValue)
        buyButton = button("Buy MattMux Pro") { billing?.launchProPurchase(this) }
        root.addView(buyButton)

        root.addView(section("Remux"))
        val runtimeMessage = engine.runtimeInfo?.let {
            "Bundled native runtime: $it\n\nVIDEO_TS folders and UDF ISO images use the same DVD title/cell planner. Select an unencrypted DVD; ISO files must be on storage that supports seeking. Interleaved multi-angle discs are not supported in this alpha."
        } ?: "The bundled native FFmpeg runtime could not be loaded in this build."
        root.addView(value(runtimeMessage))
        tracksButton = button("Show Metadata") { showTrackMetadata() }
        root.addView(tracksButton)
        remuxStatus = value("Ready")
        root.addView(remuxStatus)
        remuxButton = button("Remux to MKV") { startRemux() }
        root.addView(remuxButton)
        cancelButton = button("Cancel remux") {
            engine.cancel()
            remuxStatus.text = "Cancelling…"
        }.apply { isEnabled = false }
        root.addView(cancelButton)

        return ScrollView(this).apply { addView(root) }
    }

    private fun startRemux() {
        val source = sourceUri ?: return
        val output = outputUri ?: return
        if (!engine.isAvailable) {
            toast(engine.unavailableReason ?: "Remux engine unavailable")
            return
        }
        if (BuildConfig.ENABLE_BILLING_PURCHASES && !proOwned) {
            billing?.launchProPurchase(this)
            return
        }
        if (remuxRunning) return
        val selectedStreams = selectedTrackIndexes?.sorted()?.toIntArray()
        if (selectedStreams != null && selectedStreams.isEmpty()) {
            toast("Select at least one video, audio, or subtitle track first")
            return
        }

        remuxRunning = true
        remuxStatus.text = "Preparing DVD title…"
        updateRemuxButton()
        Thread {
            val result = runCatching { engine.remux(this, source, output, selectedStreams) }
            runOnUiThread {
                remuxRunning = false
                result.onSuccess {
                    remuxStatus.text = "Complete: title ${it.title} → ${describeUri(it.outputUri)}"
                    toast("Remux complete")
                }.onFailure {
                    remuxStatus.text = "Remux failed: ${it.message ?: it.javaClass.simpleName}"
                    toast(it.message ?: "Remux failed")
                }
                updateRemuxButton()
            }
        }.apply { name = "MattMux-remux" }.start()
    }

    private fun showTrackMetadata() {
        val source = sourceUri ?: run { toast("Choose a DVD source first"); return }
        if (!engine.isAvailable) { toast(engine.unavailableReason ?: "Remux engine unavailable"); return }
        if (metadataBusy || remuxRunning) return
        metadataBusy = true
        remuxStatus.text = "Reading title metadata…"
        updateRemuxButton()
        Thread {
            val result = runCatching { engine.probeTracks(this, source) }
            runOnUiThread {
                metadataBusy = false
                result.onSuccess { probe ->
                    if (probe.tracks.isEmpty()) {
                        remuxStatus.text = "No selectable tracks were found"
                        toast("This DVD title contains no selectable tracks")
                    } else {
                        showTrackDialog(probe)
                        remuxStatus.text = "Metadata loaded for title ${probe.title}. Choose tracks to include."
                    }
                }.onFailure {
                    remuxStatus.text = "Metadata failed: ${it.message ?: it.javaClass.simpleName}"
                    toast(it.message ?: "Metadata read failed")
                }
                updateRemuxButton()
            }
        }.apply { name = "MattMux-metadata" }.start()
    }

    private fun showTrackDialog(probe: TrackProbeResult) {
        val previous = selectedTrackIndexes
        val checked = BooleanArray(probe.tracks.size) { index -> previous?.contains(probe.tracks[index].index) ?: true }
        AlertDialog.Builder(this)
            .setTitle("Title ${probe.title} — Tracks / Metadata")
            .setMultiChoiceItems(probe.tracks.map { it.displayLabel() }.toTypedArray(), checked) { _, which, value -> checked[which] = value }
            .setPositiveButton("Use selection") { _, _ ->
                selectedTrackIndexes = probe.tracks.indices.filter { checked[it] }.map { probe.tracks[it].index }.toSet()
                val count = selectedTrackIndexes?.size ?: 0
                remuxStatus.text = if (count == 0) "No tracks selected. Select at least one track before remuxing." else "$count track(s) selected for the next remux."
                updateRemuxButton()
            }
            .setNeutralButton("All tracks") { _, _ ->
                selectedTrackIndexes = probe.tracks.map { it.index }.toSet()
                remuxStatus.text = "All ${probe.tracks.size} track(s) selected for the next remux."
                updateRemuxButton()
            }
            .setNegativeButton("Close", null)
            .show()
    }

    private fun clearTrackSelection() {
        selectedTrackIndexes = null
        if (::remuxStatus.isInitialized) remuxStatus.text = "Source changed. Show Metadata to choose tracks, or remux all tracks by default."
        if (::tracksButton.isInitialized) updateRemuxButton()
    }

    private fun chooseIso() {
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
            addCategory(Intent.CATEGORY_OPENABLE)
            type = "*/*"
            putExtra(Intent.EXTRA_MIME_TYPES, arrayOf("application/x-iso9660-image", "application/octet-stream"))
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
        }
        startActivityForResult(intent, REQUEST_SOURCE_ISO)
    }

    private fun chooseSourceFolder() {
        startActivityForResult(Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
        }, REQUEST_SOURCE_FOLDER)
    }

    private fun chooseOutputFolder() {
        startActivityForResult(Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
        }, REQUEST_OUTPUT_FOLDER)
    }

    private fun updateRemuxButton() {
        val hasPaths = sourceUri != null && outputUri != null
        val hasSelectedTracks = selectedTrackIndexes?.isNotEmpty() ?: true
        remuxButton.isEnabled = hasPaths && hasSelectedTracks && !remuxRunning && !metadataBusy && engine.isAvailable
        tracksButton.isEnabled = sourceUri != null && !remuxRunning && !metadataBusy && engine.isAvailable
        cancelButton.isEnabled = remuxRunning
        remuxButton.text = when {
            remuxRunning -> "Remuxing…"
            !engine.isAvailable -> "Remux engine unavailable"
            selectedTrackIndexes != null && selectedTrackIndexes!!.isEmpty() -> "Select at least one track"
            BuildConfig.ENABLE_BILLING_PURCHASES && !proOwned -> "Unlock Pro to remux"
            else -> "Remux to MKV"
        }
    }

    private fun describeUri(uri: Uri): String {
        try {
            contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
                val index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (index >= 0 && cursor.moveToFirst()) return cursor.getString(index)
            }
        } catch (_: SecurityException) {
        } catch (_: RuntimeException) {
        }
        return uri.toString()
    }

    private fun section(text: String) = TextView(this).apply {
        this.text = text
        textSize = 18f
        setTypeface(typeface, Typeface.BOLD)
        setPadding(0, dp(22), 0, dp(6))
    }

    private fun value(text: String) = TextView(this).apply {
        this.text = text
        textSize = 14f
        setPadding(0, dp(3), 0, dp(8))
    }

    private fun sourceButtonContainer() = LinearLayout(this).apply {
        orientation = if (WindowLayoutPolicy.stackSourceButtons(resources.configuration.screenWidthDp, resources.configuration.fontScale)) LinearLayout.VERTICAL else LinearLayout.HORIZONTAL
        gravity = Gravity.START
    }

    private fun button(text: String, onClick: () -> Unit) = Button(this).apply {
        this.text = text
        setOnClickListener { onClick() }
    }

    private fun toast(message: String) = Toast.makeText(this, message, Toast.LENGTH_LONG).show()
    private fun dp(value: Int): Int = (value * resources.displayMetrics.density).toInt()
}
