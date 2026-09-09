MattMux 1.2.0 — Offline Bundle Builder
=======================================

WHY THIS BUILDER EXISTS
This package contains MattMux itself, but not copied third-party binaries yet.
Double-click BUILD-OFFLINE.cmd once on a Windows x64 PC with internet access.
The builder downloads the exact FFmpeg/FFprobe and MediaInfo versions MattMux
1.2.0 expects, verifies their SHA-256 values, and creates:

  MattMux-1.2.0-Fully-Portable-Offline.zip

That resulting ZIP is the one to keep on a USB drive / NAS / archive and use
on other computers. Those computers will not need to download the tools again.

The final folder stores everything under MattMuxData next to MattMux-Portable.exe.
No installer is required.

SECURITY
The builder validates:
- the pinned BtbN FFmpeg release checksum manifest;
- the FFmpeg archive against that verified manifest;
- the MediaInfo 26.05 x64 archive against its pinned SHA-256.

PUBLIC REDISTRIBUTION
FFmpeg and MediaInfo are third-party open-source projects. If you redistribute
a bundle publicly, review and comply with their licenses and any corresponding
source-code obligations. For personal/offline use, keep the THIRD_PARTY.txt
created by the builder with the package.
