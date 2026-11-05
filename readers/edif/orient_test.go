package edif

import (
	"bytes"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// TestEveryOrientationCodePlacesPinsByTheSpec places one three-pin part at each of EDIF's eight
// orientation codes and holds each pin's drawn connect point to the code's matrix. The matrices
// are the EDIF 2 0 0 definitions, as docsite/content/reference/edif-schematic-primer.md section 7
// lists them: mirror first, then rotate counter-clockwise. The part's pins sit at (0,0), (600,0)
// and (0,400), so no two codes put them in the same places (agni issue 1344).
func TestEveryOrientationCodePlacesPinsByTheSpec(t *testing.T) {
	want := map[string]map[string][2]int64{
		"U1": {"1": {0, 0}, "2": {600, 0}, "3": {0, 400}},   // R0
		"U2": {"1": {0, 0}, "2": {0, 600}, "3": {-400, 0}},  // R90
		"U3": {"1": {0, 0}, "2": {-600, 0}, "3": {0, -400}}, // R180
		"U4": {"1": {0, 0}, "2": {0, -600}, "3": {400, 0}},  // R270
		"U5": {"1": {0, 0}, "2": {600, 0}, "3": {0, -400}},  // MX, flip Y
		"U6": {"1": {0, 0}, "2": {-600, 0}, "3": {0, 400}},  // MY, flip X
		"U7": {"1": {0, 0}, "2": {0, 600}, "3": {400, 0}},   // MXR90 = R90 * MX
		"U8": {"1": {0, 0}, "2": {0, -600}, "3": {-400, 0}}, // MYR90 = R90 * MY
	}
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "orient-codes.eds")), "orient-codes.eds")
	if err != nil {
		t.Fatal(err)
	}
	pins := map[string]map[string]*[2]int64{}
	for _, s := range g.GetSymbols() {
		m := map[string]*[2]int64{}
		for _, p := range s.GetPins() {
			m[p.GetPortRef()] = &[2]int64{p.GetLoc().GetX(), p.GetLoc().GetY()}
		}
		pins[s.GetCellRef()] = m
	}
	checked := 0
	for _, sheet := range g.GetSheets() {
		for _, pl := range sheet.GetPlacements() {
			w, ok := want[pl.GetRefDes()]
			if !ok {
				t.Errorf("unexpected placement %q", pl.GetRefDes())
				continue
			}
			o := pl.GetTransform().GetOrigin()
			for num, loc := range pins[pl.GetCellRef()] {
				at := geomath.ApplyTransform(pl.GetTransform(), &geom.Point{X: loc[0], Y: loc[1]})
				got := [2]int64{at.GetX() - o.GetX(), at.GetY() - o.GetY()}
				if got != w[num] {
					t.Errorf("%s pin %s lands at %v from the origin, want %v", pl.GetRefDes(), num, got, w[num])
				}
				checked++
			}
		}
	}
	if checked != 24 {
		t.Errorf("checked %d pins, want 24", checked)
	}
}
