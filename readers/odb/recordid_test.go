package odb

import (
	"strings"
	"testing"
	"testing/fstest"
)

// ODB++ 8 lets a record end with `;ID=<n>` after its attributes. One layout tool writes it on every
// component, and the last attribute's value must not keep it (agni 1685).
func TestARecordIDLeavesTheLastAttributeWhole(t *testing.T) {
	comps, _, err := parseComponents([]byte("UNITS=INCH\n@0 .comp_height\nCMP 0 1 1 0 N L1 P ;0=0.015748;ID=664\nCMP 0 2 2 0 N L2 P ;0=0.1;ID=665\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got := heights(comps); len(got) != 2 || got[0] != 399_999 || got[1] != 2_540_000 {
		t.Errorf("heights = %v, want 0.4 mm and 2.54 mm", got)
	}
}

func TestADrillHitWithARecordIDReadsAsItsTwin(t *testing.T) {
	ff, err := parseFeatures([]byte("UNITS=INCH\n$0 r10\n@0 .geometry\n@1 .drill\n&0 VIA\n#\nP 1 1 0 P 0 0 ;0=0,1=2\nP 2 2 0 P 0 0 ;0=0,1=2;ID=7\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ff.features) != 2 {
		t.Fatalf("read %d features, want 2", len(ff.features))
	}
	if !ff.features[0].via || ff.features[1].via != ff.features[0].via {
		t.Errorf("via flags %v and %v, want both marked (.drill=2)", ff.features[0].via, ff.features[1].via)
	}
}

// One part's unreadable height leaves the rest of the board readable, and the read names the line.
func TestAMalformedHeightIsNotedNotFatal(t *testing.T) {
	job := inchJobCopy(t)
	const comps = "steps/pcb/layers/comp_+_top/components"
	old := string(job[comps].Data)
	bad := strings.Replace(old, "UNITS=INCH\n", "UNITS=INCH\n@0 .comp_height\n", 1)
	bad = strings.Replace(bad, "CMP 0 0.5 0.5 90 N U1 LDO-PART ;0=0", "CMP 0 0.5 0.5 90 N U1 LDO-PART ;0=tall", 1)
	if bad == old || !strings.Contains(bad, "0=tall") {
		t.Fatal("inch-job's U1 line moved; this test gives it a bad height")
	}
	job[comps] = &fstest.MapFile{Data: []byte(bad)}
	_, g := readJob(t, job)
	if u1 := placement(t, g, "U1"); u1.Height != nil {
		t.Errorf("U1 height = %d, want none, as the file's value does not parse", u1.GetHeight())
	}
	notes := g.GetReadNotes()
	if len(notes) != 1 {
		t.Fatalf("read notes = %v, want one for U1's height", notes)
	}
	n := notes[0]
	if n.GetFile() != comps || n.GetLine() == 0 || n.GetField() != ".comp_height" || n.GetValue() != "tall" || n.GetReason() == "" {
		t.Errorf("note = %v, want %s, a line, .comp_height, value tall and a reason", n, comps)
	}
}
