package builtin

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/kicad"
)

// allPairsClearance is copper-clearance as it was before the sweep (agni issue 963): every segment
// against every other, in board order. It is the reference the sweep must agree with exactly. A
// pour's edges are segments too, after the tracks, and copper inside another net's pour is found by
// testing every pour (agni 1410).
func allPairsClearance(m check.Model) []check.Verdict {
	floor, floorDesc := floorFor(m, "clearance")
	type flatSeg struct {
		net  string
		s    check.BoardSeg
		edge bool
	}
	var segs []flatSeg
	for _, bn := range m.BoardNets() {
		for _, s := range bn.Segments {
			segs = append(segs, flatSeg{net: bn.Net, s: s})
		}
	}
	type pourRef struct {
		net string
		sh  check.BoardShape
	}
	var pours []pourRef
	for _, bn := range m.BoardNets() {
		for _, sh := range bn.Shapes {
			if len(sh.Outline) < 3 {
				continue
			}
			pours = append(pours, pourRef{bn.Net, sh})
			for _, ring := range append([][]*geom.Point{sh.Outline}, sh.Holes...) {
				for i := range ring {
					a, b := ring[i], ring[(i+1)%len(ring)]
					if a.X != b.X || a.Y != b.Y {
						segs = append(segs, flatSeg{net: bn.Net, s: check.BoardSeg{Layer: sh.Layer, A: a, B: b}, edge: true})
					}
				}
			}
		}
	}
	type pairKey struct{ a, b string }
	type worst struct {
		gap         int64
		at          *geom.Point
		count, near int
	}
	pairs := map[pairKey]*worst{}
	var order []pairKey
	for i := range segs {
		for j := i + 1; j < len(segs); j++ {
			a, b := segs[i], segs[j]
			if a.net == b.net || a.s.Layer != b.s.Layer || !bboxNear(a.s, b.s, floor) {
				continue
			}
			k := pairKey{a.net, b.net}
			if k.a > k.b {
				k.a, k.b = k.b, k.a
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
				continue
			}
			w.count++
			if w.at == nil || gap < w.gap {
				w.gap, w.at = gap, a.s.A
			}
		}
	}
	inPour := func(p pourRef, x, y int64) bool {
		n := 0
		for _, ring := range append([][]*geom.Point{p.sh.Outline}, p.sh.Holes...) {
			for i := range ring {
				if a, b := ring[i], ring[(i+1)%len(ring)]; (a.X != b.X || a.Y != b.Y) && rayCrosses(a, b, x, y) {
					n++
				}
			}
		}
		return n%2 == 1
	}
	contained := func(net, layer string, x, y, gap int64, at *geom.Point) {
		for _, p := range pours {
			if p.net == net || p.sh.Layer != layer || !inPour(p, x, y) {
				continue
			}
			k := pairKey{net, p.net}
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
			if w.at == nil || gap < w.gap {
				w.gap, w.at = gap, at
			}
		}
	}
	for _, f := range segs {
		if !f.edge {
			mx, my := (f.s.A.X+f.s.B.X)/2, (f.s.A.Y+f.s.B.Y)/2
			contained(f.net, f.s.Layer, mx, my, -f.s.Width/2, &geom.Point{X: mx, Y: my})
		}
	}
	for _, p := range pours {
		contained(p.net, p.sh.Layer, p.sh.Outline[0].X, p.sh.Outline[0].Y, 0, p.sh.Outline[0])
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].a != order[j].a {
			return order[i].a < order[j].a
		}
		return order[i].b < order[j].b
	})
	out := make([]check.Verdict, 0, len(order))
	for _, k := range order {
		w := pairs[k]
		subjects := []check.Entity{check.NetNameEntity(k.a), check.NetNameEntity(k.b)}
		v := check.Verdict{Subjects: subjects}
		if w.count == 0 {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("copper of %q and %q comes close enough to measure in %d place(s) on a shared layer and never within the %s", k.a, k.b, w.near, floorDesc),
				Terms:     []check.WitnessTerm{{Label: "places measured", Value: strconv.Itoa(w.near)}},
			}
			out = append(out, v)
			continue
		}
		msg := fmt.Sprintf("copper of %q and %q closer than the %s at %d place(s); worst gap %.3fmm near (%.2f, %.2f)mm",
			k.a, k.b, floorDesc, w.count, float64(w.gap)/1e6, float64(w.at.X)/1e6, float64(w.at.Y)/1e6)
		v.Outcome = check.Fail
		v.Witness = &check.Witness{Statement: msg, Terms: []check.WitnessTerm{
			{Label: "places under the floor", Value: strconv.Itoa(w.count)},
			{Label: "worst gap", Value: fmt.Sprintf("%.3fmm", float64(w.gap)/1e6)},
		}}
		v.Finding = &check.Finding{Subject: subjects[0], Message: msg, Context: []check.ContextSubject{check.Ctx(subjects[1], "neighbour")}}
		out = append(out, v)
	}
	return out
}

// denseBoard is a random board crowded enough that many pairs fall under the floor, with odd widths
// (so the half-width rounding matters), repeated geometry (so worst gaps tie), several layers, and
// segments of one net beside each other.
func denseBoard(seed int64, n int) *geom.BoardGeometry {
	r := rand.New(rand.NewSource(seed))
	g := &geom.BoardGeometry{UnitNm: 1}
	nets := map[string]*geom.NetCopper{}
	layers := []string{"F.Cu", "In1.Cu", "B.Cu"}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("N%d", r.Intn(12))
		nc := nets[name]
		if nc == nil {
			nc = &geom.NetCopper{Net: name}
			nets[name] = nc
			g.Nets = append(g.Nets, nc)
		}
		x, y := int64(r.Intn(40))*150_000, int64(r.Intn(40))*150_000
		dx, dy := int64(r.Intn(7)-3)*100_000, int64(r.Intn(7)-3)*100_000
		nc.Segments = append(nc.Segments, &geom.TrackSegment{
			A: &geom.Point{X: x, Y: y}, B: &geom.Point{X: x + dx, Y: y + dy},
			Width: 100_001 + int64(r.Intn(5))*50_000, Layer: layers[r.Intn(len(layers))],
		})
	}
	return g
}

// pouredBoard is denseBoard with pours: a rectangle per net on some layers, with a few rectangular
// holes, some of them around another net's tracks and some overlapping other pours (agni 1410).
func pouredBoard(seed int64, n int) *geom.BoardGeometry {
	g := denseBoard(seed, n)
	r := rand.New(rand.NewSource(seed + 1000))
	layers := []string{"F.Cu", "In1.Cu", "B.Cu"}
	rect := func(x0, y0, x1, y1 int64) *geom.Polyline {
		return &geom.Polyline{Points: []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}
	}
	for _, nc := range g.Nets {
		if r.Intn(3) != 0 {
			continue
		}
		x, y := int64(r.Intn(30))*150_000, int64(r.Intn(30))*150_000
		w, h := int64(r.Intn(10)+3)*150_000, int64(r.Intn(10)+3)*150_000
		sh := &geom.CopperShape{Layer: layers[r.Intn(len(layers))], Outline: rect(x, y, x+w, y+h)}
		for k := 0; k < r.Intn(4); k++ {
			hx, hy := x+int64(r.Intn(int(w/150_000)))*150_000+30_000, y+int64(r.Intn(int(h/150_000)))*150_000+30_000
			sh.Holes = append(sh.Holes, rect(hx, hy, hx+90_000, hy+90_000))
		}
		nc.Shapes = append(nc.Shapes, sh)
	}
	return g
}

// TestTheSweepMeasuresWhatEveryPairMeasured holds the sweep to the all-pairs walk, verdict for
// verdict and word for word, on random crowded boards and on the Jetson baseboard.
func TestTheSweepMeasuresWhatEveryPairMeasured(t *testing.T) {
	boards := map[string]*geom.BoardGeometry{}
	for seed := int64(1); seed <= 25; seed++ {
		boards[fmt.Sprintf("dense-%d", seed)] = denseBoard(seed, 400)
		boards[fmt.Sprintf("poured-%d", seed)] = pouredBoard(seed, 400)
	}
	const jetson = "../../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb"
	f, err := os.Open(jetson)
	if err != nil {
		t.Fatalf("%v (make samples-oracle fetches the boards)", err)
	}
	g, err := kicad.ReadBoardGeometry(f, jetson)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	boards["jetson"] = g
	fails, passes := 0, 0
	for name, g := range boards {
		m := check.NewModel(&ir.Design{}, check.WithBoard(g))
		want := allPairsClearance(m)
		got := copperClearanceVerdicts(context.Background(), m)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the sweep gives %d verdicts and the all-pairs walk %d, or their words differ", name, len(got), len(want))
			continue
		}
		for _, v := range want {
			if v.Outcome == check.Fail {
				fails++
			} else {
				passes++
			}
		}
	}
	// The positive control. The boards hold both failing and passing pairs, so agreeing covers each.
	if fails < 100 || passes < 100 {
		t.Fatalf("the boards compared gave %d failing and %d passing pairs, too few to prove the sweep", fails, passes)
	}
	// And the pours count. Nearly every pair of nets on these crowded boards fails on its tracks
	// alone, so the control counts the places under the floor, which the pours' edges and insides add
	// to, rather than the failing pairs.
	more := 0
	for seed := int64(1); seed <= 25; seed++ {
		count := func(g *geom.BoardGeometry) int {
			n := 0
			for _, v := range copperClearanceVerdicts(context.Background(), check.NewModel(&ir.Design{}, check.WithBoard(g))) {
				for _, term := range v.Witness.Terms {
					if term.Label == "places under the floor" {
						k, _ := strconv.Atoi(term.Value)
						n += k
					}
				}
			}
			return n
		}
		more += count(pouredBoard(seed, 400)) - count(denseBoard(seed, 400))
	}
	if more < 500 {
		t.Fatalf("the pours added %d places under the floor across the boards, too few to prove the pour edges are measured", more)
	}
}

// agni issue 1070: asked about some nets, the rule measures only their segments' neighbours, and its
// answer must be the full sweep's verdicts about those nets, count for count. The scopes here hold
// one net, and two or three nets that are each other's neighbours. In that second case a pair could
// be measured from both ends.
func TestAScopedSweepGivesTheFullSweepsVerdictsAboutItsNets(t *testing.T) {
	asked, both := 0, 0
	for seed := int64(1); seed <= 50; seed++ {
		board := denseBoard(seed, 400)
		if seed > 25 {
			board = pouredBoard(seed, 400) // agni 1410: a pour's net in scope measures its edges and its insides
		}
		m := check.NewModel(&ir.Design{}, check.WithBoard(board))
		full := copperClearanceVerdicts(context.Background(), m)
		for _, nets := range [][]string{{"N1"}, {"N2", "N3"}, {"N4", "N5", "N6"}, {"N0", "N11"}} {
			es := make([]check.Entity, len(nets))
			for i, n := range nets {
				es[i] = check.NetNameEntity(n)
			}
			scope := check.NewScope(es...)
			_, want := scope.Results(nil, full)
			got := copperClearanceVerdicts(check.WithSubjectScope(context.Background(), scope), m)
			if !(len(got) == 0 && len(want) == 0) && !reflect.DeepEqual(got, want) {
				t.Errorf("seed %d, asked about %v: the scoped sweep gives %d verdicts and the full sweep filtered %d, or their counts differ", seed, nets, len(got), len(want))
			}
			asked++
			for _, v := range want {
				if scope.Covers(v.Subjects[0]) && scope.Covers(v.Subjects[1]) {
					both++
				}
			}
		}
	}
	// The positive control. Some answers are pairs whose two nets were both asked about, and a pair
	// measured from both ends would be double counted there.
	if asked < 100 || both < 10 {
		t.Fatalf("%d scoped questions, %d pairs inside a scope, too few to prove anything", asked, both)
	}
}
