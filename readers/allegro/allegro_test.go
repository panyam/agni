package allegro

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// header builds the first 0x1200 bytes of a board with the given magic word and an empty string
// table, enough for parse to reach the version check and the block walk.
func header(magic uint32) []byte {
	b := make([]byte, stringTable)
	binary.LittleEndian.PutUint32(b, magic)
	copy(b[0xF8:], "all")
	b[hdrUnits] = 1
	binary.LittleEndian.PutUint32(b[hdrDivisor:], 1000)
	return b
}

func TestAnotherReleaseIsRefusedByName(t *testing.T) {
	for magic, want := range map[uint32]string{0x00131000: "16.5", 0x00140900: "17.4", 0x00141500: "17.5", 0x00150000: "18.0"} {
		_, err := Read(bytes.NewReader(header(magic)), "old.brd")
		if err == nil || !strings.Contains(err.Error(), "Allegro "+want) {
			t.Errorf("magic 0x%08X: err %v, want it to name Allegro %s", magic, err, want)
		}
	}
	_, err := Read(bytes.NewReader(header(0x00990000)), "odd.brd")
	if err == nil || !strings.Contains(err.Error(), "unknown format version 0x00990000") {
		t.Errorf("unknown magic: err %v", err)
	}
}

func TestAHeaderWithNoBlocksReadsEmpty(t *testing.T) {
	for magic, release := range map[uint32]string{magicV172: "17.2", magicV166: "16.6"} {
		d, err := Read(bytes.NewReader(header(magic)), "empty.brd")
		if err != nil {
			t.Fatalf("%s: %v", release, err)
		}
		if len(d.GetComponents()) != 0 || len(d.GetNets()) != 0 || d.GetName() != "empty" {
			t.Errorf("%s: empty board read as %d components, %d nets, named %q", release, len(d.GetComponents()), len(d.GetNets()), d.GetName())
		}
		if got := d.GetAttributes()["allegro_format"]; got != release {
			t.Errorf("allegro_format %q, want %s", got, release)
		}
	}
}

// A 16.6 block is sized from 16.6's table. A 0x06 is 36 bytes there and 40 in 17.2, so a 17.2 walk
// of two 16.6 component definitions lands mid-block and refuses the tag it finds there.
func TestA166BlockIsSizedByItsRelease(t *testing.T) {
	compDef := func(key byte, size int) []byte {
		b := make([]byte, size)
		b[0], b[4] = tagCompDef, key
		return b
	}
	body := append(compDef(1, 36), compDef(2, 36)...)
	if _, err := parse(append(header(magicV166), body...)); err != nil {
		t.Errorf("16.6: %v", err)
	}
	if _, err := parse(append(header(magicV172), body...)); err == nil {
		t.Error("a 17.2 walk read two 36-byte component definitions")
	}
}

// Every field a layout names sits inside the block it is read from, so a wrong offset in a table
// cannot read the next block's bytes.
func TestEveryLayoutFieldSitsInsideItsBlock(t *testing.T) {
	for _, l := range layouts {
		inside := map[string][2]int{
			"compRef": {l.compRef, l.fixedSize[tagCompInst]}, "compDef": {l.compDef, l.fixedSize[tagCompInst]},
			"fiRot": {l.fiRot, l.fixedSize[tagFootInst]}, "fiX": {l.fiX + 4, l.fixedSize[tagFootInst]},
			"fiInstRef": {l.fiInstRef, l.fixedSize[tagFootInst]}, "fiGraphics": {l.fiGraphics, l.fixedSize[tagFootInst]},
			"fiPads": {l.fiPads, l.fixedSize[tagFootInst]}, "fiText": {l.fiText, l.fixedSize[tagFootInst]},
			"ppNextInFp": {l.ppNextInFp, l.fixedSize[tagPlacedPad]}, "ppParentFp": {l.ppParentFp, l.fixedSize[tagPlacedPad]},
			"ppPad": {l.ppPad, l.fixedSize[tagPlacedPad]}, "padRot": {l.padRot, l.fixedSize[tagPad]},
			"ppPinNumber": {l.ppPinNumber, l.fixedSize[tagPlacedPad]}, "pinNumName": {l.pinNumName, l.fixedSize[tagPinNumber]},
			"pinNameStr":    {8, l.fixedSize[tagPinName]},
			"netMatchGroup": {l.netAssign + 12, l.fixedSize[tagNet]}, "mgGroup": {l.mgGroup, l.fixedSize[tagMatchGroup]},
			"tableFields": {l.tableFields, l.fixedSize[tagTable]},
			"netAssign":   {l.netAssign, l.fixedSize[tagNet]}, "trackSegs": {l.trackSegs, l.fixedSize[tagTrack]},
			"segEnd": {l.segWidth + 16, l.fixedSize[tagLineH]}, "arcRadius": {l.arcCentre() + 20, l.fixedSize[tagArc]},
			"graphSegs": {l.graphSegs, l.fixedSize[tagGraphic]}, "shapeSegs": {l.shapeSegs, l.fixedSize[tagShape]},
			"shapeTable": {l.shapeTable, l.fixedSize[tagShape]}, "tablePtr1": {l.tablePtr1, l.fixedSize[tagTable]},
			"textStr": {l.textStr, l.fixedSize[tagText]}, "textGroup": {l.textGroup, l.fixedSize[tagText]},
			"textRot": {l.textRot, l.fixedSize[tagText]}, "rectCoords": {l.rectCoords + 12, l.fixedSize[tagRect]},
			"viaPadstack": {l.viaPadstack, l.fixedSize[tagVia]},
		}
		for name, f := range inside {
			if name == "compDef" && f[0] == 0 {
				continue // 16.6 names no definition on an instance
			}
			if f[0] < 4 || f[0]+4 > f[1] {
				t.Errorf("%s: %s at %d does not fit a %d-byte block", l.release, name, f[0], f[1])
			}
		}
	}
}

func TestAnEagleBoardIsNamed(t *testing.T) {
	eagle := []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE eagle SYSTEM \"eagle.dtd\">\n<eagle version=\"9.6.2\"><drawing/></eagle>\n")
	_, err := Read(bytes.NewReader(eagle), "board.brd")
	if err == nil || !strings.Contains(err.Error(), "an Eagle board") {
		t.Errorf("err %v, want it to name an Eagle board", err)
	}
	if IsAllegro(eagle) {
		t.Error("an Eagle board sniffs as Allegro")
	}
}

func TestAFileThatIsNotAllegroIsRefused(t *testing.T) {
	b := header(magicV172)
	copy(b[0xF8:], "xyz")
	if _, err := Read(bytes.NewReader(b), "x.brd"); err == nil || !strings.Contains(err.Error(), "not an Allegro board") {
		t.Errorf("err %v", err)
	}
	if _, err := Read(bytes.NewReader([]byte("short")), "x.brd"); err == nil {
		t.Error("a five-byte file read without error")
	}
}

// A block whose fixed size runs past the end of the file stops the walk with an error, not a panic.
func TestABlockCutShortIsAnError(t *testing.T) {
	b := append(header(magicV172), tagNet, 0, 0, 0, 1, 0, 0, 0)
	_, err := Read(bytes.NewReader(b), "cut.brd")
	if err == nil || !strings.Contains(err.Error(), "runs past the end") {
		t.Errorf("err %v", err)
	}
	b = append(header(magicV172), 0x02, 0, 0, 0)
	if _, err := Read(bytes.NewReader(b), "cut.brd"); err == nil || !strings.Contains(err.Error(), "unknown block type 0x02") {
		t.Errorf("unknown tag: err %v", err)
	}
}
