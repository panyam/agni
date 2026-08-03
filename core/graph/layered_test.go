package graph

import (
	"fmt"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// chainDesign builds R1-R2-...-Rn where each consecutive pair shares a net. A grid layout
// wraps this into a square block, so the chain snakes and its net segments cross; a layered
// layout ranks it into a line, so consecutive nets do not cross.
func chainDesign(n int) *ir.Design {
	d := &ir.Design{Name: "chain"}
	for i := 1; i <= n; i++ {
		d.Components = append(d.Components, &ir.Component{RefDes: fmt.Sprintf("R%d", i)})
	}
	for i := 1; i < n; i++ {
		d.Nets = append(d.Nets, &ir.Net{
			Name: fmt.Sprintf("N%d", i),
			Connections: []*ir.Connection{
				{ComponentRef: fmt.Sprintf("R%d", i)},
				{ComponentRef: fmt.Sprintf("R%d", i+1)},
			},
		})
	}
	return d
}

// cyclicDesign has cycles (a U1-U2-U3 triangle plus a U4 feedback edge) to confirm layered
// layout handles cyclic netlists (the common case: feedback loops, power/ground) without
// looping or dropping nodes.
func cyclicDesign() *ir.Design {
	net := func(name string, refs ...string) *ir.Net {
		n := &ir.Net{Name: name}
		for _, r := range refs {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: r})
		}
		return n
	}
	return &ir.Design{
		Name:       "cyclic",
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}, {RefDes: "U3"}, {RefDes: "U4"}},
		Nets: []*ir.Net{
			net("A", "U1", "U2"),
			net("B", "U2", "U3"),
			net("C", "U3", "U1"), // closes the U1-U2-U3 triangle
			net("D", "U3", "U4"),
			net("E", "U4", "U1"), // feedback edge
		},
	}
}

func layeredGeom(t *testing.T, d *ir.Design) map[string]*geom.Point {
	t.Helper()
	g, err := LayoutWith(d, "layered")
	if err != nil {
		t.Fatal(err)
	}
	return placementOrigins(g)
}

func TestLayeredBeatsGridOnCrossings(t *testing.T) {
	d := chainDesign(12)
	grid := Measure(layout(d))
	lay := layeredMeasure(t, d)
	if lay.Crossings >= grid.Crossings {
		t.Errorf("layered should have fewer crossings than grid on a chain: grid=%d layered=%d",
			grid.Crossings, lay.Crossings)
	}
	// A chain has a crossing-free layered layout (a straight line of rows).
	if lay.Crossings != 0 {
		t.Errorf("chain should lay out crossing-free, got %d crossings", lay.Crossings)
	}
}

func TestLayeredDeterministic(t *testing.T) {
	d := chainDesign(12)
	a, b := layeredGeom(t, d), layeredGeom(t, d)
	for ref, pa := range a {
		pb, ok := b[ref]
		if !ok || pa.X != pb.X || pa.Y != pb.Y {
			t.Errorf("%s placed non-deterministically: %v vs %v", ref, pa, pb)
		}
	}
}

func TestLayeredPlacesEveryComponent(t *testing.T) {
	d := chainDesign(12)
	pos := layeredGeom(t, d)
	if len(pos) != len(d.Components) {
		t.Errorf("want %d placed nodes, got %d", len(d.Components), len(pos))
	}
	for _, c := range d.Components {
		if _, ok := pos[c.RefDes]; !ok {
			t.Errorf("component %s was not placed", c.RefDes)
		}
	}
}

func TestLayeredHandlesCycles(t *testing.T) {
	d := cyclicDesign()
	pos := layeredGeom(t, d) // must terminate despite the cycles
	if len(pos) != len(d.Components) {
		t.Errorf("cyclic design: want %d placed nodes, got %d", len(d.Components), len(pos))
	}
}

func layeredMeasure(t *testing.T, d *ir.Design) Quality {
	t.Helper()
	g, err := LayoutWith(d, "layered")
	if err != nil {
		t.Fatal(err)
	}
	return Measure(g)
}
