package classify

import (
	"math"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func compWithValue(ref, val string, classes ...string) *ir.Component {
	c := &ir.Component{RefDes: ref, DeviceClasses: Tags(classes...), Prov: &ir.Provenance{SourceFile: "t"}}
	if val != "" {
		c.Attributes = map[string]string{"Value": val}
	}
	return c
}

// TestStampValuesFillsQuantity checks that the pass turns each format's value text into a number
// once at ingestion, so a rule reads a Quantity rather than re-parsing a vendor string.
func TestStampValuesFillsQuantity(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		compWithValue("R1", "4k7", string(ClassResistor)),
		compWithValue("C1", "100nF", string(ClassCapacitor)),
	}}
	StampValues(d)
	if got := d.Components[0].GetValue(); got.GetValue() != 4700 || got.GetInput() != "4k7" {
		t.Errorf("R1 = %v %q, want 4700 from \"4k7\"", got.GetValue(), got.GetInput())
	}
	if got := d.Components[1].GetValue(); got.GetValue() != 1e-7 || got.GetUnit() != "F" {
		t.Errorf("C1 = %v %q, want 1e-7 F", got.GetValue(), got.GetUnit())
	}
}

// TestStampValuesBareNumberNeedsTheClassConvention is why ValueVocab exists rather than the parser
// deciding. The SAME text "100" means ohms on a resistor and nothing determinate on a capacitor,
// because only the first is a universal convention.
func TestStampValuesBareNumberNeedsTheClassConvention(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		compWithValue("R1", "100", string(ClassResistor)),
		compWithValue("C1", "100", string(ClassCapacitor)),
	}}
	StampValues(d)
	if u := d.Components[0].GetValue().GetUnit(); u != UnitOhm {
		t.Errorf("a bare resistor value is ohms by universal convention, got unit %q", u)
	}
	if u := d.Components[1].GetValue().GetUnit(); u != "" {
		t.Errorf("a BARE capacitor value is genuinely ambiguous and must NOT be guessed, got unit %q", u)
	}
	// The NUMBER is still known in both cases. Refusing the unit is not refusing the value.
	if v := d.Components[1].GetValue().GetValue(); v != 100 {
		t.Errorf("the number is known even when the unit is not, got %v", v)
	}
}

// TestStampValuesPrefixedValueTakesTheClassDimension is the distinction a real read forced. "10u" on a
// capacitor states the MAGNITUDE and omits only the dimension, and a capacitor's value is a capacitance
// with no ambiguity at all, so the class settles it. Only a value with no prefix EITHER is genuinely
// uncertain.
//
// Getting this wrong is not a corner case, because almost nobody writes the F or the H, so
// collapsing it into the bare-number rule leaves every capacitor and inductor with an empty unit,
// which ComponentValueIn then rejects. The feature would report nothing for most passives on a real
// board.
func TestStampValuesPrefixedValueTakesTheClassDimension(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		compWithValue("C1", "10u", string(ClassCapacitor)),
		compWithValue("L1", "4u7", string(ClassInductor)),
		compWithValue("R1", "4k7", string(ClassResistor)),
	}}
	StampValues(d)
	for i, want := range []struct {
		unit string
		val  float64
	}{{"F", 1e-5}, {"H", 4.7e-6}, {UnitOhm, 4700}} {
		q := d.Components[i].GetValue()
		if q.GetUnit() != want.unit || q.GetValue() != want.val {
			t.Errorf("%s = %v %q, want %v %q", d.Components[i].GetRefDes(), q.GetValue(), q.GetUnit(), want.val, want.unit)
		}
	}
}

// TestStampValuesDeclaredConventionFillsTheGap checks that a house that spells bare capacitor
// values in microfarads says so through the vocabulary, and the parser is untouched.
func TestStampValuesDeclaredConventionFillsTheGap(t *testing.T) {
	lex := &Lexicon{Value: BuildValueVocab(map[string]string{"capacitor": "F"})}
	d := &ir.Design{Components: []*ir.Component{compWithValue("C1", "100", string(ClassCapacitor))}}
	lex.StampValues(d)
	if u := d.Components[0].GetValue().GetUnit(); u != "F" {
		t.Errorf("a declared convention must fill the unit, got %q", u)
	}
}

// TestStampValuesUnparsedKeepsTheInput keeps "no value stated" and "a value stated we could not
// read" apart, because they are different facts. Only the second is a gap worth reporting, so they
// must not collapse.
func TestStampValuesUnparsedKeepsTheInput(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		compWithValue("R1", "DNP", string(ClassResistor)),
		compWithValue("R2", "", string(ClassResistor)),
	}}
	StampValues(d)
	q := d.Components[0].GetValue()
	if q == nil || q.GetInput() != "DNP" {
		t.Fatalf("an unparsed value must keep its input, got %v", q)
	}
	if q.Value != nil {
		t.Errorf("an unparsed value must have NO number (zero is a legal resistance), got %v", q.GetValue())
	}
	if d.Components[1].GetValue() != nil {
		t.Error("a component with no value attribute must carry no Quantity at all")
	}
}

// TestStampValuesIsIdempotent checks that a re-stamp after a re-read does not accumulate or drift,
// matching Stamp.
func TestStampValuesIsIdempotent(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{compWithValue("R1", "4k7", string(ClassResistor))}}
	StampValues(d)
	first := d.Components[0].GetValue().GetValue()
	StampValues(d)
	if got := d.Components[0].GetValue().GetValue(); got != first {
		t.Errorf("re-stamp changed the value: %v then %v", first, got)
	}
}

// A library that names its parts by value writes that name as the Value (agni issue 1115). Every
// passive on the Jetson baseboard does, and none had a value until the name was read. Only a token with
// a prefix or a unit, in the class's own unit, is the value, so the package size and the rating beside
// it are never mistaken for one, and a name holding two different values reads as none.
func TestStampValuesReadsALibraryPartName(t *testing.T) {
	for _, tc := range []struct {
		value, class string
		want         float64
		unit         string
	}{
		{"R_10k_0402", "resistor", 10e3, UnitOhm},
		{"R_2k2_0402", "resistor", 2200, UnitOhm},
		{"R_49R9_0402", "resistor", 49.9, UnitOhm},
		{"R_0R001_0805", "resistor", 0.001, UnitOhm},
		{"R_0R_22.4A_0603", "resistor", 0, UnitOhm},
		{"C_100n_0402", "capacitor", 1e-7, "F"},
		{"C_4u7_25V_0402", "capacitor", 4.7e-6, "F"},
		{"L_1u8_6.8A_WE-MAPI-4020", "inductor", 1.8e-6, "H"},
	} {
		d := &ir.Design{Components: []*ir.Component{compWithValue("X1", tc.value, tc.class)}}
		StampValues(d)
		q := d.Components[0].GetValue()
		if q.Value == nil || math.Abs(q.GetValue()-tc.want) > 1e-12*math.Max(1, tc.want) || q.GetUnit() != tc.unit {
			t.Errorf("%s (%s) = %v %q, want %v %q", tc.value, tc.class, q.Value, q.GetUnit(), tc.want, tc.unit)
		}
		if q.GetInput() != tc.value {
			t.Errorf("%s keeps %q as its input, want the whole name a finding can quote", tc.value, q.GetInput())
		}
	}

	for _, tc := range []struct{ value, class, why string }{
		{"R_0402", "resistor", "a bare package size is not ohms"},
		{"C_25V_0402", "capacitor", "a voltage rating is not a capacitance"},
		{"R_10k_22k_0402", "resistor", "two different values say nothing reliable"},
		{"U_100n_QFN48", "ic", "an IC has no unit of its own"},
		{"C_0R_0402", "capacitor", "ohms on a capacitor are not its value"},
	} {
		d := &ir.Design{Components: []*ir.Component{compWithValue("X1", tc.value, tc.class)}}
		StampValues(d)
		if q := d.Components[0].GetValue(); q.Value != nil {
			t.Errorf("%s (%s) read as %v %q; %s", tc.value, tc.class, q.GetValue(), q.GetUnit(), tc.why)
		}
	}
}
