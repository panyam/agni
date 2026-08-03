package datalogrules

import (
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func comp(ref, part string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
}

// mistypedFixture: U1's "VDD" pin is named like a supply but typed PASSIVE and left alone on a net
// (fires). Its "GND" pin is correctly typed power_in (left to power-input-not-driven, not this
// rule). Pin "3" is NO_CONNECT, enabling the has_nc_channel gate.
func mistypedFixture() *ir.Design {
	mcu := &ir.PartType{Name: "MCU", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
		{Name: "GND", Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
		{Name: "NC", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},
	}}
	return &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{mcu}}},
		Components: []*ir.Component{comp("U1", "MCU")},
		Nets: []*ir.Net{
			{Name: "N$1", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}},
			{Name: "N$2", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}},
		},
	}
}

func TestPowerPinMistyped_Fires(t *testing.T) {
	fs := powerPinMistyped.Eval(check.NewModel(mistypedFixture()))
	if len(fs) != 1 || fs[0].Subject != "U1" || fs[0].Pin != "1" {
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
	if fs := powerPinMistyped.Eval(check.NewModel(d)); len(fs) != 0 {
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
