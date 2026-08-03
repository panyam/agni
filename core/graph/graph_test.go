package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// design builds a tiny 3-component, 2-net IR fixture: R1-R2 share NET_A, R2-R3 share NET_B.
func design() *ir.Design {
	return &ir.Design{
		Name: "tiny",
		Components: []*ir.Component{
			{RefDes: "R2"}, {RefDes: "R1"}, {RefDes: "R3"}, // deliberately unsorted
		},
		Nets: []*ir.Net{
			{Name: "NET_A", Connections: []*ir.Connection{
				{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "R2", PinRef: "1"},
			}},
			{Name: "NET_B", Connections: []*ir.Connection{
				{ComponentRef: "R2", PinRef: "2"}, {ComponentRef: "R3", PinRef: "1"},
			}},
		},
	}
}

func TestLayoutNodesAndEdges(t *testing.T) {
	g := layout(design())
	if len(g.Sheets) != 1 {
		t.Fatalf("want 1 sheet, got %d", len(g.Sheets))
	}
	sheet := g.Sheets[0]
	if len(sheet.Placements) != 3 {
		t.Errorf("want 3 node placements, got %d", len(sheet.Placements))
	}
	if len(sheet.Wires) != 2 {
		t.Errorf("want 2 net edges, got %d", len(sheet.Wires))
	}
	// Each 2-member net is a star: one polyline per member, ending at the shared centroid.
	for _, w := range sheet.Wires {
		if len(w.Polylines) != 2 {
			t.Errorf("net %q: want 2 polylines, got %d", w.Net, len(w.Polylines))
		}
		for _, p := range w.Polylines {
			if len(p.Points) != 2 {
				t.Errorf("net %q: want 2 points per edge, got %d", w.Net, len(p.Points))
			}
		}
	}
}

func TestLayoutDeterministic(t *testing.T) {
	// Same design lays out identically regardless of component input order, so a rendered
	// graph is stable for diffing. Compare placement (ref_des -> origin) across two runs.
	a := layout(design())
	b := layout(design())
	pa, pb := placementOrigins(a), placementOrigins(b)
	for ref, oa := range pa {
		ob, ok := pb[ref]
		if !ok || oa.X != ob.X || oa.Y != ob.Y {
			t.Errorf("%s placed non-deterministically: %v vs %v", ref, oa, ob)
		}
	}
	// R1 is first in ref_des order, so it anchors the grid origin.
	if o := pa["R1"]; o.X != 0 || o.Y != 0 {
		t.Errorf("R1 should anchor grid origin (0,0), got (%d,%d)", o.X, o.Y)
	}
}

func TestLayoutSkipsUnconnectedAndDangling(t *testing.T) {
	d := &ir.Design{
		Name:       "edge-cases",
		Components: []*ir.Component{{RefDes: "R1"}},
		Nets: []*ir.Net{
			// Single real member: nothing to draw between, so no edge.
			{Name: "SOLO", Connections: []*ir.Connection{{ComponentRef: "R1"}}},
			// Reference to a component not in the design: dangling, dropped.
			{Name: "DANGLE", Connections: []*ir.Connection{
				{ComponentRef: "R1"}, {ComponentRef: "R9"},
			}},
		},
	}
	if wires := layout(d).Sheets[0].Wires; len(wires) != 0 {
		t.Errorf("want 0 edges (one solo net, one dangling), got %d", len(wires))
	}
}

func placementOrigins(g *geom.SchematicGeometry) map[string]*geom.Point {
	m := map[string]*geom.Point{}
	for _, pl := range g.Sheets[0].Placements {
		m[pl.RefDes] = pl.Transform.Origin
	}
	return m
}
