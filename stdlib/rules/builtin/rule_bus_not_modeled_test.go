package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestBusNotModeledResolution: the rule fires only when a bus's members are NOT all present as nets.
// A bus whose every member is a net (the flat-sheet case, members formed by the tap labels) is silent;
// a bus with a missing member, or with no known member set, fires.
func TestBusNotModeledResolution(t *testing.T) {
	design := func(buses []*ir.BusNotModeled, nets ...string) *ir.Design {
		d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{UnmodeledBuses: buses}}
		for _, n := range nets {
			d.Nets = append(d.Nets, &ir.Net{Name: n})
		}
		return d
	}
	bus := func(label string, members ...string) *ir.BusNotModeled {
		return &ir.BusNotModeled{Kind: "bus", Label: label, Members: members}
	}

	// Resolved: both members are nets -> silent.
	if fs := busNotModeled.Findings(check.NewModel(design([]*ir.BusNotModeled{bus("DATA[1:0]", "DATA0", "DATA1")}, "DATA0", "DATA1"))); len(fs) != 0 {
		t.Errorf("resolved bus should be silent, got %d findings", len(fs))
	}
	// One member missing -> fires, named after the bus.
	fs := busNotModeled.Findings(check.NewModel(design([]*ir.BusNotModeled{bus("DATA[1:0]", "DATA0", "DATA1")}, "DATA0")))
	if len(fs) != 1 || fs[0].Subject != "DATA[1:0]" {
		t.Errorf("bus with a missing member should fire on DATA[1:0], got %v", fs)
	}
	// No known member set -> cannot confirm resolution -> fires.
	if fs := busNotModeled.Findings(check.NewModel(design([]*ir.BusNotModeled{{Kind: "geda_bus", Label: "B"}}))); len(fs) != 1 {
		t.Errorf("bus with no member info should fire, got %d findings", len(fs))
	}
}

// TestBusNotModeledStatesTheResolvedBus is the half the findings contract cannot carry. A resolved
// bus is silent to `check` and always was, so a design whose buses are all properly modelled and a
// design with no bus in it report the same nothing. The verdict is where the difference now lives,
// and the witness has to rest on the member count rather than merely asserting resolution: drop a
// member's net and the same bus comes back a failure naming it.
func TestBusNotModeledStatesTheResolvedBus(t *testing.T) {
	design := func(buses []*ir.BusNotModeled, nets ...string) *ir.Design {
		d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{UnmodeledBuses: buses}}
		for _, n := range nets {
			d.Nets = append(d.Nets, &ir.Net{Name: n})
		}
		return d
	}
	bus := &ir.BusNotModeled{Kind: "bus", Label: "DATA[1:0]", Members: []string{"DATA0", "DATA1"}}

	vs := busNotModeled.Eval(check.NewModel(design([]*ir.BusNotModeled{bus}, "DATA0", "DATA1")))
	if len(vs) != 1 || vs[0].Outcome != check.Pass {
		t.Fatalf("a bus whose members are all nets should PASS, got %+v", vs)
	}
	if vs[0].Kind != check.KindBus || vs[0].Subject != "DATA[1:0]" {
		t.Errorf("verdict should be about the bus, got kind %q subject %q", vs[0].Kind, vs[0].Subject)
	}
	if !strings.Contains(vs[0].Witness.Statement, "2 member") {
		t.Errorf("the pass must rest on the member count, not assert resolution: %q", vs[0].Witness.Statement)
	}

	// The red half of the same assertion: take one member's net away and the statement changes to
	// name it. A witness that read the same either way would prove nothing.
	vs = busNotModeled.Eval(check.NewModel(design([]*ir.BusNotModeled{bus}, "DATA0")))
	if len(vs) != 1 || vs[0].Outcome != check.Fail {
		t.Fatalf("a bus with a member that is not a net should FAIL, got %+v", vs)
	}
	if !strings.Contains(vs[0].Witness.Statement, `"DATA1"`) {
		t.Errorf("the failure must name the member with no net: %q", vs[0].Witness.Statement)
	}
}
