package graph

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Quality scores a produced layout with numbers comparable across layout algorithms on the same
// design. It reads the finished geometry, so it works for every strategy unchanged.
//
// Crossings is the headline readability metric and is scale-invariant. TotalEdgeLen is in layout
// units, so it is only comparable between layouts of the *same* design.
//
// Stress is the gap between drawn distance and graph-theoretic distance (Kruskal's normalized
// stress-1, see StressOf). Graph distances live in the design, so Measure leaves it zero and
// MeasureWith fills it.
type Quality struct {
	Nodes        int     // placed component nodes
	Nets         int     // drawn net hyperedges
	Segments     int     // straight-line segments across all edges
	Crossings    int     // pairs of segments that properly cross (excludes shared endpoints)
	Bends        int     // direction changes within wire polylines (orthogonal routing's cost)
	TotalEdgeLen float64 // sum of segment lengths, in layout units
	Stress       float64 // normalized stress-1 vs graph distance (0 = perfect; MeasureWith only)
}

// segment is one straight run between two points, carrying its endpoints for the
// proper-intersection test.
type segment struct{ ax, ay, bx, by int64 }

// Measure scores the layout on sheet 0. Crossing detection is O(segments^2), so it is slow on a large
// board.
func Measure(g *geom.SchematicGeometry) Quality {
	sheet := g.Sheets[0]
	q := Quality{Nodes: len(sheet.Placements), Nets: len(sheet.Wires)}

	segs := make([]segment, 0)
	for _, w := range sheet.Wires {
		for _, pl := range w.Polylines {
			for i := 0; i+1 < len(pl.Points); i++ {
				a, b := pl.Points[i], pl.Points[i+1]
				segs = append(segs, segment{a.X, a.Y, b.X, b.Y})
				q.TotalEdgeLen += math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y))
				if i > 0 && bendsAt(pl.Points[i-1], a, b) {
					q.Bends++
				}
			}
		}
	}
	q.Segments = len(segs)

	for i := 0; i < len(segs); i++ {
		for j := i + 1; j < len(segs); j++ {
			if properCross(segs[i], segs[j]) {
				q.Crossings++
			}
		}
	}
	return q
}

// bendsAt reports whether the polyline changes direction at b (the run a->b->c is not
// collinear). Zero-length runs do not bend.
func bendsAt(a, b, c *geom.Point) bool {
	if (a.X == b.X && a.Y == b.Y) || (b.X == c.X && b.Y == c.Y) {
		return false
	}
	return orient(a.X, a.Y, b.X, b.Y, c.X, c.Y) != 0
}

// properCross reports whether two segments cross at an interior point of both. Segments that only
// touch at a shared endpoint (every edge of one net's star at the centroid, every edge leaving one
// node at that node) do not count, since those do not harm readability.
func properCross(s, t segment) bool {
	d1 := orient(t.ax, t.ay, t.bx, t.by, s.ax, s.ay)
	d2 := orient(t.ax, t.ay, t.bx, t.by, s.bx, s.by)
	d3 := orient(s.ax, s.ay, s.bx, s.by, t.ax, t.ay)
	d4 := orient(s.ax, s.ay, s.bx, s.by, t.bx, t.by)
	return ((d1 > 0) != (d2 > 0)) && ((d3 > 0) != (d4 > 0))
}

// orient returns the sign of the cross product (b-a)x(c-a): >0 left turn, <0 right, 0
// collinear. Coordinates are grid-scale, so the products stay well within int64.
func orient(ax, ay, bx, by, cx, cy int64) int64 {
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
}

// GroundTruthResidual scores how closely an auto-layout reproduces a set of ground-truth node
// positions (e.g. a design's real schematic placement), as the normalized Procrustes residual. It
// matches nodes by ref_des, fits auto onto truth under translation, uniform scale and rotation, and
// returns the residual in [0,1], where 0 is the same arrangement up to a similarity transform and 1
// is unrelated. It returns -1 when fewer than two nodes match or a set is degenerate.
func GroundTruthResidual(auto, truth map[string]*geom.Point) float64 {
	type pair struct{ ax, ay, tx, ty float64 }
	var ps []pair
	for ref, tp := range truth {
		if ap, ok := auto[ref]; ok && ap != nil && tp != nil {
			ps = append(ps, pair{float64(ap.X), float64(ap.Y), float64(tp.X), float64(tp.Y)})
		}
	}
	if len(ps) < 2 {
		return -1
	}
	var acx, acy, tcx, tcy float64
	for _, p := range ps {
		acx, acy, tcx, tcy = acx+p.ax, acy+p.ay, tcx+p.tx, tcy+p.ty
	}
	n := float64(len(ps))
	acx, acy, tcx, tcy = acx/n, acy/n, tcx/n, tcy/n
	// num/den give the optimal rotation; aN/tN are the centered sum-of-squares. The Procrustes
	// correlation is (num^2+den^2)/(aN*tN); residual = sqrt(1 - that).
	var num, den, aN, tN float64
	for _, p := range ps {
		aX, aY := p.ax-acx, p.ay-acy
		tX, tY := p.tx-tcx, p.ty-tcy
		num += aX*tY - aY*tX
		den += aX*tX + aY*tY
		aN += aX*aX + aY*aY
		tN += tX*tX + tY*tY
	}
	if aN == 0 || tN == 0 {
		return -1
	}
	r2 := (num*num + den*den) / (aN * tN)
	if r2 > 1 {
		r2 = 1
	}
	return math.Sqrt(1 - r2)
}
