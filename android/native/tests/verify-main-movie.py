"""Verify every decoded frame/sample is from the complete intended main movie.

dvdauthor changes MPEG-2 GOP headers while authoring, so pre-authoring compressed
packet hashes are not an oracle. Folder/ISO compressed parity is checked separately.
"""
from pathlib import Path
import subprocess
import sys


def media_hash(path, stream):
    return subprocess.check_output([
        "ffmpeg", "-v", "error", "-i", str(path), "-map", f"0:{stream}:0",
        "-f", "hash", "-hash", "sha256", "-",
    ], text=True).strip()


work = Path(sys.argv[1])
movies = {}
for name in ("long-first", "long-last", "single"):
    reference = work / f"{name}-main.vob"
    expected = tuple(media_hash(reference, stream) for stream in ("v", "a"))
    for source in ("folder", "iso"):
        output = work / f"{name}-{source}.mkv"
        actual = tuple(media_hash(output, stream) for stream in ("v", "a"))
        assert actual == expected, f"{output}: video/audio differs from the complete main movie"
    movies[name] = expected
assert movies["long-first"] != movies["long-last"], "Different DVD sources produced duplicate movies"
print("Full main-movie decoded video/audio identity and distinct-source outputs PASS")
