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
	for magic, want := range map[uint32]string{0x00131500: "16.6", 0x00141500: "17.5", 0x00150000: "18.0"} {
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

func TestAn172HeaderWithNoBlocksReadsEmpty(t *testing.T) {
	d, err := Read(bytes.NewReader(header(magicV172)), "empty.brd")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 0 || len(d.GetNets()) != 0 || d.GetName() != "empty" {
		t.Errorf("empty board read as %d components, %d nets, named %q", len(d.GetComponents()), len(d.GetNets()), d.GetName())
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
