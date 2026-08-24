package builtin

import (
	"fmt"
	"math"
	"sort"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"strconv"
)

// copperClearance flags cross-net track segments on the same layer whose copper edges
// sit closer than the fabrication clearance floor. It is the one rule of the WS3-008
// batch written as a Go Eval: the check is a pairwise join across entities (every
// segment of net A against every segment of net B), which the Spec AST cannot express.
// A cross-entity/spatial vocabulary needs more rules than this one behind it before it
// earns AST nodes (the WS3-003 earn-it guard;
// docsite/content/architecture/rules-and-checks.md). This rule and its O(S²) walk are
// the standing evidence for the WS3-004 spatial-index question; BenchmarkCopperClearance
// tracks the cost.
var copperClearance = &check.Rule{
	Name:       "copper-clearance",
	Severity:   "error",
	Summary:    "Copper of two different nets sits closer than the 0.127mm fabrication floor.",
	Impact:     "Sub-clearance copper either fails DFM at order time or ships as a latent short: etch variance and solder bridging turn a too-tight gap into a connection the netlist never had. It is the defect DRC exists for.",
	Remedy:     "Pull the two nets apart to the fab's minimum clearance. A gap below it is a short waiting on etch variance or a solder bridge.",
	Primitives: []string{"select", "geometry-distance"},
	Reads:      []string{"board.copper"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("copper-clearance"),
	// The two nets, ordered by name. This relation is SYMMETRIC, so the rule canonicalises the pair
	// itself rather than leaving it to a consumer: (GND, VBUS) and (VBUS, GND) are one violation and
	// must be one id. A framework that sorted every tuple would break the directional rules next door,
	// which is why ordering belongs to the rule.
	SubjectShape:        []string{check.KindNet, check.KindNet},
	Eval:                copperClearanceVerdicts,
	StatesConsideredSet: true,
}

// copperClearanceVerdicts decides every PAIR of nets whose copper shares a layer and comes close
// enough to be worth measuring, one verdict per pair.
//
// THE PAIR IS THE SUBJECT because a distance belongs to neither net. Filing under one of them was
// always a reporting compromise: a net running between two others is the filed subject of two
// findings, and under a single-entity id those two answers shared one name and one report link.
//
// THE CONSIDERED SET IS THE PAIRS THE WALK MEASURED, not every pair of nets on the board. That is a
// deliberate narrowing and the reason is cost: the bounding-box reject is what keeps this O(S²) walk
// affordable, and a pair it rejects has no computed distance at all. Claiming a pass over every
// possible pair would be claiming a measurement the walk never made. What the rule can honestly say
// is which pairs came near enough to measure and how they came out, which is also the set a reviewer
// cares about.
//
// A net with no track segments therefore appears in no pair, and that is correct rather than a gap:
// this rule compares SEGMENTS, so a net present only as vias and pads was never measured.
func copperClearanceVerdicts(m check.Model) []check.Verdict {
	type flatSeg struct {
		net string
		s   check.BoardSeg
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
		near  int // pairs that came within the bounding-box reject but cleared the floor
	}
	pairs := map[pairKey]*worst{}
	var order []pairKey
	for i := range segs {
		for j := i + 1; j < len(segs); j++ {
			a, b := segs[i], segs[j]
			if a.net == b.net || a.s.Layer != b.s.Layer {
				continue
			}
			if !bboxNear(a.s, b.s, minClearanceNm) {
				continue // too far apart to be worth measuring, so this pair is not a subject
			}
			k := pairKey{a.net, b.net}
			if k.a > k.b {
				k.a, k.b = k.b, k.a // the pair is symmetric, so its name is canonical
			}
			w := pairs[k]
			if w == nil {
				w = &worst{gap: minClearanceNm}
				pairs[k] = w
				order = append(order, k)
			}
			gap := segDistNm(a.s.A, a.s.B, b.s.A, b.s.B) - (a.s.Width+b.s.Width)/2
			if gap >= minClearanceNm {
				w.near++
				continue
			}
			w.count++
			if w.at == nil || gap < w.gap {
				w.gap, w.at = gap, a.s.A
			}
		}
	}
	sort.Slice(order, func(i, j int) bool { // map order is random; verdicts are not
		if order[i].a != order[j].a {
			return order[i].a < order[j].a
		}
		return order[i].b < order[j].b
	})

	out := make([]check.Verdict, 0, len(order))
	for _, k := range order {
		w := pairs[k]
		// NAME-ONLY entities, deliberately. Board copper joins the netlist by name (CONSTRAINTS C21),
		// so this rule genuinely holds its nets by name and has no per-instance id to carry. Resolving
		// one by name lookup would pick arbitrarily between two same-named nets and state an instance
		// the walk never distinguished.
		subjects := []check.Entity{check.NetNameEntity(k.a), check.NetNameEntity(k.b)}
		v := check.Verdict{Subjects: subjects}
		if w.count == 0 {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("copper of %q and %q comes close enough to measure in %d place(s) on a shared layer and never within 0.127mm",
					k.a, k.b, w.near),
				Terms: []check.WitnessTerm{{Label: "places measured", Value: strconv.Itoa(w.near)}},
			}
			out = append(out, v)
			continue
		}
		msg := fmt.Sprintf("copper of %q and %q closer than 0.127mm at %d place(s); worst gap %.3fmm near (%.2f, %.2f)mm",
			k.a, k.b, w.count, float64(w.gap)/1e6, float64(w.at.X)/1e6, float64(w.at.Y)/1e6)
		v.Outcome = check.Fail
		v.Witness = &check.Witness{
			Statement: msg,
			Terms: []check.WitnessTerm{
				{Label: "places under the floor", Value: strconv.Itoa(w.count)},
				{Label: "worst gap", Value: fmt.Sprintf("%.3fmm", float64(w.gap)/1e6)},
			},
		}
		v.Finding = &check.Finding{
			Subject: subjects[0],
			Message: msg,
			// The other net in the pair. The FINDING's subject is one of the two, since a reader is
			// told one place to go and look, so the other end had no way back into the drawing
			// (agni issue 349). The verdict names both, which is what the violation is about.
			Context: []check.ContextSubject{check.Ctx(subjects[1], "neighbour")},
		}
		out = append(out, v)
	}
	return out
}

// bboxNear is the cheap reject: whether two segments' bounding boxes, inflated by the
// clearance plus both half-widths, overlap. Everything that could violate passes this.
func bboxNear(a, b check.BoardSeg, clearance int64) bool {
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
