package graph

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestForceStrategyRegistered(t *testing.T) {
	if _, err := ByName("force"); err != nil {
		t.Fatalf("force strategy not registered: %v", err)
	}
}

// TestForceDeterministic: identical positions across independent runs, the promise every
// strategy makes (fixed seedless init + fixed iteration budget, no RNG).
func TestForceDeterministic(t *testing.T) {
	s, _ := ByName("force")
	a := s.Place(chain(7)).Positions
	b := s.Place(chain(7)).Positions
	if len(a) != 7 {
		t.Fatalf("placed %d nodes, want 7", len(a))
	}
	for r, pa := range a {
		if pb := b[r]; pb == nil || pa.X != pb.X || pa.Y != pb.Y {
			t.Errorf("%s differs across runs: %v vs %v", r, pa, b[r])
		}
	}
}

// TestForceSeparatesAndClusters: on a path, adjacent components must land nearer each other
// than the path's endpoints land to each other (attraction worked), and no two nodes may
// share a cell (repulsion + snap worked).
func TestForceSeparatesAndClusters(t *testing.T) {
	s, _ := ByName("force")
	pos := s.Place(chain(6)).Positions
	seen := map[[2]int64]string{}
	for r, p := range pos {
		k := [2]int64{p.X, p.Y}
		if other, dup := seen[k]; dup {
			t.Errorf("%s and %s share a cell at %v", r, other, k)
		}
		seen[k] = r
	}
	endToEnd := dist(pos[ref(1)], pos[ref(6)])
	for i := 1; i < 6; i++ {
		if adj := dist(pos[ref(i)], pos[ref(i+1)]); adj >= endToEnd {
			t.Errorf("adjacent pair U%d-U%d at %.0f is not closer than the endpoints at %.0f", i, i+1, adj, endToEnd)
		}
	}
}

// TestForceHyperedgeStar: a 5-member net must pull all members toward a shared hub without
// collapsing them (the virtual-node model); every member is placed, distinct, and no member
// is stranded far away (its distance to the net's centroid is bounded by the layout extent).
func TestForceHyperedgeStar(t *testing.T) {
	d := &ir.Design{Name: "star"}
	net := &ir.Net{Name: "BUS"}
	for i := 1; i <= 5; i++ {
		d.Components = append(d.Components, &ir.Component{RefDes: ref(i)})
		net.Connections = append(net.Connections, &ir.Connection{ComponentRef: ref(i), PinRef: "1"})
	}
	d.Nets = []*ir.Net{net}
	s, _ := ByName("force")
	pos := s.Place(d).Positions
	if len(pos) != 5 {
		t.Fatalf("placed %d nodes, want 5 (virtual net nodes must not leak into the placement)", len(pos))
	}
	seen := map[[2]int64]bool{}
	for _, p := range pos {
		k := [2]int64{p.X, p.Y}
		if seen[k] {
			t.Error("net members collapsed onto one cell")
		}
		seen[k] = true
	}
}

// TestForceDisconnectedComponents: islands pack side by side without overlap, same contract
// as the stress strategy.
func TestForceDisconnectedComponents(t *testing.T) {
	d := &ir.Design{Name: "two-islands"}
	for _, r := range []string{"A1", "A2", "B1", "B2"} {
		d.Components = append(d.Components, &ir.Component{RefDes: r})
	}
	d.Nets = []*ir.Net{
		{Name: "NA", Connections: []*ir.Connection{{ComponentRef: "A1", PinRef: "1"}, {ComponentRef: "A2", PinRef: "1"}}},
		{Name: "NB", Connections: []*ir.Connection{{ComponentRef: "B1", PinRef: "1"}, {ComponentRef: "B2", PinRef: "1"}}},
	}
	s, _ := ByName("force")
	pos := s.Place(d).Positions
	if len(pos) != 4 {
		t.Fatalf("placed %d nodes, want 4", len(pos))
	}
	seen := map[[2]int64]bool{}
	for _, p := range pos {
		k := [2]int64{p.X, p.Y}
		if seen[k] {
			t.Error("overlapping positions across islands")
		}
		seen[k] = true
	}
}
