package kicad

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// Q5's body is a background-filled circle, and its channel, arrow and diode are drawn inside it.
// KiCad paints body fills under the lines, so every background fill must come first in the
// symbol, or the circle's fill covers what is inside it (agni 1344).
func TestABodyFillDrawsUnderTheLinesInsideIt(t *testing.T) {
	g := readSheetGeometry(t, "orient-jetson-fet.kicad_sch")
	sym := geomath.IndexSymbols(g).SymbolFor(placement(g, "Q5"))
	if sym == nil {
		t.Fatal("Q5 has no symbol")
	}
	fills, after := 0, 0
	for _, s := range sym.Shapes {
		if s.Fill == geom.Shape_FILL_BACKGROUND {
			if after > 0 {
				t.Fatalf("a background fill draws after %d other shapes, over them", after)
			}
			fills++
			continue
		}
		after++
	}
	if fills == 0 || after < 13 {
		t.Fatalf("Q5 has %d background fills and %d shapes after them, want at least 1 and 13", fills, after)
	}
}

// The fet cut's U61 and Q5 are marked (dnp yes), and TP76 is fitted.
func TestDoNotPopulateReachesThePlacement(t *testing.T) {
	g := readSheetGeometry(t, "orient-jetson-fet.kicad_sch")
	for ref, want := range map[string]bool{"U61": true, "Q5": true, "TP76": false} {
		pl := placement(g, ref)
		if pl == nil {
			t.Fatalf("placement %s not found", ref)
		}
		if pl.Dnp != want {
			t.Errorf("%s dnp = %v, want %v", ref, pl.Dnp, want)
		}
	}
}
