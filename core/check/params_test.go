package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/datasheet/param"
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

// TestUnitVocabulariesAgree is a drift tripwire between the two layers that own a unit vocabulary.
//
// core/classify parses a component's value TEXT off a design and normalizes it to an SI base unit;
// datasheet/param converts a seeded parameter's printed unit to the same base. They are deliberately
// SEPARATE tables, because IEC 60062's RKM code reads M as mega and is case-insensitive on k and u,
// which is correct for a schematic value field and inverts three orders of magnitude on a printed unit
// symbol. The datasheet tier also imports nothing from core (C17), so there is no import to hold them
// together.
//
// What they must agree on is the CANONICAL SPELLING they each normalize to. A rule comparing a
// design-side resistance against a datasheet-side one compares two unit strings, so a divergence here
// would make every such comparison silently find nothing, which is this file's whole failure mode.
// This test lives in core/check because it is the one package that imports both layers.
func TestUnitVocabulariesAgree(t *testing.T) {
	if param.UnitOhm != classify.UnitOhm {
		t.Errorf("ohm symbol diverged: datasheet/param has %q (%U), core/classify has %q (%U)",
			param.UnitOhm, []rune(param.UnitOhm), classify.UnitOhm, []rune(classify.UnitOhm))
	}
	// Every base unit classify normalizes a design value to must survive a round trip through the
	// parameter layer unchanged, or a datasheet row in that unit could never be compared against a
	// component carrying it.
	for _, unit := range []string{classify.UnitOhm, "F", "H", "A", "V"} {
		base, exp, ok := param.BaseUnit(unit)
		if !ok || base != unit || exp != 0 {
			t.Errorf("classify normalizes design values to %q, but param.BaseUnit gives (%q, %d, %v)",
				unit, base, exp, ok)
		}
	}
}
