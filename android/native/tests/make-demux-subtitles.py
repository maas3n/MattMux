from pathlib import Path
import sys
p=Path(sys.argv[1])
spu=bytes.fromhex('00000006 1111 00000006 030123 04fff0 05000001000001 0600040005 01ff');spu=len(spu).to_bytes(2,'big')+spu[2:]
def pack(ms):
 pts=ms*90
 ts=bytes([0x21|((pts>>29)&14),(pts>>22)&255,((pts>>14)&254)|1,(pts>>7)&255,((pts<<1)&254)|1])
 data=b'\x80\x80\x05'+ts+b'\x20'+spu
 block=bytes.fromhex('000001ba4400040004010189c3f8')+b'\x00\x00\x01\xbd'+len(data).to_bytes(2,'big')+data
 n=2048-len(block)-6
 return block+b'\x00\x00\x01\xbe'+n.to_bytes(2,'big')+b'\xff'*n
(p/'input.sub').write_bytes(pack(200)+pack(600))
(p/'input.idx').write_text('# VobSub index file, v7 (do not modify this line!)\nsize: 720x576\npalette: 000000, ffffff, ff0000, 00ff00, 0000ff, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000\nid: en, index: 0\ntimestamp: 00:00:00:200, filepos: 000000000\ntimestamp: 00:00:00:600, filepos: 000000800\n')
