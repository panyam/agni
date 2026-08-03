package graph

import (
	"math"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// chain builds a design whose components form a path: U1-U2-...-Un, each consecutive pair
// sharing one two-pin net.
func chain(n int) *ir.Design {
	d := &ir.Design{Name: "chain"}
	for i := 1; i <= n; i++ {
		d.Components = append(d.Components, &ir.Component{RefDes: ref(i)})
	}
	for i := 1; i < n; i++ {
		d.Nets = append(d.Nets, &ir.Net{Name: netName(i), Connections: []*ir.Connection{
			{ComponentRef: ref(i), PinRef: "1"},
			{ComponentRef: ref(i + 1), PinRef: "1"},
		}})
	}
	return d
}

func ref(i int) string     { return "U" + string(rune('0'+i)) }
func netName(i int) string { return "N" + string(rune('0'+i)) }

func dist(a, b *geom.Point) float64 {
	dx, dy := float64(a.X-b.X), float64(a.Y-b.Y)
	return math.Hypot(dx, dy)
}

func TestStressStrategyRegistered(t *testing.T) {
	if _, err := ByName("stress"); err != nil {
		t.Fatalf("stress strategy not registered: %v", err)
	}
}

// TestStressPathEmbedding: on a path graph, drawn distance must grow with graph distance
// from an endpoint (the shape stress-majorization exists to produce), and no two nodes may
// coincide.
func TestStressPathEmbedding(t *testing.T) {
	s, err := ByName("stress")
	if err != nil {
		t.Fatal(err)
	}
	pos := s.Place(chain(5)).Positions
	if len(pos) != 5 {
		t.Fatalf("placed %d nodes, want 5", len(pos))
	}
	for i := 1; i < 5; i++ {
		a, b := pos[ref(1)], pos[ref(i+1)]
		if dist(a, b) <= dist(a, pos[ref(i)]) {
			t.Errorf("dist(U1,U%d)=%f not greater than dist(U1,U%d)=%f; path not embedded monotonically",
				i+1, dist(a, b), i, dist(a, pos[ref(i)]))
		}
	}
	seen := map[[2]int64]string{}
	for r, p := range pos {
		k := [2]int64{p.X, p.Y}
		if other, dup := seen[k]; dup {
			t.Errorf("%s and %s coincide at %v", r, other, k)
		}
		seen[k] = r
	}
}

// TestStressDeterministic: two independent runs must produce identical positions (the layout
// promise every strategy makes; diff stability depends on it).
func TestStressDeterministic(t *testing.T) {
	s, _ := ByName("stress")
	a := s.Place(chain(7)).Positions
	b := s.Place(chain(7)).Positions
	for r, pa := range a {
		if pb := b[r]; pb == nil || pa.X != pb.X || pa.Y != pb.Y {
			t.Errorf("%s differs across runs: %v vs %v", r, pa, b[r])
		}
	}
}

// TestStressDisconnectedComponents: two subcircuits with no shared net must both be placed,
// at distinct non-overlapping positions.
func TestStressDisconnectedComponents(t *testing.T) {
	d := &ir.Design{Name: "two-islands"}
	for _, r := range []string{"A1", "A2", "B1", "B2"} {
		d.Components = append(d.Components, &ir.Component{RefDes: r})
	}
	d.Nets = []*ir.Net{
		{Name: "NA", Connections: []*ir.Connection{{ComponentRef: "A1", PinRef: "1"}, {ComponentRef: "A2", PinRef: "1"}}},
		{Name: "NB", Connections: []*ir.Connection{{ComponentRef: "B1", PinRef: "1"}, {ComponentRef: "B2", PinRef: "1"}}},
	}
	s, _ := ByName("stress")
	pos := s.Place(d).Positions
	if len(pos) != 4 {
		t.Fatalf("placed %d nodes, want 4", len(pos))
	}
	seen := map[[2]int64]bool{}
	for _, p := range pos {
		k := [2]int64{p.X, p.Y}
		if seen[k] {
			t.Errorf("overlapping positions at %v", k)
		}
		seen[k] = true
	}
}

// TestStressOfImprovesOnLayered: the stress layout must score at or below layered on its own
// objective, and StressOf must be normalized (0 <= s, and well under 1 for a sane layout of a
// simple graph).
func TestStressOfImprovesOnLayered(t *testing.T) {
	d := chain(6)
	stressS, _ := ByName("stress")
	layeredS, _ := ByName("layered")
	ss := StressOf(d, stressS.Place(d).Positions)
	sl := StressOf(d, layeredS.Place(d).Positions)
	if ss > sl+1e-9 {
		t.Errorf("stress layout scores %f, worse than layered %f on its own objective", ss, sl)
	}
	if ss < 0 || ss >= 1 {
		t.Errorf("normalized stress = %f, want in [0,1)", ss)
	}
}

// TestMeasureWithFillsStress: the design-aware Measure companion populates Quality.Stress for
// ANY strategy (that is the point of the metric: grid and layered get scored too).
func TestMeasureWithFillsStress(t *testing.T) {
	d := chain(4)
	g, err := LayoutWith(d, "grid")
	if err != nil {
		t.Fatal(err)
	}
	q := MeasureWith(d, g)
	if q.Stress <= 0 {
		t.Errorf("grid layout of a path should have positive stress, got %f", q.Stress)
	}
	if q.Nodes != 4 {
		t.Errorf("MeasureWith must keep Measure's fields; nodes = %d, want 4", q.Nodes)
	}
}
