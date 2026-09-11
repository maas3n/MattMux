"""Generate original byte-pattern data, then put identical files in a UDF image."""
import io
from pathlib import Path
import sys
import pycdlib

root = Path(sys.argv[1])
folder = root / 'VIDEO_TS'
folder.mkdir(parents=True, exist_ok=True)
iso = pycdlib.PyCdlib()
iso.new(udf='2.60')
iso.add_directory(iso_path='/VIDEO_TS', udf_path='/VIDEO_TS')
# More than one UDF/AVIO buffer, non-sector-aligned IFOs and two VOB parts.
for index, (name, size) in enumerate([
    ('VIDEO_TS.IFO', 4097), ('VTS_01_0.IFO', 8193),
    ('VTS_01_1.VOB', 131072), ('VTS_01_2.VOB', 196608),
]):
    data = bytes((i * 37 + (i >> 8) + index * 71) % 256 for i in range(size))
    (folder / name).write_bytes(data)
    iso.add_fp(io.BytesIO(data), len(data), iso_path=f'/VIDEO_TS/{name};1', udf_path=f'/VIDEO_TS/{name}')
iso.write(str(root / 'fixture.iso'))
iso.close()
