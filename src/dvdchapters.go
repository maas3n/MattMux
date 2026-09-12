package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	dvdSectorSize = 2048
	maxIFOSize    = 64 << 20 // 64 MiB; real DVD IFOs are normally far smaller.
)

// ErrNativeDVDChaptersUnsupported means that the source or DVD authoring is
// outside the intentionally small native parser. MattMux should fall back to
// ffprobe/FFmpeg (libdvdnav) when errors.Is(err, ErrNativeDVDChaptersUnsupported).
var ErrNativeDVDChaptersUnsupported = errors.New("native DVD chapter parser cannot safely handle this title")

type DVDTitleLocation struct {
	GlobalTitle      int
	TitleSet         int
	TitleSetTitle    int
	DeclaredChapters int
}

type DVDChapter struct {
	Number   int
	Start    time.Duration
	Duration time.Duration
	PGCN     uint16
	PGN      uint16
}

type dvdPTT struct {
	pgcn uint16
	pgn  uint16
}

type dvdPGC struct {
	programs     int
	cells        int
	playbackMode byte
	stillTime    byte
	programMap   []byte // 1-based entry-cell number per program
	cellData     []byte // cells * 24 bytes
}

// ReadDVDChapters reads chapter timestamps directly from a VIDEO_TS directory.
// It deliberately does not parse ISO/UDF images; use ffprobe as the fallback
// for ISO sources and unusual/branching DVD authoring.
func ReadDVDChapters(source string, globalTitle int) (DVDTitleLocation, []DVDChapter, error) {
	var loc DVDTitleLocation
	if globalTitle < 1 {
		return loc, nil, fmt.Errorf("DVD title must be >= 1")
	}

	videoTS, err := findVideoTSDir(source)
	if err != nil {
		return loc, nil, err
	}

	vmg, err := readIFO(filepath.Join(videoTS, "VIDEO_TS.IFO"), "DVDVIDEO-VMG")
	if err != nil {
		return loc, nil, err
	}

	loc, err = mapGlobalTitle(vmg, globalTitle)
	if err != nil {
		return loc, nil, err
	}

	vtsPath := filepath.Join(videoTS, fmt.Sprintf("VTS_%02d_0.IFO", loc.TitleSet))
	vts, err := readIFO(vtsPath, "DVDVIDEO-VTS")
	if err != nil {
		return loc, nil, err
	}

	allPTTs, err := parseVTSPTTTable(vts)
	if err != nil {
		return loc, nil, err
	}
	if loc.TitleSetTitle < 1 || loc.TitleSetTitle > len(allPTTs) {
		return loc, nil, fmt.Errorf("%w: VTS title %d is outside the PTT table", ErrNativeDVDChaptersUnsupported, loc.TitleSetTitle)
	}
	ptts := allPTTs[loc.TitleSetTitle-1]
	if len(ptts) == 0 {
		return loc, nil, fmt.Errorf("%w: selected title has no PTT entries", ErrNativeDVDChaptersUnsupported)
	}

	// Keep the lightweight path conservative. A movie title normally has all
	// chapter PTTs in one sequential PGC. If authoring branches across PGCs,
	// libdvdnav is a better source of truth than trying to emulate DVD VM code.
	pgcn := ptts[0].pgcn
	if pgcn == 0 {
		return loc, nil, fmt.Errorf("%w: chapter 1 has PGCN 0", ErrNativeDVDChaptersUnsupported)
	}
	prevPGN := uint16(0)
	for i, p := range ptts {
		if p.pgcn != pgcn {
			return loc, nil, fmt.Errorf("%w: chapters span multiple PGCs", ErrNativeDVDChaptersUnsupported)
		}
		if p.pgn == 0 || (i > 0 && p.pgn <= prevPGN) {
			return loc, nil, fmt.Errorf("%w: non-monotonic program mapping", ErrNativeDVDChaptersUnsupported)
		}
		prevPGN = p.pgn
	}

	pgc, err := parsePGC(vts, pgcn)
	if err != nil {
		return loc, nil, err
	}
	if pgc.playbackMode != 0 {
		return loc, nil, fmt.Errorf("%w: PGC uses random/shuffle playback mode", ErrNativeDVDChaptersUnsupported)
	}
	if pgc.stillTime != 0 {
		return loc, nil, fmt.Errorf("%w: PGC has still-time semantics", ErrNativeDVDChaptersUnsupported)
	}

	programStarts, total, err := pgcProgramTimeline(pgc)
	if err != nil {
		return loc, nil, err
	}

	firstPGN := int(ptts[0].pgn)
	if firstPGN < 1 || firstPGN > len(programStarts) {
		return loc, nil, fmt.Errorf("%w: first chapter references program %d of %d", ErrNativeDVDChaptersUnsupported, firstPGN, len(programStarts))
	}
	base := programStarts[firstPGN-1]

	titleEnd := total
	lastPGN := int(ptts[len(ptts)-1].pgn)
	for idx, other := range allPTTs {
		if idx == loc.TitleSetTitle-1 || len(other) == 0 || other[0].pgcn != pgcn {
			continue
		}
		candidatePGN := int(other[0].pgn)
		if candidatePGN > lastPGN && candidatePGN <= len(programStarts) {
			t := programStarts[candidatePGN-1]
			if t > base && t < titleEnd {
				titleEnd = t
			}
		}
	}
	if titleEnd <= base {
		return loc, nil, fmt.Errorf("%w: invalid title timeline", ErrNativeDVDChaptersUnsupported)
	}

	chapters := make([]DVDChapter, 0, len(ptts))
	for i, p := range ptts {
		pgn := int(p.pgn)
		if pgn < 1 || pgn > len(programStarts) {
			return loc, nil, fmt.Errorf("%w: chapter %d references program %d of %d", ErrNativeDVDChaptersUnsupported, i+1, pgn, len(programStarts))
		}
		start := programStarts[pgn-1]
		if start < base || start >= titleEnd {
			return loc, nil, fmt.Errorf("%w: chapter %d starts outside selected title", ErrNativeDVDChaptersUnsupported, i+1)
		}
		start -= base

		end := titleEnd - base
		if i+1 < len(ptts) {
			nextPGN := int(ptts[i+1].pgn)
			if nextPGN < 1 || nextPGN > len(programStarts) {
				return loc, nil, fmt.Errorf("%w: chapter %d has invalid next program", ErrNativeDVDChaptersUnsupported, i+1)
			}
			end = programStarts[nextPGN-1] - base
		}
		if end <= start {
			return loc, nil, fmt.Errorf("%w: chapter %d has non-positive duration", ErrNativeDVDChaptersUnsupported, i+1)
		}

		chapters = append(chapters, DVDChapter{Number: i + 1, Start: start, Duration: end - start, PGCN: p.pgcn, PGN: p.pgn})
	}

	if loc.DeclaredChapters > 0 && loc.DeclaredChapters != len(chapters) {
		return loc, nil, fmt.Errorf("%w: VMG declares %d chapters, VTS contains %d", ErrNativeDVDChaptersUnsupported, loc.DeclaredChapters, len(chapters))
	}

	return loc, chapters, nil
}

func findVideoTSDir(source string) (string, error) {
	source = filepath.Clean(strings.TrimSpace(strings.Trim(source, "\"")))
	if strings.EqualFold(filepath.Ext(source), ".iso") {
		return "", fmt.Errorf("%w: ISO/UDF source", ErrNativeDVDChaptersUnsupported)
	}
	st, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("DVD source: %w", err)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%w: source is not a VIDEO_TS directory", ErrNativeDVDChaptersUnsupported)
	}
	candidates := []string{source, filepath.Join(source, "VIDEO_TS")}
	for _, dir := range candidates {
		if st, err := os.Stat(filepath.Join(dir, "VIDEO_TS.IFO")); err == nil && !st.IsDir() {
			return dir, nil
		}
	}
	return "", fmt.Errorf("VIDEO_TS.IFO was not found under %s", source)
}

func readIFO(path, magic string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	if st.Size() < dvdSectorSize || st.Size() > maxIFOSize {
		return nil, fmt.Errorf("%s has implausible size %d", filepath.Base(path), st.Size())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if len(b) < len(magic) || string(b[:len(magic)]) != magic {
		return nil, fmt.Errorf("%s is not a valid DVD IFO (%s magic missing)", filepath.Base(path), magic)
	}
	return b, nil
}

func mapGlobalTitle(vmg []byte, globalTitle int) (DVDTitleLocation, error) {
	var loc DVDTitleLocation
	loc.GlobalTitle = globalTitle
	table, end, err := sectorTable(vmg, 0xC4)
	if err != nil {
		return loc, fmt.Errorf("TT_SRPT: %w", err)
	}
	n, err := be16(vmg, table)
	if err != nil {
		return loc, err
	}
	if globalTitle > int(n) {
		return loc, fmt.Errorf("DVD title %d does not exist (disc has %d titles)", globalTitle, n)
	}
	entry := table + 8 + (globalTitle-1)*12
	if entry+12 > end {
		return loc, fmt.Errorf("TT_SRPT title %d entry is truncated", globalTitle)
	}
	chapters, err := be16(vmg, entry+2)
	if err != nil {
		return loc, err
	}
	titleSet := int(vmg[entry+6])
	titleSetTitle := int(vmg[entry+7])
	if titleSet < 1 || titleSet > 99 || titleSetTitle < 1 {
		return loc, fmt.Errorf("TT_SRPT title %d has invalid VTS mapping", globalTitle)
	}
	loc.TitleSet = titleSet
	loc.TitleSetTitle = titleSetTitle
	loc.DeclaredChapters = int(chapters)
	return loc, nil
}

func parseVTSPTTTable(vts []byte) ([][]dvdPTT, error) {
	base, end, err := sectorTable(vts, 0xC8)
	if err != nil {
		return nil, fmt.Errorf("VTS_PTT_SRPT: %w", err)
	}
	n16, err := be16(vts, base)
	if err != nil {
		return nil, err
	}
	n := int(n16)
	if n == 0 || n > 999 {
		return nil, fmt.Errorf("VTS_PTT_SRPT has invalid title count %d", n)
	}
	offsetsEnd := base + 8 + n*4
	if offsetsEnd > end {
		return nil, fmt.Errorf("VTS_PTT_SRPT offset table is truncated")
	}
	offsets := make([]int, n)
	for i := 0; i < n; i++ {
		rel, err := be32(vts, base+8+i*4)
		if err != nil {
			return nil, err
		}
		offsets[i] = int(rel)
	}
	titles := make([][]dvdPTT, n)
	for i := 0; i < n; i++ {
		start := base + offsets[i]
		stop := end
		if i+1 < n {
			stop = base + offsets[i+1]
		}
		if start < offsetsEnd || stop < start || stop > end || (stop-start)%4 != 0 {
			return nil, fmt.Errorf("VTS_PTT_SRPT title %d has invalid offsets", i+1)
		}
		count := (stop - start) / 4
		ptts := make([]dvdPTT, 0, count)
		for j := 0; j < count; j++ {
			off := start + j*4
			pgcn, err := be16(vts, off)
			if err != nil {
				return nil, err
			}
			pgn, err := be16(vts, off+2)
			if err != nil {
				return nil, err
			}
			ptts = append(ptts, dvdPTT{pgcn: pgcn, pgn: pgn})
		}
		titles[i] = ptts
	}
	return titles, nil
}

func parsePGC(vts []byte, pgcn uint16) (dvdPGC, error) {
	var out dvdPGC
	base, end, err := sectorTable(vts, 0xCC)
	if err != nil {
		return out, fmt.Errorf("VTS_PGCIT: %w", err)
	}
	n, err := be16(vts, base)
	if err != nil {
		return out, err
	}
	if pgcn < 1 || pgcn > n {
		return out, fmt.Errorf("%w: PGC %d is outside PGCI table (1..%d)", ErrNativeDVDChaptersUnsupported, pgcn, n)
	}
	srp := base + 8 + (int(pgcn)-1)*8
	if srp+8 > end {
		return out, fmt.Errorf("PGCI SRP %d is truncated", pgcn)
	}
	rel, err := be32(vts, srp+4)
	if err != nil {
		return out, err
	}
	pgcBase := base + int(rel)
	if pgcBase < base || pgcBase+0xEC > end {
		return out, fmt.Errorf("PGC %d header is outside PGCI", pgcn)
	}
	out.programs = int(vts[pgcBase+0x02])
	out.cells = int(vts[pgcBase+0x03])
	out.stillTime = vts[pgcBase+0xA2]
	out.playbackMode = vts[pgcBase+0xA3]
	if out.programs < 1 || out.cells < 1 || out.programs > out.cells {
		return out, fmt.Errorf("PGC %d has invalid program/cell counts (%d/%d)", pgcn, out.programs, out.cells)
	}
	programMapRel, err := be16(vts, pgcBase+0xE6)
	if err != nil {
		return out, err
	}
	cellRel, err := be16(vts, pgcBase+0xE8)
	if err != nil {
		return out, err
	}
	if programMapRel == 0 || cellRel == 0 {
		return out, fmt.Errorf("%w: PGC %d has no program map or cell playback table", ErrNativeDVDChaptersUnsupported, pgcn)
	}
	pmStart := pgcBase + int(programMapRel)
	pmEnd := pmStart + out.programs
	cellStart := pgcBase + int(cellRel)
	cellEnd := cellStart + out.cells*24
	if pmStart < pgcBase || pmEnd > end || cellStart < pgcBase || cellEnd > end {
		return out, fmt.Errorf("PGC %d tables are truncated", pgcn)
	}
	out.programMap = append([]byte(nil), vts[pmStart:pmEnd]...)
	out.cellData = append([]byte(nil), vts[cellStart:cellEnd]...)
	prev := 0
	for i, c := range out.programMap {
		cell := int(c)
		if cell < 1 || cell > out.cells || cell <= prev {
			return out, fmt.Errorf("PGC %d program %d has invalid entry cell %d", pgcn, i+1, cell)
		}
		prev = cell
	}
	return out, nil
}

func validatePGCCellStructure(pgc dvdPGC) error {
	if len(pgc.cellData) != pgc.cells*24 {
		return fmt.Errorf("%w: PGC cell playback table has invalid size", ErrNativeDVDChaptersUnsupported)
	}
	programStart := make(map[int]bool, len(pgc.programMap))
	for _, c := range pgc.programMap {
		programStart[int(c)] = true
	}
	angleBlock := false
	for cell := 1; cell <= pgc.cells; cell++ {
		flags := pgc.cellData[(cell-1)*24]
		mode := flags >> 6
		blockType := (flags >> 4) & 0x03
		if programStart[cell] && (mode == 2 || mode == 3) {
			return fmt.Errorf("%w: program starts inside angle block at cell %d", ErrNativeDVDChaptersUnsupported, cell)
		}
		switch blockType {
		case 0:
			if mode != 0 || angleBlock {
				return fmt.Errorf("%w: malformed DVD angle block at cell %d", ErrNativeDVDChaptersUnsupported, cell)
			}
		case 1:
			switch mode {
			case 1:
				if angleBlock {
					return fmt.Errorf("%w: nested DVD angle block at cell %d", ErrNativeDVDChaptersUnsupported, cell)
				}
				angleBlock = true
			case 2:
				if !angleBlock {
					return fmt.Errorf("%w: orphan middle angle cell %d", ErrNativeDVDChaptersUnsupported, cell)
				}
			case 3:
				if !angleBlock {
					return fmt.Errorf("%w: orphan last angle cell %d", ErrNativeDVDChaptersUnsupported, cell)
				}
				angleBlock = false
			default:
				return fmt.Errorf("%w: invalid DVD angle block mode at cell %d", ErrNativeDVDChaptersUnsupported, cell)
			}
		default:
			return fmt.Errorf("%w: unsupported DVD cell block type %d at cell %d", ErrNativeDVDChaptersUnsupported, blockType, cell)
		}
	}
	if angleBlock {
		return fmt.Errorf("%w: unterminated DVD angle block", ErrNativeDVDChaptersUnsupported)
	}
	return nil
}

func pgcProgramTimeline(pgc dvdPGC) ([]time.Duration, time.Duration, error) {
	if err := validatePGCCellStructure(pgc); err != nil {
		return nil, 0, err
	}
	starts := make([]time.Duration, pgc.programs)
	var total time.Duration
	for p := 0; p < pgc.programs; p++ {
		starts[p] = total
		firstCell := int(pgc.programMap[p])
		lastCell := pgc.cells
		if p+1 < pgc.programs {
			lastCell = int(pgc.programMap[p+1]) - 1
		}
		if firstCell < 1 || lastCell < firstCell || lastCell > pgc.cells {
			return nil, 0, fmt.Errorf("invalid cell span for program %d", p+1)
		}
		for cell := firstCell; cell <= lastCell; cell++ {
			entry := pgc.cellData[(cell-1)*24 : cell*24]
			mode := entry[0] >> 6
			if mode == 2 || mode == 3 {
				continue
			}
			if entry[2] != 0 {
				return nil, 0, fmt.Errorf("%w: cell %d uses still-time semantics", ErrNativeDVDChaptersUnsupported, cell)
			}
			d, err := decodeDVDTime(entry[4:8])
			if err != nil {
				return nil, 0, fmt.Errorf("cell %d playback time: %w", cell, err)
			}
			total += d
		}
	}
	if total <= 0 {
		return nil, 0, fmt.Errorf("%w: PGC duration is zero", ErrNativeDVDChaptersUnsupported)
	}
	return starts, total, nil
}

func decodeDVDTime(b []byte) (time.Duration, error) {
	if len(b) < 4 {
		return 0, errors.New("DVD time field is truncated")
	}
	hh, ok := decodeBCD(b[0])
	if !ok {
		return 0, fmt.Errorf("invalid BCD hours 0x%02x", b[0])
	}
	mm, ok := decodeBCD(b[1])
	if !ok || mm >= 60 {
		return 0, fmt.Errorf("invalid BCD minutes 0x%02x", b[1])
	}
	ss, ok := decodeBCD(b[2])
	if !ok || ss >= 60 {
		return 0, fmt.Errorf("invalid BCD seconds 0x%02x", b[2])
	}
	frameByte := b[3]
	rateCode := frameByte >> 6
	frameTens := (frameByte >> 4) & 0x03
	frameUnits := frameByte & 0x0F
	if frameUnits > 9 {
		return 0, fmt.Errorf("invalid BCD frame byte 0x%02x", frameByte)
	}
	frames := int(frameTens)*10 + int(frameUnits)
	base := time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute + time.Duration(ss)*time.Second
	switch rateCode {
	case 1:
		if frames >= 25 {
			return 0, fmt.Errorf("PAL frame %d is out of range", frames)
		}
		return base + time.Duration(int64(time.Second)*int64(frames)/25), nil
	case 3:
		if frames >= 30 {
			return 0, fmt.Errorf("NTSC frame %d is out of range", frames)
		}
		return base + time.Duration(int64(time.Second)*int64(frames)/30), nil
	case 0, 2:
		if frames == 0 {
			return base, nil
		}
		return 0, fmt.Errorf("unsupported DVD frame-rate code %d with nonzero frame fraction", rateCode)
	default:
		panic("unreachable")
	}
}

func decodeBCD(b byte) (int, bool) {
	hi, lo := b>>4, b&0x0F
	if hi > 9 || lo > 9 {
		return 0, false
	}
	return int(hi)*10 + int(lo), true
}

func sectorTable(b []byte, pointerOff int) (int, int, error) {
	sector, err := be32(b, pointerOff)
	if err != nil {
		return 0, 0, err
	}
	if sector == 0 {
		return 0, 0, fmt.Errorf("sector pointer at 0x%X is zero", pointerOff)
	}
	base64 := uint64(sector) * dvdSectorSize
	if base64 > uint64(len(b)) || base64+8 > uint64(len(b)) {
		return 0, 0, fmt.Errorf("sector pointer at 0x%X points outside IFO", pointerOff)
	}
	base := int(base64)
	endAddr, err := be32(b, base+4)
	if err != nil {
		return 0, 0, err
	}
	end64 := base64 + uint64(endAddr) + 1
	if end64 > uint64(len(b)) || end64 < base64+8 {
		return 0, 0, fmt.Errorf("table at sector %d has invalid end address", sector)
	}
	return base, int(end64), nil
}

func be16(b []byte, off int) (uint16, error) {
	if off < 0 || off+2 > len(b) {
		return 0, fmt.Errorf("u16 read at 0x%X is outside IFO", off)
	}
	return binary.BigEndian.Uint16(b[off : off+2]), nil
}
func be32(b []byte, off int) (uint32, error) {
	if off < 0 || off+4 > len(b) {
		return 0, fmt.Errorf("u32 read at 0x%X is outside IFO", off)
	}
	return binary.BigEndian.Uint32(b[off : off+4]), nil
}
