"""Check real DVD exports retain subtitle payloads, timestamps and canvas."""
import json
from pathlib import Path
import subprocess
import sys

root = Path(sys.argv[1])


def packets(path, stream):
    data = json.loads(subprocess.check_output([
        'ffprobe', '-v', 'error', '-select_streams', stream, '-show_packets',
        '-show_entries', 'packet=pts_time,data_hash', '-show_data_hash', 'sha256',
        '-of', 'json', str(path),
    ]))
    return data['packets']


for source in ('demux-folder', 'demux-iso'):
    subtitles = packets(root / (source + '.mkv'), 's')
    audio = [p['data_hash'] for p in packets(root / (source + '.mkv'), 'a')]
    assert len(subtitles) == 2, subtitles
    for mode in ('elementary', 'vob'):
        output = root / (source + '-' + mode)
        assert packets(output / 'track-02.idx', 's') == subtitles
        assert [p['data_hash'] for p in packets(output / 'track-01.ac3', 'a')] == audio
        idx = (output / 'track-02.idx').read_text()
        assert 'size: 720x576' in idx and 'palette: ffffff, 000000' in idx, idx
print('Authored DVD demux preserves subtitle packet payload/timing, palette, canvas and AC3 payload PASS')
