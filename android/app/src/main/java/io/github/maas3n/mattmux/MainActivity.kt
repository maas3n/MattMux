package io.github.maas3n.mattmux

import android.app.Activity
import android.content.Intent
import android.graphics.Typeface
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import android.view.Gravity
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
    }

    private val engine: RemuxEngine = AndroidNativeRemuxEngine()
    private lateinit var billing: BillingManager

    private lateinit var sourceValue: TextView
    private lateinit var outputValue: TextView
    private lateinit var proValue: TextView
    private lateinit var billingValue: TextView
    private lateinit var buyButton: Button
    private lateinit var remuxButton: Button

    private var sourceUri: Uri? = null
    private var outputUri: Uri? = null
    private var proOwned = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(buildUi())

        billing = BillingManager(this, this)
        billing.start()
        updateRemuxButton()
    }

    override fun onDestroy() {
        billing.close()
        super.onDestroy()
    }

    override fun onBillingState(state: BillingManager.State) {
        runOnUiThread {
            proOwned = state.proOwned
            proValue.text = if (state.proOwned) {
                "MattMux Pro: unlocked"
            } else {
                "MattMux Pro: not unlocked"
            }
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
        val uri = data?.data ?: return

        val flags = data.flags and (Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_WRITE_URI_PERMISSION)
        try {
            contentResolver.takePersistableUriPermission(uri, flags)
        } catch (_: SecurityException) {
            // Some providers grant temporary access only; the current session still works.
        }

        when (requestCode) {
            REQUEST_SOURCE_ISO, REQUEST_SOURCE_FOLDER -> {
                sourceUri = uri
                sourceValue.text = describeUri(uri)
            }
            REQUEST_OUTPUT_FOLDER -> {
                outputUri = uri
                outputValue.text = describeUri(uri)
            }
        }
        updateRemuxButton()
    }

    private fun buildUi(): ViewGroup {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(28), dp(24), dp(28), dp(28))
        }

        val title = TextView(this).apply {
            text = "MattMux"
            textSize = 30f
            setTypeface(typeface, Typeface.BOLD)
        }
        root.addView(title)

        root.addView(TextView(this).apply {
            text = "DVD / VIDEO_TS / ISO → MKV without transcoding"
            textSize = 16f
            setPadding(0, dp(4), 0, dp(22))
        })

        root.addView(section("Source"))
        sourceValue = value("No source selected")
        root.addView(sourceValue)

        val sourceButtons = horizontalRow()
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
        buyButton = button("Buy MattMux Pro") {
            billing.launchProPurchase(this)
        }
        root.addView(buyButton)

        root.addView(section("Remux"))
        root.addView(value("ChromeOS-ready file picking and Play Billing are wired. The Android-native FFmpeg/libdvdnav engine is the remaining implementation milestone."))
        remuxButton = button("Remux to MKV") {
            when {
                !engine.isAvailable -> toast(engine.unavailableReason ?: "Remux engine unavailable.")
                !proOwned -> billing.launchProPurchase(this)
                else -> toast("Remux engine is ready to be invoked.")
            }
        }
        root.addView(remuxButton)

        val scroll = ScrollView(this)
        scroll.addView(root)
        return scroll
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
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION)
        }
        startActivityForResult(intent, REQUEST_SOURCE_FOLDER)
    }

    private fun chooseOutputFolder() {
        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE).apply {
            addFlags(
                Intent.FLAG_GRANT_READ_URI_PERMISSION or
                    Intent.FLAG_GRANT_WRITE_URI_PERMISSION or
                    Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION
            )
        }
        startActivityForResult(intent, REQUEST_OUTPUT_FOLDER)
    }

    private fun updateRemuxButton() {
        val hasPaths = sourceUri != null && outputUri != null
        remuxButton.isEnabled = hasPaths
        remuxButton.text = when {
            !engine.isAvailable -> "Remux engine pending"
            !proOwned -> "Unlock Pro to remux"
            else -> "Remux to MKV"
        }
    }

    private fun describeUri(uri: Uri): String {
        contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
            val index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (index >= 0 && cursor.moveToFirst()) {
                return cursor.getString(index)
            }
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

    private fun horizontalRow() = LinearLayout(this).apply {
        orientation = LinearLayout.HORIZONTAL
        gravity = Gravity.START
    }

    private fun button(text: String, onClick: () -> Unit) = Button(this).apply {
        this.text = text
        setOnClickListener { onClick() }
    }

    private fun toast(message: String) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show()
    }

    private fun dp(value: Int): Int = (value * resources.displayMetrics.density).toInt()
}
