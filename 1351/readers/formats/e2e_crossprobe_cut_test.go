package formats

import (
	"path/filepath"
	"testing"
)

// The cross-probe walk (agni issue 1323) runs on a cut of Jetson's board, and it needs three
// things from it. Every part on the schematic has its footprint, every pad sits on the net the
// schematic gives that pin, and both sides of the board and a turned part are there to show.
func TestTheCrossProbeCutJoinsItsSchematicToItsBoard(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "cross-probe-project", "designs", "jetson-cut")
	l := &Loader{}
	d, err := l.ReadDesign(filepath.Join(dir, "jetson-cut.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.BoardGeometry(filepath.Join(dir, "jetson-cut.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	netOf := map[[2]string]string{}
	for _, n := range d.Nets {
		for _, c := range n.Connections {
			netOf[[2]string{c.ComponentRef, c.PinRef}] = n.Name
		}
	}
	placed := map[string]bool{}
	sides := map[string]int{}
	turned, pads := 0, 0
	for _, p := range b.Placements {
		placed[p.RefDes] = true
		sides[p.Layer]++
		if int(p.RotationDeg)%180 != 0 {
			turned++
		}
		for _, pad := range p.Pads {
			want, ok := netOf[[2]string{p.RefDes, pad.Number}]
			if !ok || pad.Net == "" {
				continue
			}
			pads++
			if pad.Net != want {
				t.Errorf("%s pad %s is on %q on the board and %q on the schematic", p.RefDes, pad.Number, pad.Net, want)
			}
		}
	}
	for _, c := range d.Components {
		if !placed[c.RefDes] {
			t.Errorf("%s is on the schematic and has no footprint", c.RefDes)
		}
	}
	if len(d.Components) != 19 || pads < 50 {
		t.Fatalf("read %d parts and %d joined pads, want 19 parts and at least 50 pads", len(d.Components), pads)
	}
	if sides["F.Cu"] == 0 || sides["B.Cu"] == 0 || turned == 0 {
		t.Errorf("the cut has %v by side and %d turned parts, want both sides and a turned part", sides, turned)
	}
}
