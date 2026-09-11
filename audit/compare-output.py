import json, subprocess, sys
from pathlib import Path

def probe(p):
    raw=json.loads(subprocess.check_output(['ffprobe','-v','error','-show_streams','-show_packets','-show_chapters','-show_data_hash','sha256','-of','json',p]))
    streams=[{k:s.get(k) for k in ('codec_name','codec_type','width','height','channels','sample_rate','extradata_hash')} for s in raw['streams']]
    packets=[{k:p.get(k) for k in ('stream_index','data_hash','size','pts_time','dts_time','duration_time')} for p in raw['packets']]
    chapters=[{k:c.get(k) for k in ('start_time','end_time')} for c in raw.get('chapters',[])]
    assert packets and len(chapters)==2,(p,'missing media/chapters')
    result={'streams':streams,'packets':packets,'chapters':chapters}
    Path(p).with_suffix('.fingerprint.json').write_text(json.dumps(result,indent=2))
    return result

a,b=map(probe,sys.argv[1:])
assert a==b,'Folder/ISO payload, timestamp, stream or chapter mismatch'
print('Published runtime folder/ISO exact fingerprint PASS; packets:',len(a['packets']),'chapters:',a['chapters'])
