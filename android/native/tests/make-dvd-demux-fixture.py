#!/usr/bin/env python3
"""Author a small real DVD with MPEG2, AC3, bitmap subtitles and two chapters."""
import pathlib
import struct
import subprocess
import sys
import zlib
from xml.sax.saxutils import escape

root = pathlib.Path(sys.argv[1]).resolve()
root.mkdir(parents=True, exist_ok=True)

def run(*args, **kwargs):
    subprocess.run(args, check=True, **kwargs)

def chunk(kind, payload):
    return struct.pack('>I', len(payload)) + kind + payload + struct.pack('>I', zlib.crc32(kind + payload))

# A two-colour subtitle bitmap; all assets are generated, with no external media.
w, h = 160, 32
border = b'\x00' + bytes(w * 3)
row = b'\x00' + bytes(6) + bytes([255, 255, 255]) * (w - 4) + bytes(6)
png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 2, 0, 0, 0))
png += chunk(b'IDAT', zlib.compress(border * 2 + row * (h - 4) + border * 2)) + chunk(b'IEND', b'')
(root / 'subtitle.png').write_bytes(png)
run('ffmpeg', '-v', 'error', '-f', 'lavfi', '-i', 'color=blue:size=720x576:rate=25',
    '-f', 'lavfi', '-i', 'sine=frequency=880:sample_rate=48000', '-t', '6',
    '-target', 'pal-dvd', '-bf', '2', '-y', str(root / 'movie.vob'))
image = escape(str(root / 'subtitle.png'))
(root / 'subs.xml').write_text(f'<subpictures><stream><spu start="00:00:01.00" end="00:00:02.00" image="{image}" transparent="000000" xoffset="280" yoffset="500" /><spu start="00:00:04.00" end="00:00:05.00" image="{image}" transparent="000000" xoffset="280" yoffset="500" /></stream></subpictures>')
with (root / 'movie.vob').open('rb') as src, (root / 'subtitled.vob').open('wb') as dst:
    run('spumux', '-m', 'dvd', str(root / 'subs.xml'), stdin=src, stdout=dst)
movie = escape(str(root / 'subtitled.vob'))
dest = escape(str(root / 'dvd'))
(root / 'dvd.xml').write_text(f'<dvdauthor dest="{dest}"><vmgm><menus><video format="pal" /></menus></vmgm><titleset><titles><video format="pal" /><audio lang="en" /><subpicture lang="en" /><pgc><vob file="{movie}" chapters="0,3" /></pgc></titles></titleset></dvdauthor>')
run('dvdauthor', '-x', str(root / 'dvd.xml'))

# One title deliberately repeats a cell whose MPEG clock starts over. Reuse of
# a VOB is valid DVD authoring, and exercises planner playback order as well.
reset = escape(str(root / 'clock-reset'))
(root / 'clock-reset.xml').write_text(f'<dvdauthor dest="{reset}"><vmgm><menus><video format="pal" /></menus></vmgm><titleset><titles><video format="pal" /><audio lang="en" /><subpicture lang="en" /><pgc><vob file="{movie}" /><vob file="{movie}" /></pgc></titles></titleset></dvdauthor>')
run('dvdauthor', '-x', str(root / 'clock-reset.xml'))
