package builtin

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// benchBoard synthesizes nSegs track segments spread over nNets nets on two layers in a
// grid pitched safely above the clearance floor, so the benchmark measures the pairwise
// walk (with its bbox reject), not finding construction.
func benchBoard(nSegs, nNets int) *geom.BoardGeometry {
	g := &geom.BoardGeometry{UnitNm: 1}
	nets := make([]*geom.NetCopper, nNets)
	for i := range nets {
		nets[i] = &geom.NetCopper{Net: fmt.Sprintf("N%03d", i)}
	}
	layers := []string{"F.Cu", "B.Cu"}
	for i := range nSegs {
		row, col := i/50, i%50
		s := &geom.TrackSegment{
			A:     &geom.Point{X: int64(col) * 2_000_000, Y: int64(row) * 2_000_000},
			B:     &geom.Point{X: int64(col)*2_000_000 + 1_500_000, Y: int64(row) * 2_000_000},
			Width: 200_000,
			Layer: layers[i%2],
		}
		nc := nets[i%nNets]
		nc.Segments = append(nc.Segments, s)
	}
	g.Nets = nets
	return g
}

// BenchmarkCopperClearance is the standing evidence for the WS3-004 spatial-index
// question. The pairwise walk is O(S^2) with a cheap bbox reject. Corpus boards top out
// near 400 segments; the sizes below bracket where the naive walk stops being free.
func BenchmarkCopperClearance(b *testing.B) {
	for _, n := range []int{400, 2000, 10000} {
		b.Run(fmt.Sprintf("segs=%d", n), func(b *testing.B) {
			m := check.NewModel(&ir.Design{}, check.WithBoard(benchBoard(n, 40)))
			b.ResetTimer()
			for b.Loop() {
				copperClearance.Findings(context.Background(), m)
			}
		})
	}
}

// planeBoard is benchBoard with a GND plane on each layer, as a board pours one: a pour over the
// whole board with a hole cut around every track of another net, at exactly the clearance floor
// (agni 1410). Each hole is a stadium written as 32 short edges, as a file writes a plane's rounded
// clearance, so a grid cell holds dozens of the plane's own edges, which a query must step over.
func planeBoard(nSegs, nNets int) *geom.BoardGeometry {
	g := benchBoard(nSegs, nNets)
	rect := func(x0, y0, x1, y1 int64) *geom.Polyline {
		return &geom.Polyline{Points: []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}
	}
	rows := int64(nSegs/50 + 1)
	gnd := &geom.NetCopper{Net: "GND"}
	for _, layer := range []string{"F.Cu", "B.Cu"} {
		sh := &geom.CopperShape{Layer: layer, Outline: rect(-1_000_000, -1_000_000, 100_000_000, rows*2_000_000)}
		for _, nc := range g.Nets {
			for _, s := range nc.Segments {
				if s.Layer == layer {
					// 0.227mm from the centreline is the 0.127mm floor past the track's 0.1mm half width.
					sh.Holes = append(sh.Holes, stadium(s.A, s.B, 227_000))
				}
			}
		}
		gnd.Shapes = append(gnd.Shapes, sh)
	}
	g.Nets = append(g.Nets, gnd)
	return g
}

// BenchmarkCopperClearancePlane tracks what a plane costs the rule (agni 1410), beside the tracks
// alone above.
func BenchmarkCopperClearancePlane(b *testing.B) {
	for _, n := range []int{400, 2000, 10000} {
		b.Run(fmt.Sprintf("segs=%d", n), func(b *testing.B) {
			m := check.NewModel(&ir.Design{}, check.WithBoard(planeBoard(n, 40)))
			b.ResetTimer()
			for b.Loop() {
				copperClearance.Findings(context.Background(), m)
			}
		})
	}
}

// TestAPlaneCostsTheRuleABoundedAmountOfWork holds the rule's work on a plane board to a budget
// (agni 1410), as the ESD plans are held to theirs. Work is every candidate pair or edge the rule
// looks at, so it does not vary with the machine. A change that has a track walk past the plane's
// own edges again, or the containment ray walk the plane's whole outline, multiplies it.
func TestAPlaneCostsTheRuleABoundedAmountOfWork(t *testing.T) {
	m := check.NewModel(&ir.Design{}, check.WithBoard(planeBoard(2000, 40)))
	vs, work := copperClearanceWork(context.Background(), m)
	t.Logf("%d verdicts, work %d", len(vs), work)
	// The positive control: the plane is measured. Each track's hole leaves exactly the floor, so
	// every track's net meets the plane and passes.
	passes := 0
	for _, v := range vs {
		if v.Outcome == check.Pass && (v.Subjects[0].Ref == "GND" || v.Subjects[1].Ref == "GND") {
			passes++
		}
		if v.Outcome == check.Fail {
			t.Errorf("unexpected finding: %s", v.Witness.Statement)
		}
	}
	if passes != 40 {
		t.Errorf("%d nets measured against the plane, want all 40", passes)
	}
	// Measured at 1,033,320 when this budget was set; the margin allows a quarter more. Stepping
	// through the plane's own edges one by one instead of skipping their run measured 1,886,758.
	const workBudget = 1_300_000
	if work > workBudget {
		t.Errorf("the rule did %d units of work on a 2,000-track plane board, over its budget of %d", work, workBudget)
	}
}

// stadium is the outline at distance r around a horizontal segment ab: two half circles joined by
// straight sides, each half circle written as 16 chords.
func stadium(a, b *geom.Point, r int64) *geom.Polyline {
	pl := &geom.Polyline{}
	for i := 0; i <= 15; i++ { // the right end, from the bottom round to the top
		t := -math.Pi/2 + math.Pi*float64(i)/15
		pl.Points = append(pl.Points, &geom.Point{X: b.X + int64(math.Round(float64(r)*math.Cos(t))), Y: b.Y + int64(math.Round(float64(r)*math.Sin(t)))})
	}
	for i := 0; i <= 15; i++ { // the left end, from the top round to the bottom
		t := math.Pi/2 + math.Pi*float64(i)/15
		pl.Points = append(pl.Points, &geom.Point{X: a.X + int64(math.Round(float64(r)*math.Cos(t))), Y: a.Y + int64(math.Round(float64(r)*math.Sin(t)))})
	}
	return pl
}
