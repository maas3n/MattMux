# MattMux Android native licensing policy

MattMux Android/ChromeOS keeps **FFmpeg 9.0.1 itself in an LGPL-only dynamically linked configuration**, while DVD title discovery uses **libdvdnav 6.1.1 + libdvdread 6.1.3**, matching the proven DVDNav Beta 1 scanner design.

This is an engineering compliance note, not legal advice. libdvdnav and libdvdread are GPL components; the Android package must ship their notices and corresponding source, and redistribution must be reviewed for the GPL obligations created by linking those components into the JNI native library.

## Pinned native baseline

- FFmpeg: 9.0.1, LGPL configuration, GPL/nonfree disabled
- libudfread: 1.1.2, LGPL-2.1-or-later, separate shared library
- libdvdnav: 6.1.1, GPL, statically linked into `libmattmux_jni.so`
- libdvdread: 6.1.3, GPL, statically linked into `libmattmux_jni.so`
- Android NDK: 30.0.16248370
- ABIs: `arm64-v8a`, `x86_64`
- CSS decryption/circumvention: not included

FFmpeg's `dvdvideo` demuxer remains disabled in the Android FFmpeg build. libdvdnav/libdvdread are used only to inspect staged IFO metadata and choose the longest DVD title. MattMux then converts that selected global title into its existing cell/chapter plan and performs the normal stream-copy remux through libav.

Every Android release must publish the exact FFmpeg, libudfread, libdvdnav and libdvdread source archives used by CI, retain their license notices, preserve 16 KB ELF alignment, and reject `libdvdcss` and unapproved codec dependencies.
