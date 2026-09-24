#!/usr/bin/env python3
"""Fail publication when any package, source, notice, or checksum is missing."""
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
version = sys.argv[2]
required = [
    f"MattMux-{version}-Windows-Setup.exe",
    f"MattMux-{version}-Windows-All-in-One.exe",
    f"MattMux-{version}-Windows-Portable.zip",
    f"MattMux-{version}-Linux-amd64.deb",
    f"MattMux-{version}-Linux-amd64.tar.gz",
    f"MattMux-{version}-Linux-amd64Standalone",
    f"MattMux-{version}-Linux-Library-Sources.tar.gz",
    f"MattMux-{version}-Android.apk",
    f"MattMux-{version}-Source.tar.gz",
    f"MattMux-{version}-THIRD-PARTY.md",
    "DVDNAV_COPYING.txt", "DVDREAD_COPYING.txt", "LIBUDFREAD_COPYING.txt",
    "libdvdnav-6.1.1-source.tar.gz", "libdvdread-6.1.3-source.tar.gz",
    "libudfread-1.1.2-source.tar.gz", "FFmpeg-9.0.1-source.tar.xz",
    "SHA256SUMS-Windows.txt", "SHA256SUMS-Linux.txt", "SHA256SUMS-Android.txt",
    "SHA256SUMS.txt",
]
missing = [name for name in required if not (root / name).is_file() or (root / name).stat().st_size == 0]
if missing:
    raise SystemExit("Incomplete cross-platform release: " + ", ".join(missing))
print(f"Complete release: {len(required)} required assets present")
