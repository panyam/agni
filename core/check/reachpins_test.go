package check

import (
	"testing"
)

// The tests here cover agni issue 1541 on traceFixture's topologies.

// reachedSet keys a Reachable's pins by "U7.3".
func reachedSet(r Reachable) map[string]ReachablePin {
	out := map[string]ReachablePin{}
	for _, p := range r.Pins {
		out[p.Endpoint.String()] = p
	}
	return out
}

// TestReachablePinsAgreeWithTrace holds the list to trace in both directions. Every listed pin routes
// at the same radius, and every pin that routes there and sits on no rail is listed. So a viewer that
// offers only these pins never offers a target trace refuses, and never hides one it would take.
func TestReachablePinsAgreeWithTrace(t *testing.T) {
	m := traceModel()
	starts := []Endpoint{{"U7", "3"}, {"U20", "1"}, {"U8", "1"}, {"U12", "4"}}
	checked := 0
	for _, from := range starts {
		for hops := 1; hops <= 4; hops++ {
			r := ReachablePins(m, from, hops)
			if r.Reason != "" {
				t.Fatalf("%s: %s", from, r.Reason)
			}
			listed := reachedSet(r)
			for _, p := range r.Pins {
				if tr := TracePins(m, from, p.Endpoint, hops); tr.Outcome != TraceRouted {
					t.Errorf("%s at %d lists %s, which trace answers %q (%s)", from, hops, p.Endpoint, tr.Outcome, tr.Reason)
				}
				checked++
			}
			rails := map[string]bool{}
			for _, rl := range r.Rails {
				rails[rl.Net] = true
			}
			for _, n := range m.Nets() {
				for _, c := range n.Connections {
					to := Endpoint{c.ComponentRef, c.PinRef}
					if to == from || rails[n.Name] || IsVirtualRef(to.RefDes) {
						continue
					}
					if TracePins(m, from, to, hops).Outcome == TraceRouted {
						if _, ok := listed[to.String()]; !ok {
							t.Errorf("%s at %d leaves out %s, which trace routes to", from, hops, to)
						}
					}
				}
			}
		}
	}
	if checked < 20 {
		t.Fatalf("checked %d listed pins against trace, want at least 20", checked)
	}
}

func TestReachablePinsGrowWithTheRadius(t *testing.T) {
	m := traceModel()
	from := Endpoint{"U20", "1"}
	if _, ok := reachedSet(ReachablePins(m, from, 2))["U21.1"]; ok {
		t.Errorf("at radius 2, U21.1 is listed, three crossings away")
	}
	p, ok := reachedSet(ReachablePins(m, from, 3))["U21.1"]
	if !ok {
		t.Fatal("at radius 3, U21.1 is not listed")
	}
	if p.Depth != 3 || p.Net != "CH3" {
		t.Errorf("U21.1 = depth %d on %q, want depth 3 on CH3", p.Depth, p.Net)
	}
	if r := ReachablePins(m, from, 3); r.Radius != 3 || r.From.Net != "CH0" {
		t.Errorf("radius %d from %q, want 3 from CH0", r.Radius, r.From.Net)
	}
}

// A rail a trace may end on is named with its pin count, not listed pin by pin, since a ground on a
// real board carries hundreds of pins.
func TestReachablePinsNameARailWithoutItsPins(t *testing.T) {
	r := ReachablePins(traceModel(), Endpoint{"U7", "3"}, DefaultTraceHops)
	if len(r.Rails) != 1 || r.Rails[0].Net != "VCC" || r.Rails[0].Depth != 2 || r.Rails[0].Pins != 2 {
		t.Fatalf("rails = %+v, want VCC at depth 2 with 2 pins", r.Rails)
	}
	listed := reachedSet(r)
	if _, ok := listed["U10.1"]; ok {
		t.Error("U10.1 on the VCC rail is listed pin by pin")
	}
	// The start net is depth 0, and the start pin is not its own target.
	if p, ok := listed["TP1.1"]; !ok || p.Depth != 0 {
		t.Errorf("TP1.1 = %+v (listed %v), want depth 0 on the start net", p, ok)
	}
	if _, ok := listed["U7.3"]; ok {
		t.Error("the start pin lists itself")
	}
	if p := listed["U12.4"]; p.PinName != "SDIO" || p.Depth != 1 {
		t.Errorf("U12.4 = %+v, want pin name SDIO at depth 1", p)
	}
}

func TestReachablePinsExplainAStartTheyCannotResolve(t *testing.T) {
	m := traceModel()
	for _, from := range []Endpoint{{"U99", "1"}, {"U7", ""}, {"U7", "9"}} {
		r := ReachablePins(m, from, DefaultTraceHops)
		if r.Reason == "" || len(r.Pins) != 0 || len(r.Rails) != 0 {
			t.Errorf("%s: reason %q with %d pins and %d rails, want a reason and nothing listed", from, r.Reason, len(r.Pins), len(r.Rails))
		}
	}
	if r := ReachablePins(m, Endpoint{"U7", "3"}, 0); r.Radius != DefaultTraceHops {
		t.Errorf("radius 0 searched %d, want the default %d, as trace does", r.Radius, DefaultTraceHops)
	}
}
