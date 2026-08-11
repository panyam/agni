package param

import (
	"testing"

	"google.golang.org/protobuf/proto"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ptr is the local optional-bound helper; RangeValue's bounds are optional doubles so that absent is
// distinguishable from zero.
func ptr(v float64) *float64 { return &v }

// row builds a minimal comparable parameter in the given unit. Condition coverage is UNCONDITIONAL so
// the row is not under-specified, which keeps these tests about units and nothing else.
func row(unit string, min, typ, max *float64) *parampb.Parameter {
	return &parampb.Parameter{
		Symbol:            "X",
		LimitKind:         parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
		Unit:              unit,
		Value:             &parampb.RangeValue{Min: min, Typ: typ, Max: max},
		ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL,
		Prov:              &parampb.ParamProvenance{DocRef: "d", Page: 6, Confidence: 1},
	}
}

func TestBaseUnitReducesPrintedSpellings(t *testing.T) {
	for _, tc := range []struct {
		printed string
		base    string
		exp     int
	}{
		{"V", "V", 0},
		{"mV", "V", -3},
		{"kV", "V", 3},
		{"µV", "V", -6},
		{"A", "A", 0},
		{"mA", "A", -3},
		{"nA", "A", -9},
		{UnitOhm, UnitOhm, 0},
		{ohmSign, UnitOhm, 0}, // the deprecated OHM SIGN codepoint, written as the constant because it
		// is byte-different from and visually identical to U+03A9, so a literal here would be untestable
		// by eye and would silently duplicate the row above.
		{"Ohm", UnitOhm, 0},
		{"ohms", UnitOhm, 0},
		{"mΩ", UnitOhm, -3},
		{"MΩ", UnitOhm, 6},
		{"pF", "F", -12},
		{"µH", "H", -6},
		{"mW", "W", -3},
		{"ns", "s", -9},
		{"MHz", "Hz", 6},
		{"C", "C", 0},
		{"°C", "C", 0},
	} {
		base, exp, ok := BaseUnit(tc.printed)
		if !ok || base != tc.base || exp != tc.exp {
			t.Errorf("BaseUnit(%q) = (%q, %d, %v), want (%q, %d, true)", tc.printed, base, exp, ok, tc.base, tc.exp)
		}
	}
}

// TestBaseUnitRefusesUnrecognized holds the closed vocabulary. Each of these could plausibly be
// "helpfully" resolved, and each would resolve to a guess: an empty unit is not evidence the number is
// in the unit a caller wants, uppercase K is kelvin rather than kilo, a millidegree is a typo rather
// than a temperature, and a compound or vendor unit has no single scale at all.
func TestBaseUnitRefusesUnrecognized(t *testing.T) {
	for _, printed := range []string{"", " ", "V ", "dBm", "A/µs", "KV", "mC", "K", "uV", "MV/m", "Vpp"} {
		if base, exp, ok := BaseUnit(printed); ok {
			t.Errorf("BaseUnit(%q) = (%q, %d, true), want refused", printed, base, exp)
		}
	}
}

// TestBaseUnitCaseIsNormative is the test that earns the case-sensitive table. The milli/mega pair is
// the one spelling collision in SI that a datasheet actually prints on both sides, and resolving it by
// guessing puts a number nine orders of magnitude out while looking exactly as authoritative as a
// correct one.
func TestBaseUnitCaseIsNormative(t *testing.T) {
	for _, tc := range []struct{ lower, upper string }{
		{"mΩ", "MΩ"},
		{"mHz", "MHz"},
		{"mW", "MW"},
	} {
		_, lo, okLo := BaseUnit(tc.lower)
		_, hi, okHi := BaseUnit(tc.upper)
		if !okLo || !okHi {
			t.Fatalf("both %q and %q must resolve, got %v/%v", tc.lower, tc.upper, okLo, okHi)
		}
		if lo != -3 || hi != 6 {
			t.Errorf("%q/%q = %d/%d, want -3/6", tc.lower, tc.upper, lo, hi)
		}
	}
}

// TestUnitScalesHasNoCollisions holds the generated table honest. Flattening a cross product silently
// wins a duplicate key on whichever iteration lands last, so a base unit added later whose spelling
// happens to be a prefixed form of another one would change an existing unit's scale rather than fail
// to compile. Map iteration order means it would not even change it deterministically.
func TestUnitScalesHasNoCollisions(t *testing.T) {
	seen := map[string]unitScale{}
	claim := func(spelling string, s unitScale) {
		if prev, dup := seen[spelling]; dup && prev != s {
			t.Errorf("%q means both (%s, %d) and (%s, %d)", spelling, prev.base, prev.exp, s.base, s.exp)
		}
		seen[spelling] = s
	}
	for spelling, base := range prefixableUnits {
		claim(spelling, unitScale{base, 0})
		for prefix, exp := range siPrefixes {
			claim(prefix+spelling, unitScale{base, exp})
		}
	}
	for spelling, base := range unprefixedUnits {
		claim(spelling, unitScale{base, 0})
	}
	if len(seen) != len(unitScales) {
		t.Errorf("generated %d distinct spellings but the table holds %d", len(seen), len(unitScales))
	}
}

func TestInBaseUnitScalesEveryPresentBound(t *testing.T) {
	got, ok := InBaseUnit(row("mV", ptr(45), ptr(50), ptr(50)))
	if !ok {
		t.Fatal("a millivolt row must convert")
	}
	if got.Unit != "V" {
		t.Errorf("unit = %q, want V", got.Unit)
	}
	if got.Value.GetMin() != 0.045 || got.Value.GetTyp() != 0.05 || got.Value.GetMax() != 0.05 {
		t.Errorf("bounds = %v/%v/%v, want 0.045/0.05/0.05",
			got.Value.GetMin(), got.Value.GetTyp(), got.Value.GetMax())
	}
}

// TestInBaseUnitLeavesAbsentBoundsAbsent: a row stating only a max is a real shape, and an absent
// bound arriving downstream as a scaled zero would read as "this part is rated for zero volts", which
// every comparison would treat as a defect.
func TestInBaseUnitLeavesAbsentBoundsAbsent(t *testing.T) {
	got, ok := InBaseUnit(row("mA", nil, nil, ptr(500)))
	if !ok {
		t.Fatal("a milliamp row must convert")
	}
	if got.Value.Min != nil || got.Value.Typ != nil {
		t.Errorf("absent bounds became present: min=%v typ=%v", got.Value.Min, got.Value.Typ)
	}
	if got.Value.GetMax() != 0.5 {
		t.Errorf("max = %v, want 0.5", got.Value.GetMax())
	}
}

// TestInBaseUnitRespellingIsExact is the reason scalePow10 divides instead of multiplying by a
// negative power of ten. A row transcribed as 50 mV and the same row transcribed as 0.05 V are the
// same datasheet row written twice, so they must produce the IDENTICAL double: 50 * 1e-3 does not,
// because 1e-3 has no exact binary form and the result rounds twice. Rewriting the negative branch as
// a multiplication fails this test.
func TestInBaseUnitRespellingIsExact(t *testing.T) {
	converted, ok := InBaseUnit(row("mV", nil, nil, ptr(50)))
	if !ok {
		t.Fatal("a millivolt row must convert")
	}
	if got, want := converted.Value.GetMax(), 0.05; got != want {
		t.Errorf("50 mV converted to %v, want exactly the double %v that 0.05 V parses to", got, want)
	}
	for _, tc := range []struct {
		printed string
		value   float64
		want    float64
	}{
		{"mA", 500, 0.5},
		{"mΩ", 5, 0.005},
		{"pF", 100, 1e-10},
		{"kV", 8, 8000},
	} {
		c, _ := InBaseUnit(row(tc.printed, nil, nil, ptr(tc.value)))
		if got := c.Value.GetMax(); got != tc.want {
			t.Errorf("%g%s converted to %v, want exactly %v", tc.value, tc.printed, got, tc.want)
		}
	}
}

// TestInBaseUnitReturnsBaseRowsUnchanged holds the no-allocation common path AND the pointer
// identity a resolver relies on when it stores a returned row beside the spec it came from.
func TestInBaseUnitReturnsBaseRowsUnchanged(t *testing.T) {
	for _, unit := range []string{"V", "A", UnitOhm, "C"} {
		p := row(unit, nil, nil, ptr(3.3))
		got, ok := InBaseUnit(p)
		if !ok {
			t.Fatalf("%q must convert", unit)
		}
		if got != p {
			t.Errorf("%q: returned a copy, want the same pointer", unit)
		}
	}
}

// TestInBaseUnitNormalizesOhmSpelling: "Ohm" and "Ω" are two spellings of ONE unit, not two units, so
// normalizing them is not the conversion this file otherwise performs. The value must survive it
// untouched, and the row must still be a copy, since rewriting the spec's own Unit string in place
// would change what a citation and the params panel display.
func TestInBaseUnitNormalizesOhmSpelling(t *testing.T) {
	p := row("Ohm", nil, nil, ptr(0.05))
	got, ok := InBaseUnit(p)
	if !ok {
		t.Fatal(`"Ohm" must resolve`)
	}
	if got.Unit != UnitOhm {
		t.Errorf("unit = %q, want %q", got.Unit, UnitOhm)
	}
	if got.Value.GetMax() != 0.05 {
		t.Errorf("max = %v, want 0.05 unchanged", got.Value.GetMax())
	}
	if p.Unit != "Ohm" {
		t.Errorf("source unit rewritten to %q; the printed spelling must survive", p.Unit)
	}
}

// TestInBaseUnitDoesNotMutateSource: one spec is shared across every rule in a run, and the printed
// row is what a citation, the params panel and the `param` relations must keep showing. A converting
// extractor that mutated in place would silently rewrite all three.
func TestInBaseUnitDoesNotMutateSource(t *testing.T) {
	p := row("mV", ptr(45), ptr(50), ptr(50))
	before := proto.Clone(p).(*parampb.Parameter)
	if _, ok := InBaseUnit(p); !ok {
		t.Fatal("a millivolt row must convert")
	}
	if !proto.Equal(p, before) {
		t.Errorf("source mutated: got %v, want %v", p, before)
	}
}

// TestInBaseUnitCarriesProvenance: the converted row is what a rule cites, so losing prov would
// detach a finding from the document it came from, and losing conditions would let a row that
// MachineComparable should reject read as comparable.
func TestInBaseUnitCarriesProvenance(t *testing.T) {
	p := row("mA", nil, nil, ptr(500))
	p.Name = "Output current"
	p.Conditions = []*parampb.Condition{{Symbol: "TA", Eq: ptr(25), Unit: "C", Raw: "TA = 25C"}}
	p.Attributes = map[string]string{"note": "n"}
	got, ok := InBaseUnit(p)
	if !ok {
		t.Fatal("a milliamp row must convert")
	}
	if got.Name != "Output current" || got.Symbol != "X" || got.LimitKind != p.LimitKind {
		t.Errorf("identity fields lost: %+v", got)
	}
	if got.GetProv().GetDocRef() != "d" || got.GetProv().GetPage() != 6 {
		t.Errorf("provenance lost: %+v", got.GetProv())
	}
	if len(got.Conditions) != 1 || got.Conditions[0].Symbol != "TA" {
		t.Errorf("conditions lost: %+v", got.Conditions)
	}
	if got.Attributes["note"] != "n" {
		t.Errorf("attributes lost: %+v", got.Attributes)
	}
	if !MachineComparable(got) {
		t.Error("a converted row must still answer MachineComparable the way its source did")
	}
}

// TestInBaseUnitLeavesConditionsInPrintedUnits: a condition qualifies the row rather than carrying its
// value, MachineComparable only asks that conditions be structured rather than evaluated, and nothing
// compares against one today. Converting a number no consumer reads would be the speculative half of
// this change, and it would make the condition text disagree with the datasheet page.
func TestInBaseUnitLeavesConditionsInPrintedUnits(t *testing.T) {
	p := row("V", nil, nil, ptr(1.3))
	p.Conditions = []*parampb.Condition{{Symbol: "IOUT", Eq: ptr(800), Unit: "mA", Raw: "IOUT = 800 mA"}}
	got, _ := InBaseUnit(p)
	if c := got.Conditions[0]; c.Unit != "mA" || c.GetEq() != 800 {
		t.Errorf("condition converted to %v%s, want 800mA as printed", c.GetEq(), c.Unit)
	}
}

func TestInBaseUnitRefusesUnknownUnitAndNil(t *testing.T) {
	if got, ok := InBaseUnit(row("dBm", nil, nil, ptr(10))); ok {
		t.Errorf("an unrecognized unit must be refused, got %+v", got)
	}
	if got, ok := InBaseUnit(row("", nil, nil, ptr(10))); ok {
		t.Errorf("an unstated unit must be refused, got %+v", got)
	}
	if _, ok := InBaseUnit(nil); ok {
		t.Error("nil must be refused")
	}
}

// TestInBaseUnitHandlesAValuelessRow: Validate rejects a row with no bounds, but an extractor calls
// this BEFORE it checks for one, so the conversion has to survive the shape rather than panic on it.
func TestInBaseUnitHandlesAValuelessRow(t *testing.T) {
	p := row("mV", nil, nil, nil)
	p.Value = nil
	got, ok := InBaseUnit(p)
	if !ok {
		t.Fatal("a millivolt row with no value must still convert its unit")
	}
	if got.Unit != "V" || got.Value != nil {
		t.Errorf("got unit %q value %v, want V and nil", got.Unit, got.Value)
	}
}
