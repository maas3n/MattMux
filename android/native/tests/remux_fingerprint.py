"""Compare demuxed streams, compressed packets, common-clock timing and chapters.

Container hashes, UIDs and stream order are deliberately excluded. Timestamps
use one common origin; independently zeroing every stream would hide A/V drift.
"""
import json
from pathlib import Path
import subprocess
import sys
from decimal import Decimal


def fingerprint(path):
    raw = json.loads(subprocess.check_output([
        'ffprobe', '-v', 'error', '-show_streams', '-show_packets', '-show_chapters',
        '-show_data_hash', 'sha256', '-of', 'json', str(path),
    ]))
    packets = raw.get('packets', [])
    if not packets:
        raise AssertionError(f'{path}: no packets')
    origin = min(Decimal(p['pts_time']) for p in packets if 'pts_time' in p)
    streams = []
    for stream in raw['streams']:
        if stream['codec_type'] not in ('video', 'audio', 'subtitle'):
            continue
        props = {key: stream.get(key) for key in (
            'codec_type', 'codec_name', 'width', 'height', 'sample_rate', 'channels',
            'channel_layout', 'sample_aspect_ratio', 'extradata_hash',
        )}
        props['language'] = stream.get('tags', {}).get('language', 'und')
        props['disposition'] = stream.get('disposition', {})
        items = []
        for packet in packets:
            if packet['stream_index'] != stream['index']:
                continue
            item = {key: packet.get(key) for key in ('data_hash', 'size', 'flags')}
            for field in ('pts_time', 'dts_time'):
                item[field] = str(Decimal(packet[field]) - origin) if field in packet else None
            item['duration_time'] = packet.get('duration_time')
            items.append(item)
        props['packets'] = items
        streams.append(props)
    streams.sort(key=lambda item: json.dumps(item, sort_keys=True))
    return {'timeline_origin': str(origin), 'streams': streams, 'chapters': [
        {key: chapter[key] for key in ('start_time', 'end_time')}
        for chapter in raw.get('chapters', [])
    ]}


def first_difference(a, b, path='$'):
    if type(a) is not type(b):
        return f'{path}: type differs'
    if isinstance(a, dict):
        if a.keys() != b.keys():
            return f'{path}: fields differ'
        for key in a:
            diff = first_difference(a[key], b[key], f'{path}.{key}')
            if diff:
                return diff
    elif isinstance(a, list):
        if len(a) != len(b):
            return f'{path}: count {len(a)} != {len(b)}'
        for index, (left, right) in enumerate(zip(a, b)):
            diff = first_difference(left, right, f'{path}[{index}]')
            if diff:
                return diff
    elif a != b:
        return f'{path}: {a!r} != {b!r}'
    return None


if __name__ == '__main__':
    paths = [Path(arg) for arg in sys.argv[1:]]
    if len(paths) not in (2, 3):
        raise SystemExit('usage: remux_fingerprint.py folder.mkv iso.mkv [source.vob]')
    results = [fingerprint(path) for path in paths]
    for path, result in zip(paths, results):
        path.with_suffix('.fingerprint.json').write_text(json.dumps(result, indent=2) + '\n')
    difference = first_difference(results[0], results[1])
    if difference:
        raise SystemExit(difference)
    if abs(Decimal(results[0]['timeline_origin'])) > Decimal('0.001'):
        raise SystemExit('Output timestamps are not aligned with zero-based chapters')
    if len(results[0]['chapters']) != 2:
        raise SystemExit('Expected two chapters in generated remux fixture')
    if len(results) == 3:
        def payloads(result):
            return sorted((s['codec_type'], s['codec_name'],
                [(p['data_hash'], p['size']) for p in s['packets']]) for s in result['streams'])
        difference = first_difference(payloads(results[0]), payloads(results[2]))
        if difference:
            raise SystemExit('Source compressed payload mismatch: ' + difference)
    print('Folder/ISO stream, packet payload, common-clock timestamp and chapter parity PASS')
