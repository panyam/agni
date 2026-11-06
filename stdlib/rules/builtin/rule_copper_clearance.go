package builtin

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"strconv"
)

// clearanceEpsilonNm is how far under the floor a measured gap may sit and still meet it. It is
// KiCad's own DRC epsilon (its DRCEpsilon advanced setting, 0.0005mm by default). A gap between
// diagonal tracks is irrational in nanometres, so copper routed exactly at the minimum measures a
// nanometre or two under it. KiCad passes that gap, and a strict comparison failed it (16 pairs on
// the Jetson baseboard, agni 933).
const clearanceEpsilonNm = 500

// pourEdgeToleranceNm is how far under the floor a gap measured to a pour's edge may sit and still
// meet it (agni 1410). A pour's edge around a round pad or a track end is an arc, and the files
// write it as short straight runs, which cut inside the curve by up to the writer's arc error.
// KiCad fills to 5 microns (its ARC_HIGH_DEF), so a pour filled at exactly the clearance measures
// up to 5 microns short against its own edge. Without this, Jetson's ODB++ export gave 93 pairs at
// 0.125 or 0.126mm against a 0.127mm floor that KiCad's DRC passes.
const pourEdgeToleranceNm = 5000

// copperClearance flags copper of two nets on the same layer, tracks and pours, whose edges sit
// closer than the fabrication clearance floor (pours since agni 1410). It is the one rule of the WS3-008 batch written as a Go
// Eval. The check is a pairwise join across entities, every segment of net A against every segment
// of net B, and the Spec AST cannot express that join. A cross-entity or spatial vocabulary needs
// more rules behind it than this one before it gets AST nodes (the WS3-003 earn-it guard;
// docsite/content/architecture/rules-and-checks.md). The WS3-004 spatial-index question was about
// this rule and its O(S²) walk. The rule sweeps each layer's segments in x order rather than
// comparing every pair (agni issue 963), and BenchmarkCopperClearance tracks the cost.
var copperClearance = &check.Rule{
	Name:       "copper-clearance",
	Severity:   "error",
	Summary:    "Copper of two different nets sits closer than the minimum the board declares, or the 0.127mm fabrication floor when it declares none.",
	Impact:     "Sub-clearance copper either fails DFM at order time or ships as a latent short: etch variance and solder bridging turn a too-tight gap into a connection the netlist never had. It is the defect DRC exists for.",
	Remedy:     "Pull the two nets apart to the fab's minimum clearance. A gap below it is a short waiting on etch variance or a solder bridge.",
	Primitives: []string{"select", "geometry-distance"},
	Reads:      []string{"board.copper", "board.rules"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("copper-clearance"),
	// The two nets, ordered by name. This relation is SYMMETRIC, so the rule canonicalises the pair
	// itself rather than leaving it to a consumer, and (GND, VBUS) and (VBUS, GND) are one violation
	// with one id. Ordering belongs to the rule because a framework that sorted every tuple would
	// break the directional rules.
	SubjectShape:        []string{check.KindNet, check.KindNet},
	Eval:                copperClearanceVerdicts,
	StatesConsideredSet: true,
	// Asked about some nets, it measures only the segments of those nets against their neighbours
	// (agni issue 1070). Those are the same pairs a full sweep filtered to the nets would hold.
	Scoping: check.ScopeSubjects,
}

// copperClearanceVerdicts decides every PAIR of nets whose copper shares a layer and comes close
// enough to be worth measuring, one verdict per pair.
//
// THE PAIR IS THE SUBJECT because a distance belongs to neither net (#404). A net running between
// two others is party to two findings, and under a single-entity id those would share one name and
// one report link.
//
// THE CONSIDERED SET IS THE PAIRS THE WALK MEASURED, not every pair of nets on the board. The
// bounding-box reject keeps this O(S²) walk affordable. A pair it rejects has no computed distance
// at all, so a pass over every possible pair would claim a measurement the walk never made. The
// rule reports which pairs came near enough to measure and how they came out. A reviewer cares
// about the same set.
//
// The copper measured is each net's tracks and its pours (agni 1410). A pour enters as the edges of
// its outline and holes, and copper wholly inside another net's pour is a short. A net with no
// track and no pour appears in no pair, so a net present only as vias and pads is never measured.
func copperClearanceVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	vs, _ := copperClearanceWork(ctx, m)
	return vs
}

// copperClearanceWork is the rule's body. It also returns the work it did, every candidate pair or
// edge it looked at, so a test can hold the rule to a work budget the way the ESD plans are held to
// theirs (agni 1410). A budget on time would vary with the machine; this does not.
func copperClearanceWork(ctx context.Context, m check.Model) ([]check.Verdict, int64) {
	var work int64
	floor, floorDesc := floorFor(m, "clearance")
	// idx is a segment's place in the board's own order, which decides ties below.
	type flatSeg struct {
		net                    string
		s                      check.BoardSeg
		idx                    int
		edge                   bool // an edge of a pour rather than a track
		shape                  int  // the pour an edge belongs to, numbered across the board
		minX, maxX, minY, maxY int64
	}
	// Sized up front: a plane adds tens of thousands of edges, and regrowing the slice was a tenth of
	// the rule's time on the BeagleBone AI.
	n := 0
	for _, bn := range m.BoardNets() {
		n += len(bn.Segments)
		for _, sh := range bn.Shapes {
			n += len(sh.Outline)
			for _, h := range sh.Holes {
				n += len(h)
			}
		}
	}
	segs := make([]flatSeg, 0, n)
	addSeg := func(net string, s check.BoardSeg, edge bool, shape int) {
		segs = append(segs, flatSeg{
			net: net, s: s, idx: len(segs), edge: edge, shape: shape,
			minX: min64(s.A.X, s.B.X), maxX: max64(s.A.X, s.B.X),
			minY: min64(s.A.Y, s.B.Y), maxY: max64(s.A.Y, s.B.Y),
		})
	}
	for _, bn := range m.BoardNets() {
		for _, s := range bn.Segments {
			addSeg(bn.Net, s, false, -1)
		}
	}
	tracks := len(segs)
	var containedIn []overlap
	// A pour's copper ends at its outline and at the edge of each hole, so each edge of those rings
	// is a segment of no width on the pour's net (agni 1410). A track then measures against the pour
	// as it does against another track, and two nets' pours against each other. A plane has tens of
	// thousands of edges, so they are kept out of the sweep and found through a grid below.
	// pours lists each pour with its net and its outline's box, numbered as the edges name them.
	type pourBox struct {
		net, layer             string
		first                  *geom.Point
		minX, maxX, minY, maxY int64
	}
	var pours []pourBox
	for _, bn := range m.BoardNets() {
		for _, sh := range bn.Shapes {
			if len(sh.Outline) < 3 {
				continue
			}
			pb := pourBox{net: bn.Net, layer: sh.Layer, first: sh.Outline[0], minX: sh.Outline[0].X, maxX: sh.Outline[0].X, minY: sh.Outline[0].Y, maxY: sh.Outline[0].Y}
			for _, q := range sh.Outline {
				pb.minX, pb.maxX = min64(pb.minX, q.X), max64(pb.maxX, q.X)
				pb.minY, pb.maxY = min64(pb.minY, q.Y), max64(pb.maxY, q.Y)
			}
			shape := len(pours)
			pours = append(pours, pb)
			for _, ring := range append([][]*geom.Point{sh.Outline}, sh.Holes...) {
				for i := range ring {
					a, b := ring[i], ring[(i+1)%len(ring)]
					if a.X != b.X || a.Y != b.Y {
						addSeg(bn.Net, check.BoardSeg{Layer: sh.Layer, A: a, B: b}, true, shape)
					}
				}
			}
		}
	}

	type pairKey struct{ a, b string }
	type worst struct {
		gap   int64
		at    *geom.Point
		ai    int // the segment pair the worst gap was measured at, in board order
		bi    int
		count int
		near  int // pairs that came within the bounding-box reject but cleared the floor
	}
	pairs := map[pairKey]*worst{}
	var order []pairKey
	// measure is one segment pair, a before b in board order.
	measure := func(a, b *flatSeg) {
		work++
		if a.net == b.net || !bboxNear(a.s, b.s, floor) {
			return // too far apart to be worth measuring, so this pair is not a subject
		}
		k := pairKey{a.net, b.net}
		if k.a > k.b {
			k.a, k.b = k.b, k.a // the pair is symmetric, so its name is canonical
		}
		w := pairs[k]
		if w == nil {
			w = &worst{gap: floor}
			pairs[k] = w
			order = append(order, k)
		}
		gap := segDistNm(a.s.A, a.s.B, b.s.A, b.s.B) - (a.s.Width+b.s.Width)/2
		tolerance := int64(clearanceEpsilonNm)
		if a.edge || b.edge {
			tolerance = pourEdgeToleranceNm
		}
		if gap >= floor-tolerance {
			w.near++
			return
		}
		w.count++
		// The worst gap, and among equal gaps the pair that comes first in board order, so the place a
		// finding names does not depend on the order the sweep met the pairs in.
		if w.at == nil || gap < w.gap || (gap == w.gap && (a.idx < w.ai || (a.idx == w.ai && b.idx < w.bi))) {
			w.gap, w.at, w.ai, w.bi = gap, a.s.A, a.idx, b.idx
		}
	}
	// A sweep per layer (agni issue 963). Comparing every segment with every other was O(S^2),
	// about 197 million pairs and a second natively on the Jetson baseboard's 19,835 segments,
	// nearly all rejected by the bounding box. With segments sorted by left edge, a segment only
	// meets the ones that start before its right edge plus the farthest reach bboxNear allows. So
	// the sweep measures the same set of pairs as the full walk.
	byLayer := map[string][]*flatSeg{}
	widest := map[string]int64{}
	longest := map[string]int64{} // the widest x extent of a segment on the layer, for the scoped window
	for i := range segs[:tracks] {
		sg := &segs[i]
		byLayer[sg.s.Layer] = append(byLayer[sg.s.Layer], sg)
		widest[sg.s.Layer] = max64(widest[sg.s.Layer], sg.s.Width)
		longest[sg.s.Layer] = max64(longest[sg.s.Layer], sg.maxX-sg.minX)
	}
	// Asked about some nets (agni issue 1070), only the pairs with a segment of one of them are
	// measured. Each such segment looks up the segments whose left edge lies within bboxNear's
	// reach of it. A pair of two scoped segments is measured once, from the one earlier in board
	// order, so every count and tie-break matches the full sweep's.
	scope := check.SubjectScope(ctx)
	if scope != nil {
		inScope := map[string]bool{}
		for i := range segs[:tracks] {
			if n := segs[i].net; !inScope[n] && scope.Covers(check.NetNameEntity(n)) {
				inScope[n] = true
			}
		}
		for layer, ls := range byLayer {
			// A layer holding none of the scoped nets' copper has no pair to measure, and sorting it is
			// most of the cost left.
			if !slices.ContainsFunc(ls, func(sg *flatSeg) bool { return inScope[sg.net] }) {
				continue
			}
			sort.Slice(ls, func(i, j int) bool { return ls[i].minX < ls[j].minX })
			for _, f := range ls {
				if !inScope[f.net] {
					continue
				}
				reach := floor + (f.s.Width+widest[layer])/2 + 1
				from := sort.Search(len(ls), func(i int) bool { return ls[i].minX >= f.minX-reach-longest[layer] })
				for _, b := range ls[from:] {
					if b.minX > f.maxX+reach {
						break
					}
					if b == f || (inScope[b.net] && b.idx < f.idx) {
						continue
					}
					if f.idx < b.idx {
						measure(f, b)
					} else {
						measure(b, f)
					}
				}
			}
		}
	} else {
		for layer, ls := range byLayer {
			sort.Slice(ls, func(i, j int) bool { return ls[i].minX < ls[j].minX })
			for p, a := range ls {
				// bboxNear pads by floor + (a.Width+b.Width)/2, which never exceeds this; the extra
				// nanometre covers the integer halving.
				reach := a.maxX + floor + (a.s.Width+widest[layer])/2 + 1
				for _, b := range ls[p+1:] {
					if b.minX > reach {
						break
					}
					if a.idx < b.idx {
						measure(a, b)
					} else {
						measure(b, a)
					}
				}
			}
		}
	}
	// The pour edges sit in a grid per layer (agni 1410). A track looks only at the cells within its
	// reach, where the sweep would walk past a plane's own edges by the thousand. A pour looks at
	// another net's edges only where the two pours' boxes come within reach. Asked about some nets,
	// only the pairs one of them is in are measured, as the sweep does.
	if len(segs) > tracks {
		// Nets and layers by number, so the grid's keys are integers.
		netNo, layerNo := map[string]int32{}, map[string]int32{}
		number := func(m map[string]int32, k string) int32 {
			n, ok := m[k]
			if !ok {
				n = int32(len(m)) // #nosec G115 -- a board has far fewer nets or layers than 2^31
				m[k] = n
			}
			return n
		}
		netOf := make([]int32, len(segs))
		for i := range segs {
			netOf[i] = number(netNo, segs[i].net)
		}
		cellOf := func(v int64) int64 {
			if v < 0 {
				return (v - pourGridNm + 1) / pourGridNm
			}
			return v / pourGridNm
		}
		type cellKey struct {
			layer int32
			x, y  int64
		}
		// The grid is one array of (cell, net, edge) entries, sorted, with each cell's span found
		// through one map. Within a cell the entries run net by net, and runEnd says where an entry's
		// net run ends, so a query steps over its own net's edges, most of a plane's, in one step.
		type entry struct {
			cell cellKey
			net  int32
			seg  int32
		}
		var entries []entry
		type rowKey struct {
			shape int
			row   int64
		}
		rows := map[rowKey][]int32{} // each pour's edges per grid row, for the containment ray
		for i := tracks; i < len(segs); i++ {
			e := &segs[i]
			ly := number(layerNo, e.s.Layer)
			for x := cellOf(e.minX); x <= cellOf(e.maxX); x++ {
				for y := cellOf(e.minY); y <= cellOf(e.maxY); y++ {
					entries = append(entries, entry{cellKey{ly, x, y}, netOf[i], int32(i)}) // #nosec G115 -- a board has far fewer than 2^31 segments
				}
			}
			for y := cellOf(e.minY); y <= cellOf(e.maxY); y++ {
				rows[rowKey{e.shape, y}] = append(rows[rowKey{e.shape, y}], int32(i)) // #nosec G115 -- as above
			}
		}
		slices.SortFunc(entries, func(a, b entry) int {
			if c := cmp.Compare(a.cell.layer, b.cell.layer); c != 0 {
				return c
			}
			if c := cmp.Compare(a.cell.x, b.cell.x); c != 0 {
				return c
			}
			if c := cmp.Compare(a.cell.y, b.cell.y); c != 0 {
				return c
			}
			if c := cmp.Compare(a.net, b.net); c != 0 {
				return c
			}
			return cmp.Compare(a.seg, b.seg)
		})
		span := map[cellKey][2]int{}
		runEnd := make([]int, len(entries))
		for k := 0; k < len(entries); {
			c := entries[k].cell
			start := k
			for k < len(entries) && entries[k].cell == c {
				run := k
				for k < len(entries) && entries[k].cell == c && entries[k].net == entries[run].net {
					k++
				}
				for r := run; r < k; r++ {
					runEnd[r] = k
				}
			}
			span[c] = [2]int{start, k}
		}
		wanted := func(a, b string) bool {
			return scope == nil || scope.Covers(check.NetNameEntity(a)) || scope.Covers(check.NetNameEntity(b))
		}
		seen := make([]int32, len(segs)) // the last query that met each edge, so an edge in two cells is measured once
		var query int32
		near := func(f *flatSeg, skip func(e *flatSeg) bool) {
			query++
			fn := netOf[f.idx]
			ly, ok := layerNo[f.s.Layer]
			if !ok {
				return // no pour on this layer
			}
			reach := floor + f.s.Width/2 + 1
			for x := cellOf(f.minX - reach); x <= cellOf(f.maxX+reach); x++ {
				for y := cellOf(f.minY - reach); y <= cellOf(f.maxY+reach); y++ {
					sp, ok := span[cellKey{ly, x, y}]
					if !ok {
						continue
					}
					for k := sp[0]; k < sp[1]; {
						en := entries[k]
						if en.net == fn || !wanted(f.net, segs[en.seg].net) {
							k = runEnd[k]
							continue
						}
						k++
						work++
						if seen[en.seg] == query {
							continue
						}
						seen[en.seg] = query
						e := &segs[en.seg]
						if skip(e) {
							continue
						}
						if f.idx < e.idx {
							measure(f, e)
						} else {
							measure(e, f)
						}
					}
				}
			}
		}
		for i := 0; i < tracks; i++ {
			near(&segs[i], func(*flatSeg) bool { return false })
		}
		// Pour against pour: each edge of one pour against the edges of another net's pours, the pair
		// measured from the edge earlier in board order.
		for i := tracks; i < len(segs); i++ {
			f := &segs[i]
			near(f, func(e *flatSeg) bool { return e.idx < f.idx })
		}
		// Copper that lies wholly inside another net's pour crosses none of its edges, so no query
		// above meets it, and it is a short all the same. A point is inside a pour's copper when a ray
		// from it to the right crosses the pour's edges, outline and holes together, an odd number of
		// times. The pour's edges in the ray's row of the grid are every edge it can cross. A track is
		// tested by its midpoint and a pour by its first point.
		inside := func(px, py int64, shape int) bool {
			crossings := 0
			for _, i := range rows[rowKey{shape, cellOf(py)}] {
				e := &segs[i]
				work++
				if e.maxX > px && rayCrosses(e.s.A, e.s.B, px, py) {
					crossings++
				}
			}
			return crossings%2 == 1
		}
		poursOn := map[string][]int{}
		for shape, pb := range pours {
			poursOn[pb.layer] = append(poursOn[pb.layer], shape)
		}
		test := func(net, layer string, x, y, gap int64, at *geom.Point) {
			for _, shape := range poursOn[layer] {
				pb := pours[shape]
				if pb.net == net || x < pb.minX || x > pb.maxX || y < pb.minY || y > pb.maxY || !wanted(net, pb.net) {
					continue
				}
				if inside(x, y, shape) {
					containedIn = append(containedIn, overlap{a: net, b: pb.net, at: at, gap: gap})
				}
			}
		}
		for i := 0; i < tracks; i++ {
			s := segs[i].s
			mx, my := (s.A.X+s.B.X)/2, (s.A.Y+s.B.Y)/2
			test(segs[i].net, s.Layer, mx, my, -s.Width/2, &geom.Point{X: mx, Y: my})
		}
		for _, pb := range pours {
			test(pb.net, pb.layer, pb.first.X, pb.first.Y, 0, pb.first)
		}
	}
	// Copper that lies wholly inside another net's pour crosses none of its edges, so the sweep never
	// meets it, and it is a short all the same. Each track, and each pour by its first point, is
	// tested against the other nets' pours on its layer.
	for _, inside := range containedIn {
		k := pairKey{inside.a, inside.b}
		if k.a > k.b {
			k.a, k.b = k.b, k.a
		}
		w := pairs[k]
		if w == nil {
			w = &worst{gap: floor}
			pairs[k] = w
			order = append(order, k)
		}
		w.count++
		if w.at == nil || inside.gap < w.gap {
			w.gap, w.at, w.ai, w.bi = inside.gap, inside.at, -1, -1
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
		// NAME-ONLY entities. Board copper joins the netlist by name (CONSTRAINTS C21), so this rule
		// holds its nets by name and has no per-instance id to carry. Resolving one by name lookup would
		// pick arbitrarily between two same-named nets and state an instance the walk never distinguished.
		subjects := []check.Entity{check.NetNameEntity(k.a), check.NetNameEntity(k.b)}
		v := check.Verdict{Subjects: subjects}
		if w.count == 0 {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("copper of %q and %q comes close enough to measure in %d place(s) on a shared layer and never within the %s",
					k.a, k.b, w.near, floorDesc),
				Terms: []check.WitnessTerm{{Label: "places measured", Value: strconv.Itoa(w.near)}},
			}
			out = append(out, v)
			continue
		}
		msg := fmt.Sprintf("copper of %q and %q closer than the %s at %d place(s); worst gap %.3fmm near (%.2f, %.2f)mm",
			k.a, k.b, floorDesc, w.count, float64(w.gap)/1e6, float64(w.at.X)/1e6, float64(w.at.Y)/1e6)
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
			// told one place to go and look, and this context gives the other end its way back into
			// the drawing (agni issue 349). The verdict names both.
			Context: []check.ContextSubject{check.Ctx(subjects[1], "neighbour")},
		}
		out = append(out, v)
	}
	return out, work
}

// bboxNear is the cheap reject. It reports whether two segments' bounding boxes, inflated
// by the clearance plus both half-widths, overlap. Everything that could violate passes this.
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

// segDistNm is the minimum centerline distance between two 2D segments. It is zero when they
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
	on := func(p, q, r *geom.Point) bool { // for r collinear with pq, whether r sits on pq
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

// overlap is copper of net a found inside a pour of net b, with the gap reported for it: the track's
// half width below zero, since the two overlap, or zero for a pour.
type overlap struct {
	a, b string
	at   *geom.Point
	gap  int64
}

// pourGridNm is the side of a pour grid cell. A cell this size holds a few hundred of a plane's
// edges, and a track's reach covers a handful of cells.
const pourGridNm = 1_000_000

// rayCrosses reports whether the ray from (px, py) to the right crosses the edge ab. An edge counts
// when it straddles the ray's height, its lower end included and its upper end not, so a ray through
// a vertex counts it once.
func rayCrosses(a, b *geom.Point, px, py int64) bool {
	if (a.Y > py) == (b.Y > py) {
		return false
	}
	return float64(px) < float64(b.X-a.X)*(float64(py)-float64(a.Y))/float64(b.Y-a.Y)+float64(a.X)
}
