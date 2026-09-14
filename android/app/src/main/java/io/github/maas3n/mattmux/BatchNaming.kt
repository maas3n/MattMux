package io.github.maas3n.mattmux

internal object BatchNaming {
    private val invalid = Regex("[<>:\"/\\\\|?*\\u0000-\\u001F]")

    fun outputName(movieName: String): String {
        var base = movieName.trim().replace(invalid, "_").trimEnd('.', ' ')
        if (base.isBlank()) base = "DVD"
        return "$base.mkv"
    }
}
