package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestComponentClass drives one fixture component per class through the Model: bare ref-des
// prefixes, part-data refinement (LED/TVS out of D, ferrite out of L), the designator_prefix
// override, the KiCad Value attribute, and the unknown cases (unfamiliar prefix, ambiguous X,
// absent ref-des).
func TestComponentClass(t *testing.T) {
	lib := &ir.PartLibrary{Name: "lib", Parts: []*ir.PartType{
		{Name: "LED_0805"},
		{Name: "SP0503BAHT", Kind: "esd"},
		{Name: "FerriteBead"},
		{Name: "RES_ARRAY", DesignatorPrefix: "R"},
		{Name: "ABM8_Crystal"},
		// A 4-pin oscillator part type declaring a Vcc supply pin — the structural oscillator signal
		// (WS10-015). Pin NAMES only; direction is left unset on purpose (EDIF under-types it).
		{Name: "OSC_4PIN", Pins: []*ir.Pin{
			{Name: "Vcc", Designator: "4"}, {Name: "OUTPUT", Designator: "3"},
			{Name: "GND", Designator: "2"}, {Name: "STANDBY", Designator: "1"},
		}},
	}}
	sec := func(part string) []*ir.ComponentSection {
		return []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}
	}
	cases := []struct {
		c    *ir.Component
		want ComponentClass
	}{
		{&ir.Component{RefDes: "R1"}, ClassResistor},
		{&ir.Component{RefDes: "RN2"}, ClassResistor},
		{&ir.Component{RefDes: "C1"}, ClassCapacitor},
		{&ir.Component{RefDes: "L1"}, ClassInductor},
		{&ir.Component{RefDes: "FB1"}, ClassFerrite},
		{&ir.Component{RefDes: "D1"}, ClassDiode},
		{&ir.Component{RefDes: "CR2"}, ClassDiode},
		{&ir.Component{RefDes: "LED1"}, ClassLED},
		{&ir.Component{RefDes: "F1"}, ClassFuse},
		{&ir.Component{RefDes: "J1"}, ClassConnector},
		{&ir.Component{RefDes: "P3"}, ClassConnector},
		{&ir.Component{RefDes: "TP7"}, ClassTestPoint},
		// WS10-015: a bare Y-prefix clock source is AMBIGUOUS (oscillator/crystal/resonator), so it
		// resolves to the clock FAMILY, not the crystal subtype — the subtype needs a specific signal.
		{&ir.Component{RefDes: "Y1"}, ClassClock},
		{&ir.Component{RefDes: "U1"}, ClassIC},
		{&ir.Component{RefDes: "IC2"}, ClassIC},
		{&ir.Component{RefDes: "Q1"}, ClassTransistor},
		// part data refines the D/L prefix within the device family
		{&ir.Component{RefDes: "D2", Sections: sec("LED_0805")}, ClassLED},
		{&ir.Component{RefDes: "D3", Sections: sec("SP0503BAHT")}, ClassTVS},
		{&ir.Component{RefDes: "L2", Sections: sec("FerriteBead")}, ClassFerrite},
		// the part's designator_prefix overrides the ref-des guess
		{&ir.Component{RefDes: "A1", Sections: sec("RES_ARRAY")}, ClassResistor},
		// KiCad-style Value attribute classifies an otherwise-unknown prefix
		{&ir.Component{RefDes: "E1", Attributes: map[string]string{"Value": "LED"}}, ClassLED},
		// WS10-015: a "crystal"/"xtal"/"resonator" token marks CLOCK-FAMILY candidacy only (the vendor
		// label can't be trusted to tell crystal from resonator), so X1 with an ABM8_Crystal part reaches
		// the clock family, not the crystal subtype — the crystal subtype comes only from a datasheet.
		{&ir.Component{RefDes: "X1", Sections: sec("ABM8_Crystal")}, ClassClock},
		// WS10-015: an "oscillator" TOKEN is family-only (unusable for subtyping on real vendor data — a
		// whole library is named "Oscillator"), so a token-only clock part stays at the clock family.
		{&ir.Component{RefDes: "X3", Attributes: map[string]string{"Description": "50MHz Oscillator"}}, ClassClock},
		// WS10-015: STRUCTURE is the reliable keyword-time oscillator signal — a Y-prefix part whose part
		// type declares a Vcc supply pin is active (the automotive-ECU lever, where the reader surfaces the pins).
		{&ir.Component{RefDes: "Y9", Sections: sec("OSC_4PIN")}, ClassOscillator},
		// a supply pin on a NON-clock part (an MCU) must NOT become an oscillator (structure is scoped).
		{&ir.Component{RefDes: "U7", Sections: sec("OSC_4PIN")}, ClassIC},
		{&ir.Component{RefDes: "X2"}, ClassUnknown},
		{&ir.Component{RefDes: "W9"}, ClassUnknown},
		// a token hint outside the prefix's device family does not flip the class
		{&ir.Component{RefDes: "R5", Attributes: map[string]string{"Value": "LED"}}, ClassResistor},
		// WS3-065: a diode's TVS/ESD identity lives in a Description / Part Label attribute (not just
		// Value), and TVS wins over the generic "diode" token whatever order they tokenize in
		{&ir.Component{RefDes: "D9", Attributes: map[string]string{
			"Description": "Diode, TVS array 24V", "Part Label": "ESD Protection Diodes"}}, ClassTVS},
		// a plain signal diode with no TVS/ESD evidence stays a diode
		{&ir.Component{RefDes: "D10", Attributes: map[string]string{"Description": "Fast switching diode 100V"}}, ClassDiode},
		// WS3-066: a debug/test/edge connector refines out of the connector base to test_connector
		{&ir.Component{RefDes: "J101", Attributes: map[string]string{"Description": "Debugger"}}, ClassTestConnector},
		{&ir.Component{RefDes: "J9001", Attributes: map[string]string{"Description": "40 POS EDGE CARD for MEC1"}}, ClassTestConnector},
		// a harness connector with no debug word stays a plain connector
		{&ir.Component{RefDes: "J1906", Attributes: map[string]string{"Description": "Molex MxDASH Gen2 harness"}}, ClassConnector},
	}
	d := &ir.Design{Libraries: []*ir.PartLibrary{lib}}
	for _, tc := range cases {
		d.Components = append(d.Components, tc.c)
	}
	m := NewModel(d)
	for _, tc := range cases {
		if got := m.ComponentClass(tc.c.RefDes); got != tc.want {
			t.Errorf("ComponentClass(%s) = %s, want %s", tc.c.RefDes, got, tc.want)
		}
	}
	if got := m.ComponentClass("NOPE1"); got != ClassUnknown {
		t.Errorf("ComponentClass(absent ref-des) = %s, want unknown", got)
	}
}

// TestModelReadsStampedDeviceClasses proves the left-shift wiring: when a design carries a
// device_classes set (the ingestion pass stamped it, WS3-071), Model.ComponentClass READS the set
// rather than re-deriving from the ref-des. A resistor-prefixed part stamped {tvs} reads tvs, so the
// data fact is authoritative; an un-stamped part still falls back to the ref-des derivation.
func TestModelReadsStampedDeviceClasses(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "R1", DeviceClasses: []string{string(ClassTVS)}}, // contradicts the R prefix on purpose
		{RefDes: "R2"}, // un-stamped -> fallback derivation
	}}
	m := NewModel(d)
	if got := m.ComponentClass("R1"); got != ClassTVS {
		t.Errorf("ComponentClass(R1) = %s, want tvs (the stamped set is authoritative)", got)
	}
	if got := m.ComponentClass("R2"); got != ClassResistor {
		t.Errorf("ComponentClass(R2) = %s, want resistor (fallback derivation)", got)
	}
}

// TestHasClassFamilyMembership: a TVS carries its diode family tag, so HasClass answers family
// membership (the isDiodeFamily replacement), while ComponentClass still reports the most-specific
// class. A test_connector does NOT satisfy HasClass(connector) — the WS3-066 split holds.
func TestHasClassFamilyMembership(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "D1", DeviceClasses: []string{string(ClassTVS), string(ClassDiode)}},
		{RefDes: "J1", DeviceClasses: []string{string(ClassTestConnector)}},
		{RefDes: "R1"}, // fallback -> {resistor}
	}}
	m := NewModel(d)
	if !m.HasClass("D1", ClassDiode) || !m.HasClass("D1", ClassTVS) {
		t.Error("HasClass(D1, diode) and HasClass(D1, tvs) should both be true for a TVS")
	}
	if m.HasClass("D1", ClassLED) {
		t.Error("HasClass(D1, led) should be false")
	}
	if got := m.ComponentClass("D1"); got != ClassTVS {
		t.Errorf("ComponentClass(D1) = %s, want tvs (most-specific)", got)
	}
	if m.HasClass("J1", ClassConnector) {
		t.Error("a test_connector must NOT satisfy HasClass(connector) (WS3-066 split)")
	}
	if !m.HasClass("R1", ClassResistor) {
		t.Error("HasClass(R1, resistor) should be true via the fallback set")
	}
	if got := m.Classes("D1"); len(got) != 2 {
		t.Errorf("Classes(D1) = %v, want 2 tags", got)
	}
}

// TestDecouplingPresent covers the rule's whole guard set: a cap-less power rail fires; a rail
// with a cap, a ground-named net, a cross-sheet rail, and a signal net (no power_in) do not.
func TestDecouplingPresent(t *testing.T) {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "REG", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_OUT}}},
			{Name: "MCU", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
		}}},
		Components: []*ir.Component{
			{RefDes: "REG1", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "REG2", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U3", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "C1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			tnet("VCC1", "REG1.1", "U1.1"),         // power rail, no cap -> fires
			tnet("VCC2", "REG2.1", "U2.1", "C1.1"), // has C1 -> quiet
			tnet("AGND", "U1.1", "U2.1"),           // ground-named -> quiet
			tnet("DATA", "U1.9", "U2.9"),           // no power_in pin -> quiet
			func() *ir.Net {
				n := tnet("VCC3", "U3.1")
				n.Attributes = map[string]string{"external": "true"}
				return n
			}(), // cross-sheet -> quiet
		},
	}
	fired := map[string]bool{}
	for _, f := range RunDesign(d) {
		if f.Rule == "decoupling-present" {
			fired[f.Subject] = true
		}
	}
	if !fired["VCC1"] {
		t.Error("VCC1 (power rail, no cap) should be flagged")
	}
	for _, quiet := range []string{"VCC2", "AGND", "DATA", "VCC3"} {
		if fired[quiet] {
			t.Errorf("%s must not be flagged", quiet)
		}
	}
}

// TestI2CPullUpSeesNonRDigitResistors is the behavioral acceptance for the component.class
// fact: a pull-up whose ref-des is not "R + digit" (a resistor network, or a part whose
// designator_prefix says resistor) must still satisfy i2c-pull-up.
func TestI2CPullUpSeesNonRDigitResistors(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "RN1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			tnet("SDA", "U1.5", "RN1.1"),
		},
	}
	for _, f := range RunDesign(d) {
		if f.Rule == "i2c-pull-up" {
			t.Errorf("SDA has RN1 (resistor network) pull-up; must not be flagged, got %+v", f)
		}
	}
}
