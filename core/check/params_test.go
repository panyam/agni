package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestNominalVoltageFromName(t *testing.T) {
	cases := []struct {
		name string
		want float64
		ok   bool
	}{
		{"+5V", 5, true},
		{"5V", 5, true},
		{"3V3", 3.3, true},
		{"1V8", 1.8, true},
		{"12V0", 12, true},
		{"3.3V", 3.3, true},
		{"VCC_5V", 5, true},
		{"5V0_RAIL", 5, true},
		{"+24V_IN", 24, true},
		{"vbus_5v", 5, true},
		{"GND", 0, false},
		{"PCIE", 0, false},
		{"USB5V", 0, false},     // embedded, not a delimited voltage token
		{"EVENT5", 0, false},    // trailing digit is not a voltage
		{"12V_TO_5V", 0, false}, // two conflicting nominals: refuse to guess
		{"VCC", 0, false},       // rail-ish but carries no number
		{"5V_AND_5V0", 5, true}, // two tokens, same nominal: unambiguous
	}
	for _, tc := range cases {
		got, ok := NominalVoltageFromName(tc.name)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("NominalVoltageFromName(%q) = %v,%v want %v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

// supplyDesign builds a one-part design: U1 (part LDO, pin 1 = VDD power_in) with its
// VDD pin on the given net, joined to a spec via the given identity channel. It is a shared
// fixture for the Available param-tier gate here and (a copy) the datasheet rule tests in
// stdlib/rules/builtin.
func supplyDesign(netName string, viaBomLine bool, mpn string) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "LDO",
			Pins: []*ir.Pin{{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}},
		}}}},
		Components: []*ir.Component{{
			RefDes:     "U1",
			Sections:   []*ir.ComponentSection{{PartRef: "LDO", LibraryRef: "lib"}},
			Attributes: map[string]string{},
			Prov:       &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{{
			Name:        netName,
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}},
			Prov:        &ir.Provenance{SourceFile: "t"},
		}},
	}
	if mpn != "" {
		if viaBomLine {
			d.Bom = []*ir.BomLine{{RefDes: []string{"U1"}, Mpn: mpn, Manufacturer: "Acme"}}
		} else {
			d.Components[0].Attributes["MPN"] = mpn
		}
	}
	return d
}
