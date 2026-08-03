package check

import (
	"fmt"
	"math"
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// copperClearance flags cross-net track segments on the same layer whose copper edges
// sit closer than the fabrication clearance floor. It is the one rule of the WS3-008
// batch written as a Go Eval: the check is a pairwise join across entities (every
// segment of net A against every segment of net B), which the Spec AST deliberately
// cannot express — a cross-entity/spatial vocabulary must be evidenced by more rules
// than this one before it earns AST nodes (docs/19, the WS3-003 earn-it guard). This
// rule and its O(S²) walk are also the standing evidence for the WS3-004 spatial-index
// question; BenchmarkCopperClearance tracks the cost.
var copperClearance = &Rule{
	Name:       "copper-clearance",
	Severity:   "error",
	Summary:    "Copper of two different nets sits closer than the 0.127mm fabrication floor.",
	Impact:     "Sub-clearance copper either fails DFM at order time or ships as a latent short: etch variance and solder bridging turn a too-tight gap into a connection the netlist never had. It is the defect DRC exists for.",
	Primitives: []string{"select", "geometry-distance"},
	Reads:      []string{"board.copper"},
	Tags: map[string]string{
		KeyCategory:     CategoryBoard,
		KeyTier:         "P",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("copper-clearance"),
	Eval: func(m Model) []Finding {
		type flatSeg struct {
			net string
			s   BoardSeg
		}
		var segs []flatSeg
		for _, bn := range m.BoardNets() {
			for _, s := range bn.Segments {
				segs = append(segs, flatSeg{net: bn.Net, s: s})
			}
		}
		type pairKey struct{ a, b string }
		type worst struct {
			gap   int64
			at    *geom.Point
			count int
		}
		pairs := map[pairKey]*worst{}
		for i := range segs {
			for j := i + 1; j < len(segs); j++ {
				a, b := segs[i], segs[j]
				if a.net == b.net || a.s.Layer != b.s.Layer {
					continue
				}
				if !bboxNear(a.s, b.s, minClearanceNm) {
					continue
				}
				gap := segDistNm(a.s.A, a.s.B, b.s.A, b.s.B) - (a.s.Width+b.s.Width)/2
				if gap >= minClearanceNm {
					continue
				}
				k := pairKey{a.net, b.net}
				if k.a > k.b {
					k.a, k.b = k.b, k.a
				}
				w := pairs[k]
				if w == nil {
					w = &worst{gap: gap, at: a.s.A}
					pairs[k] = w
				}
				w.count++
				if gap < w.gap {
					w.gap, w.at = gap, a.s.A
				}
			}
		}
		out := []Finding{}
		for k, w := range pairs {
			out = append(out, Finding{
				Kind:    KindNet,
				Subject: k.a,
				Message: fmt.Sprintf("copper of %q and %q closer than 0.127mm at %d place(s); worst gap %.3fmm near (%.2f, %.2f)mm",
					k.a, k.b, w.count, float64(w.gap)/1e6, float64(w.at.X)/1e6, float64(w.at.Y)/1e6),
			})
		}
		sort.Slice(out, func(i, j int) bool { // map order is random; findings are not
			if out[i].Subject != out[j].Subject {
				return out[i].Subject < out[j].Subject
			}
			return out[i].Message < out[j].Message
		})
		return out
	},
}

// bboxNear is the cheap reject: whether two segments' bounding boxes, inflated by the
// clearance plus both half-widths, overlap. Everything that could violate passes this.
func bboxNear(a, b BoardSeg, clearance int64) bool {
	pad := clearance + (a.Width+b.Width)/2
	return min64(a.A.X, a.B.X)-pad <= max64(b.A.X, b.B.X) &&
		min64(b.A.X, b.B.X)-pad <= max64(a.A.X, a.B.X) &&
		min64(a.A.Y, a.B.Y)-pad <= max64(b.A.Y, b.B.Y) &&
		min64(b.A.Y, b.B.Y)-pad <= max64(a.A.Y, a.B.Y)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// segDistNm is the minimum centerline distance between two 2D segments: zero when they
// intersect, otherwise the smallest of the four endpoint-to-segment distances.
func segDistNm(a1, a2, b1, b2 *geom.Point) int64 {
	if segsIntersect(a1, a2, b1, b2) {
		return 0
	}
	d := math.Min(
		math.Min(pointSegDist(a1, b1, b2), pointSegDist(a2, b1, b2)),
		math.Min(pointSegDist(b1, a1, a2), pointSegDist(b2, a1, a2)),
	)
	return int64(math.Round(d))
}

// pointSegDist is the distance from p to the segment ab.
func pointSegDist(p, a, b *geom.Point) float64 {
	px, py := float64(p.X), float64(p.Y)
	ax, ay := float64(a.X), float64(a.Y)
	bx, by := float64(b.X), float64(b.Y)
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// segsIntersect reports whether segments a1a2 and b1b2 cross (including touching).
func segsIntersect(a1, a2, b1, b2 *geom.Point) bool {
	o := func(p, q, r *geom.Point) int {
		v := (float64(q.X)-float64(p.X))*(float64(r.Y)-float64(p.Y)) -
			(float64(q.Y)-float64(p.Y))*(float64(r.X)-float64(p.X))
		switch {
		case v > 0:
			return 1
		case v < 0:
			return -1
		}
		return 0
	}
	on := func(p, q, r *geom.Point) bool { // r collinear with pq: does r sit on pq?
		return min64(p.X, q.X) <= r.X && r.X <= max64(p.X, q.X) &&
			min64(p.Y, q.Y) <= r.Y && r.Y <= max64(p.Y, q.Y)
	}
	o1, o2, o3, o4 := o(a1, a2, b1), o(a1, a2, b2), o(b1, b2, a1), o(b1, b2, a2)
	if o1 != o2 && o3 != o4 {
		return true
	}
	return (o1 == 0 && on(a1, a2, b1)) || (o2 == 0 && on(a1, a2, b2)) ||
		(o3 == 0 && on(b1, b2, a1)) || (o4 == 0 && on(b1, b2, a2))
}
