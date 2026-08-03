package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestOrthogonalStrategyRegistered(t *testing.T) {
	if _, err := ByName("orthogonal"); err != nil {
		t.Fatalf("orthogonal strategy not registered: %v", err)
	}
}

// TestOrthogonalSegmentsAxisAligned is the ticket's core promise: every wire segment in the
// assembled geometry runs horizontally or vertically, never diagonally.
func TestOrthogonalSegmentsAxisAligned(t *testing.T) {
	g, err := LayoutWith(chain(6), "orthogonal")
	if err != nil {
		t.Fatal(err)
	}
	segs := 0
	for _, w := range g.Sheets[0].Wires {
		for _, pl := range w.Polylines {
			for i := 1; i < len(pl.Points); i++ {
				a, b := pl.Points[i-1], pl.Points[i]
				if a.X != b.X && a.Y != b.Y {
					t.Errorf("net %s has a diagonal segment (%d,%d)-(%d,%d)", w.Net, a.X, a.Y, b.X, b.Y)
				}
				segs++
			}
		}
	}
	if segs == 0 {
		t.Fatal("no wire segments drawn")
	}
}

// TestOrthogonalRoutesConnect: each polyline of a net starts at a member attach point and
// ends at the net's shared hub, so the net is electrically contiguous on screen.
func TestOrthogonalRoutesConnect(t *testing.T) {
	d := &ir.Design{Name: "star"}
	net := &ir.Net{Name: "BUS"}
	for i := 1; i <= 4; i++ {
		d.Components = append(d.Components, &ir.Component{RefDes: ref(i)})
		net.Connections = append(net.Connections, &ir.Connection{ComponentRef: ref(i), PinRef: "1"})
	}
	d.Nets = []*ir.Net{net}
	g, err := LayoutWith(d, "orthogonal")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Sheets[0].Wires) != 1 {
		t.Fatalf("wires = %d, want 1", len(g.Sheets[0].Wires))
	}
	polys := g.Sheets[0].Wires[0].Polylines
	if len(polys) != 4 {
		t.Fatalf("polylines = %d, want one per member", len(polys))
	}
	end := func(pl *geom.Polyline) [2]int64 {
		p := pl.Points[len(pl.Points)-1]
		return [2]int64{p.X, p.Y}
	}
	hub := end(polys[0])
	for i, pl := range polys {
		if end(pl) != hub {
			t.Errorf("polyline %d ends at %v, want the shared hub %v", i, end(pl), hub)
		}
	}
}

// TestConnectionDots: every wire attach point (each polyline's pin end) carries a dot shape,
// so a reader can see where a wire actually starts and ends; the hub junction dot for wide
// nets stays. Applies to every auto-layout route style.
func TestConnectionDots(t *testing.T) {
	for _, layout := range []string{"grid", "orthogonal"} {
		g, err := LayoutWith(chain(3), layout) // two 2-pin nets: previously dotless
		if err != nil {
			t.Fatal(err)
		}
		dots := map[[2]int64]bool{}
		for _, s := range g.Sheets[0].Shapes {
			if s.Kind == geom.Shape_KIND_DOT {
				dots[[2]int64{s.Points[0].X, s.Points[0].Y}] = true
			}
		}
		for _, w := range g.Sheets[0].Wires {
			for _, pl := range w.Polylines {
				start := pl.Points[0]
				if !dots[[2]int64{start.X, start.Y}] {
					t.Errorf("%s: net %s attach point (%d,%d) has no connection dot", layout, w.Net, start.X, start.Y)
				}
			}
		}
	}
}

// TestOrthogonalDeterministic: identical geometry across runs, same promise as every strategy.
func TestOrthogonalDeterministic(t *testing.T) {
	a, err := LayoutWith(chain(7), "orthogonal")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := LayoutWith(chain(7), "orthogonal")
	as, bs := a.Sheets[0], b.Sheets[0]
	if len(as.Wires) != len(bs.Wires) {
		t.Fatalf("wire counts differ: %d vs %d", len(as.Wires), len(bs.Wires))
	}
	for i := range as.Wires {
		for j := range as.Wires[i].Polylines {
			pa, pb := as.Wires[i].Polylines[j], bs.Wires[i].Polylines[j]
			for k := range pa.Points {
				if pa.Points[k].X != pb.Points[k].X || pa.Points[k].Y != pb.Points[k].Y {
					t.Fatalf("wire %d polyline %d point %d differs across runs", i, j, k)
				}
			}
		}
	}
}

// TestBendsCounted: an L-shaped route is exactly one bend; the star layouts (straight
// spokes) are zero. The metric is the ticket's other deliverable.
func TestBendsCounted(t *testing.T) {
	d := &ir.Design{Name: "bend"}
	// Two components joined by one net; layered puts them on different rows and columns is
	// not guaranteed, so use three in a triangle of nets to force at least one true L.
	for i := 1; i <= 3; i++ {
		d.Components = append(d.Components, &ir.Component{RefDes: ref(i)})
	}
	for i := 1; i <= 3; i++ {
		j := i%3 + 1
		d.Nets = append(d.Nets, &ir.Net{Name: netName(i), Connections: []*ir.Connection{
			{ComponentRef: ref(i), PinRef: "1"},
			{ComponentRef: ref(j), PinRef: "1"},
		}})
	}
	og, err := LayoutWith(d, "orthogonal")
	if err != nil {
		t.Fatal(err)
	}
	if q := Measure(og); q.Bends == 0 {
		t.Error("orthogonal layout of a triangle should have at least one bend")
	}
	sg, _ := LayoutWith(d, "grid")
	if q := Measure(sg); q.Bends != 0 {
		t.Errorf("straight-spoke star layout reports %d bends, want 0", q.Bends)
	}
}
