package relations

import (
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// hopFixture: SIG_A --R1--> SIG_B, plus a SECOND resistor R2 bridging the same pair, a capacitor C1
// that must not be a hop, a three-net part RN1 that is not a series element, and R9 with both pins on
// one net, which is a short.
func hopFixture() *ir.Design {
	net := func(name string, conns ...[2]string) *ir.Net {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c[0], PinRef: c[1]})
		}
		return n
	}
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	return &ir.Design{
		Components: []*ir.Component{comp("R1"), comp("R2"), comp("C1"), comp("RN1"), comp("R9"), comp("L4"), comp("FB3"), comp("F2")},
		Nets: []*ir.Net{
			net("SIG_A", [2]string{"R1", "1"}, [2]string{"R2", "1"}, [2]string{"C1", "1"}, [2]string{"RN1", "1"},
				[2]string{"R9", "1"}, [2]string{"R9", "2"}, [2]string{"L4", "1"}, [2]string{"FB3", "1"}, [2]string{"F2", "1"}),
			net("SIG_B", [2]string{"R1", "2"}, [2]string{"R2", "2"}, [2]string{"RN1", "2"}),
			net("GND", [2]string{"C1", "2"}),
			net("RN_C", [2]string{"RN1", "3"}),
			net("L_END", [2]string{"L4", "2"}),
			net("FB_END", [2]string{"FB3", "2"}),
			net("F_END", [2]string{"F2", "2"}),
		},
	}
}

func hops(t *testing.T, d *ir.Design) []facts.Row {
	t.Helper()
	var out []facts.Row
	for _, r := range netHopFacts(check.NewModel(d)) {
		if r.Relation == RelNetHop {
			out = append(out, r)
		}
	}
	return out
}

// THE POINT OF THE RELATION: two parts bridging one pair of nets are two hops, where `reaches`
// reports one destination. Counting them is what tells a double pull-up from a correct board.
func TestTwoPartsBridgingOnePairAreTwoHops(t *testing.T) {
	var through []string
	for _, r := range hops(t, hopFixture()) {
		if r.Subject == "SIG_A" && r.Value == "SIG_B" {
			through = append(through, r.Object)
		}
	}
	sort.Strings(through)
	if len(through) != 2 || through[0] != "R1" || through[1] != "R2" {
		t.Errorf("SIG_A to SIG_B goes through %v, want both R1 and R2", through)
	}
}

func TestEveryHopIsEmittedBothWays(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range hops(t, hopFixture()) {
		seen[r.Subject+"|"+r.Object+"|"+r.Value] = true
	}
	if !seen["SIG_A|R1|SIG_B"] || !seen["SIG_B|R1|SIG_A"] {
		t.Errorf("a crossing is not queryable from both ends: %v", seen)
	}
}

// The class rule has to agree with the reach walk's, and it is a separate copy, so this is what
// holds the two together.
func TestEverySeriesClassCrossesAndNothingElseDoes(t *testing.T) {
	crossed := map[string]bool{}
	for _, r := range hops(t, hopFixture()) {
		crossed[r.Object] = true
	}
	for _, ref := range []string{"R1", "L4", "FB3", "F2"} {
		if !crossed[ref] {
			t.Errorf("%s is a series pass element and produced no hop", ref)
		}
	}
	// A capacitor is a DC block, a three-net part is not a series element, and a part with both
	// pins on one net is a short. None of them is a crossing.
	for _, ref := range []string{"C1", "RN1", "R9"} {
		if crossed[ref] {
			t.Errorf("%s produced a hop and is not a series crossing", ref)
		}
	}
}

// Ground is reachable like anything else, because excluding it would put one rule's question into a
// fact everybody reads. A rule that must not cross it says so itself.
func TestGroundIsNotExcluded(t *testing.T) {
	d := hopFixture()
	d.Nets = append(d.Nets, &ir.Net{
		Name: "GND2", Prov: &ir.Provenance{SourceFile: "t"},
		Connections: []*ir.Connection{{ComponentRef: "L4", PinRef: "2"}},
	})
	// L4 now touches SIG_A, L_END and GND2, which makes it a three-net part rather than a crossing,
	// so use a fresh two-net part to ground instead.
	d.Components = append(d.Components, &ir.Component{RefDes: "R7", Prov: &ir.Provenance{SourceFile: "t"}})
	d.Nets[0].Connections = append(d.Nets[0].Connections, &ir.Connection{ComponentRef: "R7", PinRef: "1"})
	d.Nets[2].Connections = append(d.Nets[2].Connections, &ir.Connection{ComponentRef: "R7", PinRef: "2"})
	found := false
	for _, r := range hops(t, d) {
		if r.Object == "R7" && r.Value == "GND" {
			found = true
		}
	}
	if !found {
		t.Error("a crossing onto ground was dropped; that is a rule's question, not a fact's")
	}
}
