package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestNetRoutesFromPins asserts a net edge starts at the connected pin's world position (node
// origin + the resistor glyph's pin-"1" location, -40 on X), not at the node centre.
func TestNetRoutesFromPins(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}},
		Nets: []*ir.Net{{Name: "N", Connections: []*ir.Connection{
			{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "R2", PinRef: "1"},
		}}},
	}
	g := layout(d)
	origin := placementOrigins(g)

	starts := map[[2]int64]bool{}
	for _, w := range g.Sheets[0].Wires {
		for _, pl := range w.Polylines {
			starts[[2]int64{pl.Points[0].X, pl.Points[0].Y}] = true
		}
	}

	// The resistor glyph's pin "1" sits at symbol-local (-terminalX, 0), so its world point is the
	// node origin shifted -terminalX on X.
	for _, ref := range []string{"R1", "R2"} {
		pin := [2]int64{origin[ref].X - terminalX, origin[ref].Y}
		if !starts[pin] {
			t.Errorf("no edge starts at %s's pin %v; edges start at %v", ref, pin, starts)
		}
		if center := [2]int64{origin[ref].X, origin[ref].Y}; starts[center] {
			t.Errorf("an edge starts at %s's centre %v; it should start at the pin", ref, center)
		}
	}
}

// TestJunctionDotAtConvergence asserts a net where three or more pins meet gets one junction dot at
// the convergence, while a two-pin net gets none.
func TestJunctionDotAtConvergence(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}, {RefDes: "R3"}},
		Nets: []*ir.Net{
			{Name: "TRI", Connections: []*ir.Connection{
				{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "R2", PinRef: "1"}, {ComponentRef: "R3", PinRef: "1"},
			}},
			{Name: "PAIR", Connections: []*ir.Connection{
				{ComponentRef: "R1", PinRef: "2"}, {ComponentRef: "R2", PinRef: "2"},
			}},
		},
	}
	// Dots are now: one junction at the 3-pin net's hub, plus one connection dot per
	// distinct attach point (five: R1.1/R2.1/R3.1 and R1.2/R2.2). The 2-pin net's hub
	// still gets no junction dot, so its dot count stays exactly its two endpoints.
	g := layout(d)
	dots := map[[2]int64]int{}
	for _, s := range g.Sheets[0].Shapes {
		if s.Kind == geom.Shape_KIND_DOT {
			dots[[2]int64{s.Points[0].X, s.Points[0].Y}]++
		}
	}
	total := 0
	for _, n := range dots {
		total += n
	}
	if total != 6 {
		t.Errorf("dots = %d, want 6 (1 hub junction + 5 attach points)", total)
	}
	for k, n := range dots {
		if n > 1 {
			t.Errorf("point %v has %d stacked dots, want deduped", k, n)
		}
	}
}
