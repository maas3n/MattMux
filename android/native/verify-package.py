#!/usr/bin/env python3
"""Audit the actual ELF bytes in every APK/AAB ABI, including unexpected libraries."""
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import zipfile

EXPECTED = {'libavutil.so', 'libavcodec.so', 'libavformat.so', 'libudfread.so', 'libmattmux_jni.so'}
ABIS = {'arm64-v8a', 'x86_64'}

for filename in sys.argv[1:]:
    with zipfile.ZipFile(filename) as archive, tempfile.TemporaryDirectory() as tmp:
        prefix = 'base/' if Path(filename).suffix == '.aab' else ''
        libraries = [name for name in archive.namelist() if name.startswith(prefix + 'lib/') and name.endswith('.so')]
        found = {}
        for name in libraries:
            parts = name[len(prefix):].split('/')
            if len(parts) != 3:
                raise SystemExit(f'{filename}: unexpected library path {name}')
            _, abi, lib = parts
            found.setdefault(abi, set()).add(lib)
            # Never extract archive paths. Inspect bytes in one private regular file.
            elf = Path(tmp) / 'library.so'
            elf.write_bytes(archive.read(name))
            program = subprocess.check_output(['readelf', '-lW', str(elf)], text=True)
            loads = [line.split()[-1] for line in program.splitlines() if line.lstrip().startswith('LOAD ')]
            if not loads or any(int(value, 16) < 16384 for value in loads):
                raise SystemExit(f'{filename}: ELF LOAD alignment below 16 KB: {name}')
            dynamic = subprocess.check_output(['readelf', '-dW', str(elf)], text=True)
            needed = '\n'.join(line for line in dynamic.splitlines() if '(NEEDED)' in line)
            if re.search(r'dvdnav|dvdread|dvdcss|x264|x265|xvid|libav\w+\.so\.', needed, re.I):
                raise SystemExit(f'{filename}: forbidden or versioned dependency: {name}\n{needed}')
        if set(found) != ABIS or any(found[abi] != EXPECTED for abi in ABIS):
            raise SystemExit(f'{filename}: unexpected native package contents: {found}')
        for notice in ('COPYING.LGPLv2.1', 'LIBUDFREAD_COPYING.txt', 'ffmpeg-build-info.txt'):
            if not archive.read(prefix + 'assets/ffmpeg/' + notice):
                raise SystemExit(f'{filename}: missing/empty notice {notice}')
        print(f'{filename}: both ABIs, all native ELFs, 16 KB alignment, dependency and notice audit PASS')
