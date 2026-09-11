"""Wrap an original generated DVD program stream in UDF; split at an awkward VOB boundary."""
import io
from pathlib import Path
import sys
import pycdlib
root = Path(sys.argv[1])
data = (root / 'input.vob').read_bytes()
assert len(data) % 2048 == 0
split = (len(data) // 4096) * 2048
folder = root / 'VIDEO_TS'
folder.mkdir(exist_ok=True)
(folder / 'VTS_01_1.VOB').write_bytes(data[:split])
(folder / 'VTS_01_2.VOB').write_bytes(data[split:])
(folder / 'VIDEO_TS.IFO').write_bytes(b'IFO byte-read fixture; planner tested separately')
iso = pycdlib.PyCdlib()
iso.new(udf='2.60')
iso.add_directory(iso_path='/VIDEO_TS', udf_path='/VIDEO_TS')
for path in sorted(folder.iterdir()):
    content = path.read_bytes()
    iso.add_fp(io.BytesIO(content), len(content), iso_path='/VIDEO_TS/' + path.name + ';1', udf_path='/VIDEO_TS/' + path.name)
iso.write(str(root / 'fixture.iso'))
iso.close()
