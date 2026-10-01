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

# Every selected elementary packet must survive both clock domains, with the
# same A/V/subtitle offset and B-frame ordering (no timestamp clamping).
for kind in ('folder', 'iso'):
    video = packets(root / f'clock-reset-{kind}.mkv', 'v')
    # dvdauthor includes the audio tail in the last NAV interval; the clock
    # duration can exceed the nominal six seconds by less than one frame.
    clock_shift = float(video[len(video) // 2]['pts_time']) - float(video[0]['pts_time'])
    assert 6 <= clock_shift < 6.04, clock_shift
    for stream in ('v', 'a', 's'):
        original = packets(root / f'demux-{kind}.mkv', stream)
        repeated = packets(root / f'clock-reset-{kind}.mkv', stream)
        assert len(repeated) == 2 * len(original), (kind, stream, len(original), len(repeated))
        for i, packet in enumerate(repeated):
            expected = original[i % len(original)]
            assert packet['data_hash'] == expected['data_hash'], (kind, stream, i)
            shift = clock_shift if i >= len(original) else 0
            assert abs(float(packet['pts_time']) - float(expected['pts_time']) - shift) < .002, (kind, stream, i, packet, expected)
    for mode in ('elementary', 'vob'):
        output = root / f'clock-reset-{kind}-{mode}'
        assert packets(output / 'track-02.idx', 's') == packets(root / f'clock-reset-{kind}.mkv', 's')
print('Clock-reset DVD folder/ISO preserves every A/V/subtitle packet and shared presentation timing PASS')
