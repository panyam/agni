package query

import (
	"fmt"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// walkChain is a design shaped to make an unbound walk expensive: n nets in a chain, each pair joined
// by a resistor so every net reaches every other, two IO nets at one end that the query is really
// about, and a TVS on the far end.
func walkChain(n int) *ir.Design {
	p := &ir.Provenance{SourceFile: "chain"}
	d := &ir.Design{}
	net := func(name string) *ir.Net {
		for _, x := range d.Nets {
			if x.Name == name {
				return x
			}
		}
		x := &ir.Net{Name: name, Prov: p}
		d.Nets = append(d.Nets, x)
		return x
	}
	join := func(ref, a, b string) {
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Prov: p})
		net(a).Connections = append(net(a).Connections, &ir.Connection{ComponentRef: ref, PinRef: "1"})
		net(b).Connections = append(net(b).Connections, &ir.Connection{ComponentRef: ref, PinRef: "2"})
	}
	for i := 0; i+1 < n; i++ {
		join(fmt.Sprintf("R%d", i), fmt.Sprintf("N%d", i), fmt.Sprintf("N%d", i+1))
	}
	join("R100", "IO1", "N0")
	join("R101", "IO2", "N0")
	d.Components = append(d.Components, &ir.Component{RefDes: "TVS1", Prov: p})
	last := net(fmt.Sprintf("N%d", n-1))
	last.Connections = append(last.Connections, &ir.Connection{ComponentRef: "TVS1", PinRef: "1"})
	return d
}

// TestTheDefaultEvaluatorPlansAnUnboundWalk is the WS3-114 regression guard, now that the engine plans
// rule bodies rather than an engine lint refusing one shape. A shipped rule opened with an unbound
// walk, `esd_ok(?n) :- net.reaches(?n, ...), needs_esd(?n), ...`, which walked from every net on the
// board before its guard applied and took `agni check` from 13s to not finishing. Under query.Default
// the written order no longer decides where the walk starts, so the walk-first body must cost about
// what the guard-first body costs.
//
// Naive runs bodies as written, and is the positive control: on the same design the walk-first body
// must cost far more there, or the work measure could not see the hazard at all. net.route walks the
// same way and is held to the same property.
func TestTheDefaultEvaluatorPlansAnUnboundWalk(t *testing.T) {
	m := check.NewModel(walkChain(60))
	const io = `io(?n) :- entity(?n, "net"), str.prefix(?n, "IO"); `
	for _, walk := range []string{"net.reaches(?n, ?rn, ?h)", "net.route(?n, ?rn, ?h)"} {
		walkFirst := io + `esd_ok(?n) :- ` + walk + `, io(?n), component.net(?t, ?rn), component.class(?t, "tvs"); esd_ok(?n) => ?n`
		guardFirst := io + `esd_ok(?n) :- io(?n), ` + walk + `, component.net(?t, ?rn), component.class(?t, "tvs"); esd_ok(?n) => ?n`
		cost := func(ev Evaluator, text string) (int64, int) {
			b := NewBase(m)
			rows, err := ev.Eval(MustParse(text), b)
			if err != nil {
				t.Fatalf("%s: %v", text, err)
			}
			return b.Work(), len(rows)
		}
		guarded, n := cost(Default, guardFirst)
		if n != 2 {
			t.Fatalf("%s: the guarded rule answered %d rows, want IO1 and IO2", walk, n)
		}
		planned, n := cost(Default, walkFirst)
		if n != 2 {
			t.Fatalf("%s: the walk-first rule answered %d rows under Default, want 2", walk, n)
		}
		if planned > 2*guarded {
			t.Errorf("%s: walk-first cost %d under Default against %d guard-first; the planner should start the walk from the guard", walk, planned, guarded)
		}
		naiveGuarded, _ := cost(Naive{}, guardFirst)
		naiveWalkFirst, _ := cost(Naive{}, walkFirst)
		t.Logf("%s: Default guard-first %d, walk-first %d; Naive guard-first %d, walk-first %d", walk, guarded, planned, naiveGuarded, naiveWalkFirst)
		if naiveWalkFirst < 10*naiveGuarded {
			t.Errorf("%s: walk-first cost %d under Naive against %d guard-first; the control should show the hazard, or this test cannot see it", walk, naiveWalkFirst, naiveGuarded)
		}
	}
}
