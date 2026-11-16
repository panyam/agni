package kicad

import (
	"strings"
	"testing"
)

// TestAPasteWindowIsNotAPad reads a footprint with a copper pad, a paste-only window, a mask-only
// opening and a pad that lists no layers (agni issue 133). A placement holds its copper pads, so the
// two windows drop and the other two stay.
func TestAPasteWindowIsNotAPad(t *testing.T) {
	const board = `(kicad_pcb (version 20240108) (generator "pcbnew")
	(footprint "x:Y" (layer "F.Cu") (at 10 10)
		(property "Reference" "U1" (at 0 0) (layer "F.SilkS"))
		(pad "1" smd rect (at 0 0) (size 1 1) (layers "F.Cu" "F.Paste" "F.Mask"))
		(pad "" smd rect (at 0 2) (size 1 1) (layers "F.Paste"))
		(pad "" smd rect (at 0 4) (size 1 1) (layers "F.Mask"))
		(pad "2" smd rect (at 2 0) (size 1 1))))`
	g, err := ReadBoardGeometry(strings.NewReader(board), "paste.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.GetPlacements()) != 1 {
		t.Fatalf("%d placements, want 1", len(g.GetPlacements()))
	}
	var got []string
	for _, p := range g.GetPlacements()[0].GetPads() {
		got = append(got, p.GetNumber())
	}
	if strings.Join(got, ",") != "1,2" {
		t.Errorf("pads %q, want 1 and 2", got)
	}
}
