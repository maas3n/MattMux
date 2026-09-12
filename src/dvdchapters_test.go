package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeDVDTime(t *testing.T) {
	got, err := decodeDVDTime([]byte{0x01, 0x23, 0x45, 0xD5}) // 01:23:45 + 15/30
	if err != nil {
		t.Fatal(err)
	}
	want := time.Hour + 23*time.Minute + 45*time.Second + 500*time.Millisecond
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}

	got, err = decodeDVDTime([]byte{0x00, 0x00, 0x00, 0x41}) // 1/25 sec
	if err != nil {
		t.Fatal(err)
	}
	if got != 40*time.Millisecond {
		t.Fatalf("got %v want 40ms", got)
	}
}

func TestReadDVDChaptersSimple(t *testing.T) {
	root := t.TempDir()
	videoTS := filepath.Join(root, "VIDEO_TS")
	if err := os.Mkdir(videoTS, 0755); err != nil {
		t.Fatal(err)
	}

	vmg := make([]byte, 2*dvdSectorSize)
	copy(vmg, []byte("DVDVIDEO-VMG"))
	binary.BigEndian.PutUint32(vmg[0xC4:], 1)
	tt := dvdSectorSize
	binary.BigEndian.PutUint16(vmg[tt:], 1)
	binary.BigEndian.PutUint32(vmg[tt+4:], 19)
	entry := tt + 8
	binary.BigEndian.PutUint16(vmg[entry+2:], 3)
	vmg[entry+6] = 1
	vmg[entry+7] = 1
	if err := os.WriteFile(filepath.Join(videoTS, "VIDEO_TS.IFO"), vmg, 0644); err != nil {
		t.Fatal(err)
	}

	vts := make([]byte, 3*dvdSectorSize)
	copy(vts, []byte("DVDVIDEO-VTS"))
	binary.BigEndian.PutUint32(vts[0xC8:], 1)
	binary.BigEndian.PutUint32(vts[0xCC:], 2)

	ptt := dvdSectorSize
	binary.BigEndian.PutUint16(vts[ptt:], 1)
	binary.BigEndian.PutUint32(vts[ptt+4:], 23)
	binary.BigEndian.PutUint32(vts[ptt+8:], 12)
	for i := 0; i < 3; i++ {
		off := ptt + 12 + i*4
		binary.BigEndian.PutUint16(vts[off:], 1)
		binary.BigEndian.PutUint16(vts[off+2:], uint16(i+1))
	}

	pgci := 2 * dvdSectorSize
	binary.BigEndian.PutUint16(vts[pgci:], 1)
	pgcRel := 16
	pgcSize := 0xF0 + 3*24
	binary.BigEndian.PutUint32(vts[pgci+4:], uint32(pgcRel+pgcSize-1))
	binary.BigEndian.PutUint32(vts[pgci+8+4:], uint32(pgcRel))

	pgc := pgci + pgcRel
	vts[pgc+2] = 3
	vts[pgc+3] = 3
	binary.BigEndian.PutUint16(vts[pgc+0xE6:], 0xEC)
	binary.BigEndian.PutUint16(vts[pgc+0xE8:], 0xF0)
	copy(vts[pgc+0xEC:], []byte{1, 2, 3})
	setCellTime(vts[pgc+0xF0+0*24:], 0x10, 0)
	setCellTime(vts[pgc+0xF0+1*24:], 0x20, 0)
	setCellTime(vts[pgc+0xF0+2*24:], 0x30, 0)

	if err := os.WriteFile(filepath.Join(videoTS, "VTS_01_0.IFO"), vts, 0644); err != nil {
		t.Fatal(err)
	}

	loc, chapters, err := ReadDVDChapters(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if loc.TitleSet != 1 || loc.TitleSetTitle != 1 || len(chapters) != 3 {
		t.Fatalf("unexpected mapping/result: %+v chapters=%d", loc, len(chapters))
	}
	starts := []time.Duration{0, 10 * time.Second, 30 * time.Second}
	durations := []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second}
	for i := range chapters {
		if chapters[i].Start != starts[i] || chapters[i].Duration != durations[i] {
			t.Fatalf("chapter %d = start %v duration %v", i+1, chapters[i].Start, chapters[i].Duration)
		}
	}
}

func TestAngleOneDoesNotDoubleCount(t *testing.T) {
	pgc := dvdPGC{
		programs:   2,
		cells:      3,
		programMap: []byte{1, 3},
		cellData:   make([]byte, 3*24),
	}
	// Valid angle block flags include block type 1 (0x10) plus the mode bits.
	setCellTime(pgc.cellData[0*24:], 0x05, 0x50) // first of angle block
	setCellTime(pgc.cellData[1*24:], 0x07, 0xD0) // last of angle block
	setCellTime(pgc.cellData[2*24:], 0x10, 0x00) // normal

	starts, total, err := pgcProgramTimeline(pgc)
	if err != nil {
		t.Fatal(err)
	}
	if starts[0] != 0 || starts[1] != 5*time.Second || total != 15*time.Second {
		t.Fatalf("starts=%v total=%v", starts, total)
	}
}

func TestOrphanMiddleAngleCellRejected(t *testing.T) {
	pgc := dvdPGC{
		programs:   1,
		cells:      2,
		programMap: []byte{1},
		cellData:   make([]byte, 2*24),
	}
	setCellTime(pgc.cellData[0*24:], 0x05, 0x90) // block type 1, middle mode without a start
	setCellTime(pgc.cellData[1*24:], 0x10, 0x00)
	if _, _, err := pgcProgramTimeline(pgc); err == nil {
		t.Fatal("orphan middle angle cell was accepted")
	}
}

func TestUnterminatedAngleBlockRejected(t *testing.T) {
	pgc := dvdPGC{
		programs:   1,
		cells:      1,
		programMap: []byte{1},
		cellData:   make([]byte, 24),
	}
	setCellTime(pgc.cellData, 0x05, 0x50)
	if _, _, err := pgcProgramTimeline(pgc); err == nil {
		t.Fatal("unterminated angle block was accepted")
	}
}

func setCellTime(cell []byte, secondsBCD, category byte) {
	cell[0] = category
	cell[4] = 0x00
	cell[5] = 0x00
	cell[6] = secondsBCD
	cell[7] = 0xC0
}
