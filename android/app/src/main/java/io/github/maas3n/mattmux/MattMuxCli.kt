package io.github.maas3n.mattmux

import android.content.Context
import android.net.Uri
import android.provider.DocumentsContract
import java.io.File

internal sealed class MattMuxCliCommand {
    data class Batch(val inputRoot: String, val outputRoot: String?, val logFile: String?) : MattMuxCliCommand()
    object Version : MattMuxCliCommand()
    object Help : MattMuxCliCommand()
}

internal object MattMuxCliSyntax {
    fun parse(commandLine: String): MattMuxCliCommand {
        val tokens = tokenize(commandLine).toMutableList()
        if (tokens.firstOrNull() == "mattmux-cli") tokens.removeAt(0)
        if (tokens.isEmpty() || tokens == listOf("--help") || tokens == listOf("-h")) return MattMuxCliCommand.Help
        if (tokens == listOf("--version")) return MattMuxCliCommand.Version
        require(tokens.removeFirstOrNull() == "--batch") { "Usage: mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]" }

        var logFile: String? = null
        val positional = mutableListOf<String>()
        var index = 0
        while (index < tokens.size) {
            val token = tokens[index]
            when {
                token == "--log" -> {
                    require(index + 1 < tokens.size) { "--log requires a filename" }
                    logFile = tokens[index + 1]
                    index += 2
                }
                token.startsWith("--log=") -> {
                    logFile = token.substringAfter("--log=").also { require(it.isNotBlank()) { "--log requires a filename" } }
                    index++
                }
                token.startsWith("-") -> error("Unknown option: $token")
                else -> { positional += token; index++ }
            }
        }
        require(positional.size in 1..2) { "Usage: mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]" }
        return MattMuxCliCommand.Batch(positional[0], positional.getOrNull(1), logFile)
    }

    private fun tokenize(text: String): List<String> {
        val out = mutableListOf<String>()
        val current = StringBuilder()
        var quote: Char? = null
        var escaped = false
        fun flush() { if (current.isNotEmpty()) { out += current.toString(); current.setLength(0) } }
        text.forEach { ch ->
            when {
                escaped -> { current.append(ch); escaped = false }
                ch == '\\' && quote != '\'' -> escaped = true
                quote != null && ch == quote -> quote = null
                quote != null -> current.append(ch)
                ch == '\'' || ch == '"' -> quote = ch
                ch.isWhitespace() -> flush()
                else -> current.append(ch)
            }
        }
        require(!escaped && quote == null) { "Unterminated quote or escape in command" }
        flush()
        return out
    }
}

internal class MattMuxCliRunner(private val context: Context) {
    @Volatile private var processor: AndroidBatchProcessor? = null

    fun cancel() { processor?.cancel() }

    fun execute(
        commandLine: String,
        emit: (String) -> Unit,
        progress: (Int, String) -> Unit,
    ): Int {
        return when (val command = MattMuxCliSyntax.parse(commandLine)) {
            MattMuxCliCommand.Help -> {
                emit("MattMux CLI ${BuildConfig.VERSION_NAME}")
                emit("Usage: mattmux-cli --batch [--log FILE] MOVIES_ROOT [OUTPUT_ROOT]")
                emit("Android uses Storage Access Framework content:// tree URIs for MOVIES_ROOT and OUTPUT_ROOT.")
                emit("Use the CLI tab folder buttons to insert valid URIs.")
                0
            }
            MattMuxCliCommand.Version -> { emit("MattMux CLI ${BuildConfig.VERSION_NAME} (Android/ChromeOS native)"); 0 }
            is MattMuxCliCommand.Batch -> runBatch(command, emit, progress)
        }
    }

    private fun runBatch(command: MattMuxCliCommand.Batch, emit: (String) -> Unit, progress: (Int, String) -> Unit): Int {
        val input = parseTreeUri(command.inputRoot, "MOVIES_ROOT")
        val output = command.outputRoot?.let { parseTreeUri(it, "OUTPUT_ROOT") }
        val writer = command.logFile?.let { name ->
            require(name == File(name).name && name != "." && name != "..") { "Android --log must be an app-private filename, not a filesystem path" }
            File(context.filesDir, name).bufferedWriter()
        }
        val localProcessor = AndroidBatchProcessor(context)
        processor = localProcessor
        return try {
            val result = localProcessor.run(input, output, progress, log = { line ->
                writer?.apply { appendLine(line); flush() }
                emit(line)
            })
            emit("Result: completed=${result.completed} failed=${result.failures.size} total=${result.total}${if (result.cancelled) " cancelled=true" else ""}")
            result.outputs.forEach { emit("Output: $it") }
            result.failures.forEach { emit("Failure: ${it.movie}: ${it.message}") }
            when {
                result.cancelled -> 130
                result.failures.isNotEmpty() -> 1
                else -> 0
            }
        } finally {
            writer?.close()
            processor = null
        }
    }

    private fun parseTreeUri(value: String, label: String): Uri {
        val uri = Uri.parse(value)
        require(uri.scheme == "content" && DocumentsContract.isTreeUri(uri)) { "$label must be a content:// document-tree URI" }
        return uri
    }
}
