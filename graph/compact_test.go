package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestSizeAwareNoOverlap covers WS7-032: mixing one large (faithful) symbol with small glyph nodes
// must not overlap any two nodes, and — unlike the old uniform scale, which spread EVERY node by
// the largest symbol's extent — small nodes in a big-symbol-free row stay one pitch apart. The
// second assertion is the regression's green: on the pre-WS7-032 uniform scale R1..R2 were ~62x
// pitch apart.
func TestSizeAwareNoOverlap(t *testing.T) {
	// Six components on a 3-wide grid: R1..R5 draw the resistor glyph (no faithful symbol -> the
	// registry fallback), U1 draws the 5000-unit faithful symbol.
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "R1"}, {RefDes: "R2"}, {RefDes: "R3"}, {RefDes: "R4"}, {RefDes: "R5"}, {RefDes: "U1"},
	}}
	g, err := LayoutWith(d, "grid", WithSymbolSource(NewFaithfulSource(faithfulGeom("U1"), DefaultRegistry())))
	if err != nil {
		t.Fatal(err)
	}

	symByCell := map[string]*geom.SymbolDef{}
	for _, s := range g.Symbols {
		symByCell[s.CellRef] = s
	}
	type box struct{ x0, y0, x1, y1 int64 }
	boxes := map[string]box{}
	origin := map[string]*geom.Point{}
	for _, pl := range g.Sheets[0].Placements {
		s := symbolSize(symByCell[pl.CellRef])
		o := pl.Transform.Origin
		origin[pl.RefDes] = o
		boxes[pl.RefDes] = box{o.X - s.w/2, o.Y - s.h/2, o.X + s.w/2, o.Y + s.h/2}
	}

	// No two node bounding boxes overlap.
	refs := []string{"R1", "R2", "R3", "R4", "R5", "U1"}
	for i := 0; i < len(refs); i++ {
		for j := i + 1; j < len(refs); j++ {
			a, b := boxes[refs[i]], boxes[refs[j]]
			if a.x0 < b.x1 && b.x0 < a.x1 && a.y0 < b.y1 && b.y0 < a.y1 {
				t.Errorf("%s and %s overlap: %+v vs %+v", refs[i], refs[j], a, b)
			}
		}
	}

	// R1 and R2 share the big-symbol-free top row and adjacent small columns, so they stay one
	// pitch apart — the layout is not uniformly inflated by U1.
	if dx := origin["R2"].X - origin["R1"].X; dx != pitch {
		t.Errorf("small nodes spaced %d apart, want the tight pitch %d (not scaled by the big symbol)", dx, pitch)
	}
	if origin["R1"].Y != origin["R2"].Y {
		t.Errorf("R1 and R2 should share a row, got Y %d vs %d", origin["R1"].Y, origin["R2"].Y)
	}
}
