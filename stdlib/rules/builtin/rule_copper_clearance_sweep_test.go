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
// against every other, in board order. It is the reference the sweep must agree with exactly.
func allPairsClearance(m check.Model) []check.Verdict {
	floor, floorDesc := floorFor(m, "clearance")
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
			if gap >= floor-clearanceEpsilonNm {
				w.near++
				continue
			}
			w.count++
			if w.at == nil || gap < w.gap {
				w.gap, w.at = gap, a.s.A
			}
		}
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

// TestTheSweepMeasuresWhatEveryPairMeasured holds the sweep to the all-pairs walk, verdict for
// verdict and word for word, on random crowded boards and on the Jetson baseboard.
func TestTheSweepMeasuresWhatEveryPairMeasured(t *testing.T) {
	boards := map[string]*geom.BoardGeometry{}
	for seed := int64(1); seed <= 25; seed++ {
		boards[fmt.Sprintf("dense-%d", seed)] = denseBoard(seed, 400)
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
	// Positive control: the boards hold failing and passing pairs both, so agreeing covers each.
	if fails < 100 || passes < 100 {
		t.Fatalf("the boards compared gave %d failing and %d passing pairs, too few to prove the sweep", fails, passes)
	}
}
