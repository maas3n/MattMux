package io.github.maas3n.mattmux

import org.junit.Assert.assertEquals
import org.junit.Test

class BatchNamingTest {
    @Test fun createsMovieNamedMkv() { assertEquals("Movie Title.mkv", BatchNaming.outputName("Movie Title")) }
    @Test fun sanitizesPortableFilenameCharacters() { assertEquals("Movie_________.mkv", BatchNaming.outputName("Movie<>:\"/\\|?*")) }
    @Test fun fallsBackForBlankName() { assertEquals("DVD.mkv", BatchNaming.outputName("   ...   ")) }
}
