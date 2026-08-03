package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// bigSymbol is a faithful symbol far larger than the synthetic glyphs, to exercise scaling.
func bigSymbol() *geom.SymbolDef {
	return &geom.SymbolDef{
		CellRef: "lib:BIG",
		Bbox:    &geom.BBox{Min: &geom.Point{X: -2500, Y: -2500}, Max: &geom.Point{X: 2500, Y: 2500}},
	}
}

func faithfulGeom(refs ...string) *geom.SchematicGeometry {
	sh := &geom.SheetGeometry{}
	for _, r := range refs {
		sh.Placements = append(sh.Placements, &geom.SymbolPlacement{RefDes: r, CellRef: "lib:BIG"})
	}
	return &geom.SchematicGeometry{Symbols: []*geom.SymbolDef{bigSymbol()}, Sheets: []*geom.SheetGeometry{sh}}
}

// TestFaithfulSource asserts a component with a faithful symbol draws it (joined by ref_des),
// one without falls back to the base source (the classified glyph), and Covers counts the join.
func TestFaithfulSource(t *testing.T) {
	src := NewFaithfulSource(faithfulGeom("U1"), DefaultRegistry())

	if s := src.Symbol("U1", &ir.Component{RefDes: "U1"}, nil); s.CellRef != "lib:BIG" {
		t.Errorf("U1 faithful symbol = %q, want lib:BIG", s.CellRef)
	}
	// R1 has no faithful symbol -> falls back to the registry's resistor glyph.
	want := DefaultRegistry().cellFor(ClassResistor)
	if s := src.Symbol("R1", &ir.Component{RefDes: "R1"}, nil); s.CellRef != want {
		t.Errorf("R1 fallback = %q, want the resistor glyph %q", s.CellRef, want)
	}
	if src.Covers() != 1 {
		t.Errorf("Covers = %d, want 1", src.Covers())
	}
}

// TestFaithfulLayoutScalesToAvoidOverlap asserts that re-laying-out large faithful symbols
// spreads the positions to at least the symbol extent, so they do not overlap, and that the
// placements draw the faithful symbol.
func TestFaithfulLayoutScalesToAvoidOverlap(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}}}
	g, err := LayoutWith(d, "grid", WithSymbolSource(NewFaithfulSource(faithfulGeom("U1", "U2"), DefaultRegistry())))
	if err != nil {
		t.Fatal(err)
	}
	origin := map[string]*geom.Point{}
	for _, pl := range g.Sheets[0].Placements {
		if pl.CellRef != "lib:BIG" {
			t.Errorf("%s cell = %q, want the faithful lib:BIG", pl.RefDes, pl.CellRef)
		}
		origin[pl.RefDes] = pl.Transform.Origin
	}
	// The two nodes sit one grid step apart; scaled, that step must clear the 5000-unit symbol.
	if dx := abs64(origin["U2"].X - origin["U1"].X); dx < 5000 {
		t.Errorf("faithful symbols overlap: spacing %d < symbol extent 5000", dx)
	}
}

// TestGlyphSpacingUnchanged is the regression guard: synthetic glyphs are <= the reference node,
// so their layout is unscaled (scale 1) and positions stay on the base pitch grid, byte-identical
// to before the symbol-source change.
func TestGlyphSpacingUnchanged(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}}}
	g := layout(d)
	origin := map[string]int64{}
	for _, pl := range g.Sheets[0].Placements {
		origin[pl.RefDes] = pl.Transform.Origin.X
	}
	if got := origin["R2"] - origin["R1"]; got != pitch {
		t.Errorf("glyph spacing = %d, want the unscaled pitch %d", got, pitch)
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
