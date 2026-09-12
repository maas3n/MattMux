"""Create a valid sparse-timestamp MPEG-PS fixture and check its repaired timing."""
import json
from pathlib import Path
import subprocess
import sys
from decimal import Decimal


def packets(path, genpts=False):
    args = ['ffprobe', '-v', 'error']
    if genpts:
        args += ['-fflags', '+genpts']
    return json.loads(subprocess.check_output(args + [
        '-show_packets', '-show_data_hash', 'sha256', '-of', 'json', str(path),
    ]))['packets']


def make(source, destination):
    data = bytearray(source.read_bytes())
    offset = 0
    stamped = 0
    while True:
        offset = data.find(b'\x00\x00\x01', offset)
        if offset < 0:
            break
        stream_id = data[offset + 3]
        if stream_id == 0xba:
            assert data[offset + 4] & 0xc0 == 0x40, 'Expected MPEG-2 pack header'
            offset += 14 + (data[offset + 13] & 7)
            continue
        if stream_id < 0xbb:
            offset += 4
            continue
        size = int.from_bytes(data[offset + 4:offset + 6], 'big')
        if stream_id != 0xe0:
            offset += 6 + size
            continue
        assert size > 0 and data[offset + 6] & 0xc0 == 0x80
        flags = data[offset + 7] & 0xc0
        assert flags in (0, 0x80, 0xc0)
        if flags:
            stamped += 1
            if stamped > 1:
                length = 10 if flags == 0xc0 else 5
                assert data[offset + 8] >= length
                # Keep the initial clock anchor, then replace optional video
                # PTS/DTS fields with PES stuffing. Payloads/sector sizes stay exact.
                data[offset + 7] &= 0x3f
                data[offset + 9:offset + 9 + length] = b'\xff' * length
        offset += 6 + size
    assert stamped > 2, 'Fixture needs multiple timestamped video PES packets'
    destination.write_bytes(data)
    raw = packets(destination)
    assert any(p['codec_type'] == 'video' and 'pts' not in p and 'dts' in p for p in raw)
    assert all('pts' in p for p in packets(destination, genpts=True)
               if p['codec_type'] in ('video', 'audio'))


def verify(source, output):
    expected = packets(source)
    actual = packets(output)
    # One shared origin preserves the audio/video offset, including B-frame order.
    def origin(items):
        return min(Decimal(p['pts_time']) for p in items
                   if p['codec_type'] in ('video', 'audio'))
    start_expected, start_actual = origin(expected), origin(actual)
    for kind in ('video', 'audio'):
        left = [p for p in expected if p['codec_type'] == kind]
        right = [p for p in actual if p['codec_type'] == kind]
        assert len(left) == len(right), (kind, len(left), len(right))
        for index, (a, b) in enumerate(zip(left, right)):
            assert a['data_hash'] == b['data_hash'], (kind, index, 'payload changed')
            difference = ((Decimal(a['pts_time']) - start_expected)
                          - (Decimal(b['pts_time']) - start_actual))
            # Matroska's 1 ms clock quantizes the original 90 kHz DVD clock.
            assert abs(difference) <= Decimal('0.001'), (kind, index, difference)
    print('Sparse PTS repair preserves compressed payloads, B-frame timing and A/V offset PASS')


if __name__ == '__main__':
    mode, source, destination = sys.argv[1:]
    {'make': make, 'verify': verify}[mode](Path(source), Path(destination))
