package datalog

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func comp(ref, part string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
}

// mistypedFixture: U1's "VDD" pin is named like a supply but typed PASSIVE and left alone on a net
// (fires). Its "GND" pin is correctly typed power_in (left to power-input-not-driven, not this
// rule). Pin "3" is NO_CONNECT, enabling the has_nc_channel gate.
//
// Pin "4" is a SIGNAL pin alone on its own net, and it is here for the considered-set test rather
// than the finding tests. Without it the fixture cannot tell a domain scoped to supply-named pins
// apart from one that sweeps in every pin on the part: the NO_CONNECT pin sits on no net, so it
// drops out of both for a reason that has nothing to do with its role.
func mistypedFixture() *ir.Design {
	mcu := &ir.PartType{Name: "MCU", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
		{Name: "GND", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
		{Name: "NC", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},
		{Name: "IO", Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
	}}
	return &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{mcu}}},
		Components: []*ir.Component{comp("U1", "MCU")},
		Nets: []*ir.Net{
			{Name: "N$1", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "N$2", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}},
			{Name: "N$4", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "4"}}},
		},
	}
}

func TestPowerPinMistyped_Fires(t *testing.T) {
	fs := powerPinMistyped.Findings(check.NewModel(mistypedFixture()))
	if len(fs) != 1 || fs[0].Subject.Ref != "U1" || fs[0].Subject.Pin != "1" {
		t.Fatalf("want exactly {U1 pin 1 (VDD, passive, alone)}, got %+v", fs)
	}
}

// A correctly-typed power pin (power_in) and a mistyped-but-CONNECTED pin both stay silent — the
// rule fires only where name≠type AND the pin is alone on its net.
func TestPowerPinMistyped_Silent(t *testing.T) {
	mcu := &ir.PartType{Name: "MCU", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}, // correct type
		{Name: "VDDA", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE}, // mistyped but wired
		{Name: "NC", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},
	}}
	res := &ir.PartType{Name: "RES", Pins: []*ir.Pin{{Name: "~", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE}}}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{mcu, res}}},
		Components: []*ir.Component{comp("U1", "MCU"), comp("R1", "RES")},
		Nets: []*ir.Net{
			{Name: "N$1", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			// VDDA (mistyped) shares its net with R1, so it is not alone -> no fire.
			{Name: "P3V3", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}, {ComponentRef: "R1", PinRef: "1"}}},
		},
	}
	if fs := powerPinMistyped.Findings(check.NewModel(d)); len(fs) != 0 {
		t.Fatalf("want silent (correct type / connected), got %+v", fs)
	}
}

// The rule registers into DefaultCatalog under the "dl" source, so it runs in agni check / serve.
func TestRegistered(t *testing.T) {
	found := false
	for _, r := range check.DefaultCatalog().Rules() {
		if r.Name == "dl/power-pin-mistyped" {
			found = true
		}
	}
	if !found {
		t.Fatal(`rule "dl/power-pin-mistyped" not in DefaultCatalog`)
	}
}

// TestPowerPinMistyped_StatesConsideredSet (agni issue 424): the rule reports what it EXAMINED, not
// only what it faulted. On the fires fixture that is both name-derived supply pins: VDD, which is
// mistyped and alone, and GND, which is correctly typed and therefore a pass.
//
// The GND pin is the assertion that matters. It is the pin the rule looked at and cleared, and before
// this it left exactly the same trace as a pin the rule never reached.
func TestPowerPinMistyped_StatesConsideredSet(t *testing.T) {
	if !powerPinMistyped.StatesConsideredSet {
		t.Fatal("the rule declares a Domain, so it must state its considered set")
	}
	got := map[string]check.Verdict{}
	for _, v := range powerPinMistyped.Eval(check.NewModel(mistypedFixture())) {
		got[check.EntityRef(v.Subjects[0])] = v
	}
	if len(got) != 2 {
		t.Fatalf("want verdicts on both supply-named pins, got %d: %v", len(got), got)
	}
	if v := got["U1.1"]; v.Outcome != check.Fail {
		t.Errorf("U1.1 (VDD, passive, alone) = %s, want fail", v.Outcome)
	}
	v := got["U1.2"]
	if v.Outcome != check.Pass {
		t.Fatalf("U1.2 (GND, correctly typed) = %s, want pass", v.Outcome)
	}
	if v.Witness == nil || v.Witness.Statement == "" {
		t.Error("a pass with no witness is the silence this work removes")
	}
	// The SIGNAL pin is not in the set, and it is alone on a net exactly like the failing VDD pin, so
	// the only thing keeping it out is its role. A domain that swept in every pin would look like
	// better coverage while meaning less, and this is the assertion that says so.
	if _, ok := got["U1.4"]; ok {
		t.Error("a signal pin has no supply role and must not appear in the considered set")
	}
}

// TestPowerPinMistyped_FormatGateKeepsItOutOfTheSet: on a format that cannot express intentional
// no-connect the rule is structurally silent, and its considered set must be EMPTY rather than
// full of passes. Reporting every supply pin as verified by a rule that cannot run there is the
// false-pass shape the capability gate exists to prevent, arriving through the coverage half.
func TestPowerPinMistyped_FormatGateKeepsItOutOfTheSet(t *testing.T) {
	d := mistypedFixture()
	// Drop the NO_CONNECT pin: has_nc_channel goes false and the rule can conclude nothing.
	pins := d.Libraries[0].Parts[0].Pins
	d.Libraries[0].Parts[0].Pins = append(pins[:2:2], pins[3])
	if vs := powerPinMistyped.Eval(check.NewModel(d)); len(vs) != 0 {
		t.Fatalf("want no verdicts where the format cannot answer, got %d: %+v", len(vs), vs)
	}
}
