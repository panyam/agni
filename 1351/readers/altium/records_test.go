package altium

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

func propList(s string) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(s)+1))
	return append(append(b, s...), 0)
}

func TestAPropertyListKeepsTheFirstOfARepeatedKey(t *testing.T) {
	lists, err := parseProps(append(propList("|LAYER=UNKNOWN|Layer=TOP|ORIGINX=4149.6063mil|ROTATION= 3.60000000000000E+0002"), propList("|NAME=GND")...))
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 2 {
		t.Fatalf("read %d lists, want 2", len(lists))
	}
	p := lists[0]
	if p["LAYER"] != "UNKNOWN" {
		t.Errorf("LAYER %q, want the first value, UNKNOWN", p["LAYER"])
	}
	if got := p.length("ORIGINX"); got != 41496063 {
		t.Errorf("4149.6063mil reads %d internal units, want 41,496,063", got)
	}
	if got := p.float("ROTATION"); got != 360 {
		t.Errorf("rotation reads %v, want 360", got)
	}
	if lists[1]["NAME"] != "GND" {
		t.Errorf("second list %v", lists[1])
	}
	if _, err := parseProps([]byte{10, 0, 0, 0, '|'}); err == nil {
		t.Error("a list longer than its stream read without an error")
	}
}

func wide(idx uint32, s string) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b, idx)
	if s == "" {
		binary.LittleEndian.PutUint32(b[4:], 2) // an empty string: the length, and no bytes after it
		return b
	}
	u := append(utf16.Encode([]rune(s)), 0)
	binary.LittleEndian.PutUint32(b[4:], uint32(2*len(u)))
	for _, r := range u {
		b = binary.LittleEndian.AppendUint16(b, r)
	}
	return b
}

func TestAnEmptyWideStringHasNoBytesAfterIt(t *testing.T) {
	var b []byte
	b = append(b, wide(0, "R101")...)
	b = append(b, wide(1, "")...)
	b = append(b, wide(2, "µC")...)
	ws, err := parseWideStrings(b)
	if err != nil {
		t.Fatal(err)
	}
	if ws[0] != "R101" || ws[1] != "" || ws[2] != "µC" {
		t.Errorf("read %v, want R101, an empty string, then µC", ws)
	}
}

func TestABinaryRecordOfTheWrongTypeIsAnError(t *testing.T) {
	rec := []byte{0x04, 3, 0, 0, 0, 1, 2, 3}
	if rs, err := parseRecords(rec, 0x04, 1); err != nil || len(rs) != 1 || len(rs[0].subs[0]) != 3 {
		t.Fatalf("a well-formed record read as %v, %v", rs, err)
	}
	if _, err := parseRecords(rec, 0x01, 1); err == nil || !strings.Contains(err.Error(), "type 0x04") {
		t.Errorf("a track in the arcs stream: err %v", err)
	}
	if _, err := parseRecords(rec[:6], 0x04, 1); err == nil {
		t.Error("a cut-short subrecord read without an error")
	}
	if got := field([]byte{1, 2}).i32(0); got != 0 {
		t.Errorf("a field past the end of a short subrecord reads %d, want its default 0", got)
	}
}

func TestTheCopperStackFollowsTheNextChain(t *testing.T) {
	// MID3 (id 4) sits above MID2 (id 3) on this stack, as on one foxBMS board.
	b := propsOf([]byte("|LAYER1NEXT=2|LAYER2NEXT=4|LAYER4NEXT=3|LAYER3NEXT=5|LAYER5NEXT=32|LAYER32NEXT=0|"))
	got := copperStack(b)
	want := []int{1, 2, 4, 3, 5, 32}
	if len(got) != len(want) {
		t.Fatalf("stack %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stack %v, want %v", got, want)
		}
	}
	if s := copperStack(props{}); len(s) != 2 || s[0] != 1 || s[1] != 32 {
		t.Errorf("a board with no chain stacks %v, want top and bottom", s)
	}
}
