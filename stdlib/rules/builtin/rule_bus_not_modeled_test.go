package builtin

import (
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
	if fs := busNotModeled.Eval(check.NewModel(design([]*ir.BusNotModeled{bus("DATA[1:0]", "DATA0", "DATA1")}, "DATA0", "DATA1"))); len(fs) != 0 {
		t.Errorf("resolved bus should be silent, got %d findings", len(fs))
	}
	// One member missing -> fires, named after the bus.
	fs := busNotModeled.Eval(check.NewModel(design([]*ir.BusNotModeled{bus("DATA[1:0]", "DATA0", "DATA1")}, "DATA0")))
	if len(fs) != 1 || fs[0].Subject != "DATA[1:0]" {
		t.Errorf("bus with a missing member should fire on DATA[1:0], got %v", fs)
	}
	// No known member set -> cannot confirm resolution -> fires.
	if fs := busNotModeled.Eval(check.NewModel(design([]*ir.BusNotModeled{{Kind: "geda_bus", Label: "B"}}))); len(fs) != 1 {
		t.Errorf("bus with no member info should fire, got %d findings", len(fs))
	}
}
