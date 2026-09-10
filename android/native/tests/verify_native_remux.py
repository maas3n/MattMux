#!/usr/bin/env python3
import json, subprocess, sys
from pathlib import Path

def probe(path):
    return json.loads(subprocess.check_output(["ffprobe","-v","error","-show_streams","-show_chapters","-show_format","-of","json",path],text=True))
def signature(data):
    return sorted((s.get("codec_type"),s.get("codec_name")) for s in data.get("streams",[]) if s.get("codec_type") in {"video","audio","subtitle"})
def duration(data):
    try:return float(data["format"]["duration"])
    except (KeyError,TypeError,ValueError):return 0.0

def main(folder_mkv,iso_mkv,source_vob):
    folder,iso,source=map(probe,(folder_mkv,iso_mkv,source_vob))
    ss,fs,is_=map(signature,(source,folder,iso))
    if not ss or fs!=ss or is_!=ss: raise SystemExit(f"stream parity failed: source={ss} folder={fs} iso={is_}")
    fc,ic=folder.get("chapters",[]),iso.get("chapters",[])
    if len(fc)<2 or len(fc)!=len(ic): raise SystemExit(f"chapter parity failed: folder={len(fc)} iso={len(ic)}")
    fd,id_=duration(folder),duration(iso)
    if not (3.0<=fd<=6.0 and abs(fd-id_)<=0.15): raise SystemExit(f"duration parity failed: folder={fd:.3f}s iso={id_:.3f}s")
    for path in (folder_mkv,iso_mkv):
        if Path(path).stat().st_size<4096: raise SystemExit(f"output unexpectedly small: {path}")
    print(f"native remux parity passed: streams={fs}, chapters={len(fc)}, folder={fd:.3f}s iso={id_:.3f}s")

if __name__=="__main__":
    if len(sys.argv)!=4: raise SystemExit("usage: verify_native_remux.py <folder.mkv> <iso.mkv> <source.vob>")
    main(*sys.argv[1:])
