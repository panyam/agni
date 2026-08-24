package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// decouplingDesign builds one net carrying an IC's supply pin and whatever else the case needs, with
// no capacitor anywhere. Every design here would fire the rule before the scope guards; what differs
// is whether the net is a supply rail at all.
func decouplingDesign(extra ...*ir.Connection) *ir.Design {
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	fet := &ir.PartType{Name: "FET", Pins: []*ir.Pin{
		{Name: "G", Designator: "1"}, {Name: "S", Designator: "2"}, {Name: "D", Designator: "3"},
	}}
	coil := &ir.PartType{Name: "COIL", Pins: []*ir.Pin{{Name: "1", Designator: "1"}, {Name: "2", Designator: "2"}}}
	conns := append([]*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, extra...)
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{load, fet, coil}}},
		Components: []*ir.Component{
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "LOAD", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "Q1", Sections: []*ir.ComponentSection{{PartRef: "FET", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "L1", Sections: []*ir.ComponentSection{{PartRef: "COIL", LibraryRef: "lib"}}, DeviceClasses: []string{"inductor"}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{{Name: "NODE", Prov: &ir.Provenance{SourceFile: "t"}, Connections: conns}},
	}
}

func decouplingFires(t *testing.T, d *ir.Design) bool {
	t.Helper()
	return len(decouplingPresent.Findings(check.NewModel(d))) > 0
}

// The positive control, and it comes first because every silence assertion below is worthless
// without it: this fixture has a bare supply pin and no capacitor, and the rule must fire.
func TestDecouplingFiresOnABareSupplyPin(t *testing.T) {
	if !decouplingFires(t, decouplingDesign()) {
		t.Fatal("a supply pin with no capacitor is the defect this rule exists for")
	}
}

// A gate-drive net carries a controller output and a FET's GATE. It is a control node, and on one
// real board six of this rule's fourteen findings were this shape (agni issue 382).
func TestDecouplingIsSilentOnAGateDrive(t *testing.T) {
	if decouplingFires(t, decouplingDesign(&ir.Connection{ComponentRef: "Q1", PinRef: "1"})) {
		t.Error("a net driving a transistor's gate is a control node, not a rail to decouple")
	}
}

// The guard asks about the GATE rather than about the transistor, and this is why. A high-side load
// switch's OUTPUT carries the FET's SOURCE, feeds downstream supply pins, and wants decoupling
// exactly as much as a regulator output does. Disqualifying any net with a transistor on it would be
// simpler and would silence a real defect.
func TestDecouplingStillFiresOnALoadSwitchOutput(t *testing.T) {
	if !decouplingFires(t, decouplingDesign(&ir.Connection{ComponentRef: "Q1", PinRef: "2"})) {
		t.Error("a FET's source feeding a supply pin is a switched rail; it still wants a capacitor")
	}
}

// A buck's switch node carries the switching FET and the inductor. Telling someone to fit a capacitor
// there is advice that shorts the switch to ground every cycle, which is worse than saying nothing.
func TestDecouplingIsSilentOnASwitchingNode(t *testing.T) {
	d := decouplingDesign(
		&ir.Connection{ComponentRef: "Q1", PinRef: "2"}, // source: the switch node side
		&ir.Connection{ComponentRef: "L1", PinRef: "1"},
	)
	if decouplingFires(t, d) {
		t.Error("an inductor beside a transistor is a switching node, not a rail to decouple")
	}
}

// And the inductor only disqualifies alongside a transistor. An inductor on its own is an LC or
// ferrite FILTER, and a filtered supply is a rail that genuinely wants decoupling on the far side.
// The committed reach.fires fixture is this shape, so an inductor-alone guard would have silenced a
// real finding to catch a case the gate guard already catches.
func TestDecouplingStillFiresBehindAFilterInductor(t *testing.T) {
	if !decouplingFires(t, decouplingDesign(&ir.Connection{ComponentRef: "L1", PinRef: "1"})) {
		t.Error("a ferrite- or LC-filtered supply is a rail; the inductor alone must not disqualify it")
	}
}
