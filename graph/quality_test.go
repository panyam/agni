package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// sheetWith wraps some wires in the minimal geometry Measure needs.
func sheetWith(wires ...*geom.WireGeometry) *geom.SchematicGeometry {
	return &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Wires: wires}}}
}

func edge(ax, ay, bx, by int64) *geom.WireGeometry {
	return &geom.WireGeometry{Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: ax, Y: ay}, {X: bx, Y: by}}}}}
}

func TestMeasureCrossing(t *testing.T) {
	// Two segments forming an X cross once at their interior.
	q := Measure(sheetWith(edge(0, 0, 10, 10), edge(0, 10, 10, 0)))
	if q.Crossings != 1 {
		t.Errorf("crossing X: want 1 crossing, got %d", q.Crossings)
	}
	if q.Segments != 2 {
		t.Errorf("want 2 segments, got %d", q.Segments)
	}
}

func TestMeasureSharedEndpointIsNotACrossing(t *testing.T) {
	// Two segments meeting at a shared endpoint (a star spoke) must not count as a crossing.
	q := Measure(sheetWith(edge(0, 0, 10, 0), edge(0, 0, 0, 10)))
	if q.Crossings != 0 {
		t.Errorf("shared endpoint: want 0 crossings, got %d", q.Crossings)
	}
}

func TestMeasureTotalEdgeLen(t *testing.T) {
	// A 3-4-5 triangle leg pair: lengths 3 and 4, total 7.
	q := Measure(sheetWith(edge(0, 0, 3, 0), edge(0, 0, 0, 4)))
	if q.TotalEdgeLen != 7 {
		t.Errorf("want total edge length 7, got %g", q.TotalEdgeLen)
	}
}

func TestMeasureOnGridLayout(t *testing.T) {
	// The shipped grid layout produces a measurable score; the star spokes of each net share
	// a centroid, so a 2-member net contributes 2 segments and no self-crossing.
	q := Measure(layout(design()))
	if q.Nodes != 3 || q.Nets != 2 || q.Segments != 4 {
		t.Errorf("grid layout: got nodes=%d nets=%d segments=%d, want 3/2/4", q.Nodes, q.Nets, q.Segments)
	}
}

func TestGroundTruthResidual(t *testing.T) {
	truth := map[string]*geom.Point{
		"A": {X: 0, Y: 0}, "B": {X: 100, Y: 0}, "C": {X: 0, Y: 100}, "D": {X: 100, Y: 100},
	}
	// Identical -> 0.
	if r := GroundTruthResidual(truth, truth); r > 1e-9 {
		t.Errorf("identical sets: residual = %v, want ~0", r)
	}
	// A similarity transform of truth (rotate 90 CCW, scale 3, translate) -> ~0 (invariant).
	auto := map[string]*geom.Point{}
	for k, p := range truth {
		auto[k] = &geom.Point{X: -3*p.Y + 50, Y: 3*p.X - 20}
	}
	if r := GroundTruthResidual(auto, truth); r > 1e-6 {
		t.Errorf("similarity transform: residual = %v, want ~0", r)
	}
	// A scrambled arrangement (swap two corners) is not a similarity of truth -> clearly > 0.
	scr := map[string]*geom.Point{
		"A": {X: 0, Y: 0}, "B": {X: 0, Y: 100}, "C": {X: 100, Y: 0}, "D": {X: 100, Y: 100},
	}
	if r := GroundTruthResidual(scr, truth); r < 0.1 {
		t.Errorf("scrambled: residual = %v, want clearly > 0", r)
	}
	// Fewer than two matches -> -1.
	if r := GroundTruthResidual(map[string]*geom.Point{"A": {}}, truth); r != -1 {
		t.Errorf("one match: residual = %v, want -1", r)
	}
}
