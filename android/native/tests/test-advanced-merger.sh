#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
HOST_WORK="${1:?Pass completed host-remux test directory}"
WORK="$HOST_WORK/merger"
mkdir -p "$WORK"
printf '1\n00:00:00,000 --> 00:00:00,900\nHello\n' > "$WORK/captions.srt"
printf ';FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1000\ntitle=Opening\n' > "$WORK/chapters.txt"
ffmpeg -v error -f lavfi -i color=size=32x32:rate=25:duration=1 -f lavfi -i sine=duration=1 \
  -i "$WORK/captions.srt" -f ffmetadata -i "$WORK/chapters.txt" \
  -map 0:v -map 0:v -map 1:a -map 1:a -map 2:s -map_chapters 3 \
  -c:v mpeg4 -c:a pcm_s16le -c:s srt -y "$WORK/mixed.mkv"
ffmpeg -v error -f lavfi -i sine=duration=1 -c:a ac3 -y "$WORK/raw.ac3"
for ext in h264 m2v vob; do
  codec=mpeg2video; if [[ "$ext" == h264 ]]; then codec=libx264; fi
  ffmpeg -v error -f lavfi -i color=size=32x32:rate=25:duration=1 -c:v "$codec" -bf 0 -y "$WORK/raw.$ext"
done
javac -d "$HOST_WORK/classes" "$ROOT/android/native/tests/java/io/github/maas3n/mattmux/AdvancedMergerNative.java"
java -cp "$HOST_WORK/classes" io.github.maas3n.mattmux.AdvancedMergerNative "$HOST_WORK/libmattmux_host_test.so" "$WORK"
python3 - "$WORK" <<'PY'
import json, subprocess, sys
from pathlib import Path
p=Path(sys.argv[1])
def probe(file):
    return json.loads(subprocess.check_output(['ffprobe','-v','error','-show_streams','-show_chapters','-of','json',str(file)]))
r=probe(p/'merged.mkv')
assert [s['codec_name'] for s in r['streams']]==['mpeg4','pcm_s16le','subrip','ac3','subrip'],r
assert len(r['chapters'])==1 and r['chapters'][0]['tags']['title']=='Opening',r
embedded=probe(p/'embedded-chapters.mkv')
assert len(embedded['chapters'])==1 and embedded['chapters'][0]['tags']['title']=='Opening',embedded
assert not probe(p/'no-chapters.mkv')['chapters']
def hashes(file,stream):
    r=json.loads(subprocess.check_output(['ffprobe','-v','error','-select_streams',str(stream),'-show_packets','-show_data_hash','sha256','-of','json',str(file)]))
    return [packet['data_hash'] for packet in r['packets']]
assert hashes(p/'mixed.mkv',1)==hashes(p/'merged.mkv',0),'video packets changed'
assert hashes(p/'raw.ac3',0)==hashes(p/'merged.mkv',3),'audio packets changed'
PY
