package io.github.maas3n.mattmux

import android.app.Activity
import android.app.Instrumentation
import android.os.Bundle
import java.io.File

/** Development APK only: exercise the packaged JNI binaries on real Android. */
class DemuxSmokeInstrumentation : Instrumentation() {
    override fun onCreate(arguments: Bundle?) {
        super.onCreate(arguments)
        start()
    }
    override fun onStart() {
        val result = Bundle()
        val root = File(targetContext.cacheDir, "demux-smoke").apply { mkdirs() }
        try {
            val runtime = AndroidNativeRemuxEngine()
            check(runtime.isAvailable) { runtime.unavailableReason ?: "Native runtime unavailable" }
            val native = AdvancedMergerNative()
            for (name in listOf("mixed.mkv", "subtitles.mkv", "raw-h264.mkv")) {
                val source = File(root, name)
                targetContext.assets.open("demux-smoke/$name").use { input -> source.outputStream().use { input.copyTo(it) } }
                val metadata = String(MediaInfoNative().metadata(source.absolutePath), Charsets.UTF_8)
                check(metadata.contains("Matroska")) { "MediaInfo did not identify $name: $metadata" }
                val indexes = native.probe(source.absolutePath).map { it.substringBefore('\t').toInt() }.filter { it >= 0 }.toIntArray()
                val output = File(root, "$name-export").apply { mkdirs() }
                native.demux(source.absolutePath, output.absolutePath, indexes, true, false)?.let { error(it) }
                check(output.listFiles()?.all { it.length() > 0 } == true) { "Empty export: $name" }
                when (name) {
                    "mixed.mkv" -> {
                        check(File(output, "track-04.srt").readText().contains("Hello"))
                        check(File(output, "Chapters.txt").readText().contains("CHAPTER01NAME=Opening"))
                    }
                    "subtitles.mkv" -> {
                        val idx = File(output, "track-01.idx").readText()
                        check(idx.contains("00:00:00:200") && idx.contains("00:00:00:600") && idx.contains("palette:")) { idx }
                        check(File(output, "track-01.sub").length() > 0)
                    }
                    "raw-h264.mkv" -> check(File(output, "track-00.h264").length() > 0)
                }
            }
            result.putString("stream", "MATTMUX_DEMUX_SMOKE_PASS")
            finish(Activity.RESULT_OK, result)
        } catch (error: Throwable) {
            result.putString("stream", error.stackTraceToString())
            finish(Activity.RESULT_CANCELED, result)
        } finally { root.deleteRecursively() }
    }
}
