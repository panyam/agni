package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// xlatSpec hand-builds the part the pin tier exists for: two supply terminals with DIFFERENT
// limits, so the most-restrictive-row shortcut the alias-path rules take is wrong on it. Values
// mirror the seeded TXB0104 (VCCA 1.2..3.6 recommended, 4.6 abs-max; VCCB 1.65..5.5 recommended,
// 6.5 abs-max) without depending on that fixture.
func xlatSpec(mpn string) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	prov := func() *parampb.ParamProvenance {
		return &parampb.ParamProvenance{DocRef: "ds", Page: 5, TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 1}
	}
	pin := func(id, name string) *parampb.Pin {
		return &parampb.Pin{Id: id, Name: name, Function: parampb.PinFunction_PIN_FUNCTION_POWER_INPUT, Prov: prov()}
	}
	row := func(sym string, kind parampb.LimitKind, min, max float64, ref string) *parampb.Parameter {
		return &parampb.Parameter{
			Name: sym + " supply voltage", Symbol: sym, LimitKind: kind,
			Value: &parampb.RangeValue{Min: f(min), Max: f(max)}, Unit: "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Min: f(-40), Max: f(85), Unit: "C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			PinRefs:           []string{ref}, Prov: prov(),
		}
	}
	return &parampb.PartSpec{
		Mpn: mpn, Manufacturer: "Acme",
		Docs:     []*parampb.SourceDoc{{Id: "ds", Title: "ACME-XLAT Rev A", Vendor: "Acme"}},
		Packages: []*parampb.Package{{Id: "pw", Name: "PW (TSSOP-14)", MpnSuffix: "PW"}},
		Pins:     []*parampb.Pin{pin("vcca", "VCCA"), pin("vccb", "VCCB")},
		Parameters: []*parampb.Parameter{
			row("VCCA", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX, -0.5, 4.6, "vcca"),
			row("VCCB", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX, -0.5, 6.5, "vccb"),
			row("VCCA", parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING, 1.2, 3.6, "vcca"),
			row("VCCB", parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING, 1.65, 5.5, "vccb"),
		},
	}
}

// xlatDesign places one two-supply part with VCCA and VCCB on the named rails.
func xlatDesign(mpn, netA, netB string) *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "XLAT",
			Pins: []*ir.Pin{
				{Name: "VCCA", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Name: "VCCB", Designator: "14", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
			},
		}}}},
		Components: []*ir.Component{{
			RefDes:     "U1",
			Sections:   []*ir.ComponentSection{{PartRef: "XLAT", LibraryRef: "lib"}},
			Mpn: mpn,
			Prov:       &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{
			{Name: netA, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: netB, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "14"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
}

func xlatModel(t *testing.T, netA, netB string) check.Model {
	t.Helper()
	return check.NewModelWithParams(xlatDesign("ACME-XLAT", netA, netB), nil,
		param.ParamSet{"ACME-XLAT": xlatSpec("ACME-XLAT")})
}

// The rule's reason for existing: 5V on VCCB is WITHIN that terminal's 6.5V absolute maximum, and
// must not fire. The alias path applies the most restrictive row across every power-in pin, so it
// checks VCCB against VCCA's 4.6V and reports a violation that is not there.
func TestPinAbsMaxDoesNotFireOnAPinRatedForTheRail(t *testing.T) {
	m := xlatModel(t, "+3V3", "+5V")
	for _, f := range pinExceedsAbsMax.Findings(m) {
		if strings.Contains(f.Message, "VCCB") || strings.Contains(f.Message, "14") {
			t.Errorf("VCCB tolerates 6.5V and sits on +5V; must not fire: %s", f.Message)
		}
	}
}

// The other half: the SAME rail on the OTHER terminal is a genuine breach, and the finding has to
// name that terminal rather than the part.
func TestPinAbsMaxFiresOnTheTerminalThatIsActuallyOver(t *testing.T) {
	m := xlatModel(t, "+5V", "+5V") // both rails 5V: over VCCA's 4.6, within VCCB's 6.5
	fs := pinExceedsAbsMax.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("want exactly one finding (VCCA only), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "VCCA") {
		t.Errorf("finding must name the offending terminal, got %q", fs[0].Message)
	}
	if !strings.Contains(fs[0].Message, "4.6") {
		t.Errorf("finding must cite THAT pin's limit (4.6), got %q", fs[0].Message)
	}
	if len(fs[0].DatasheetProv) == 0 {
		t.Error("every param-backed finding carries a datasheet citation")
	}
}

// The recommended-range rule answers per terminal on a part rail-nominal-out-of-recommended skips
// entirely, because that rule acts only on a spec with exactly ONE recommended row.
func TestPinOutOfRecommendedAnswersPerTerminal(t *testing.T) {
	m := xlatModel(t, "+5V", "+5V") // over VCCA's 3.6 max, inside VCCB's 1.65..5.5
	fs := pinOutOfRecommended.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("want one finding (VCCA over its 3.6 max), got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "VCCA") || !strings.Contains(fs[0].Message, "3.6") {
		t.Errorf("finding must name the terminal and its own range, got %q", fs[0].Message)
	}
	if n := len(railNominalOutOfRecommended.Findings(m)); n != 0 {
		t.Errorf("the alias-path rule declines multi-supply parts; want 0 findings, got %d", n)
	}
}

// A rail below a terminal's recommended MINIMUM is as much a breach as one above its maximum, and
// the two-sided answer is what the alias path could not give for a multi-supply part.
func TestPinOutOfRecommendedCatchesUnderVoltage(t *testing.T) {
	m := xlatModel(t, "+3V3", "+1V2") // VCCB minimum is 1.65
	fs := pinOutOfRecommended.Findings(m)
	if len(fs) != 1 || !strings.Contains(fs[0].Message, "VCCB") {
		t.Fatalf("want one VCCB under-voltage finding, got %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "1.65") {
		t.Errorf("finding must cite the terminal's own minimum, got %q", fs[0].Message)
	}
}

// Degrade-safety (C9) and the no-double-report rule, together. A spec with no pin data must keep
// the alias path exactly as it behaves today, and a spec WITH pin data must silence it so one
// problem is reported once.
func TestAliasPathDefersOnlyWhenPinDataExists(t *testing.T) {
	withPins := xlatModel(t, "+5V", "+5V")
	if n := len(supplyExceedsAbsMax.Findings(withPins)); n != 0 {
		t.Errorf("with pin bindings the alias rule must defer; want 0 findings, got %d", n)
	}
	if n := len(pinExceedsAbsMax.Findings(withPins)); n == 0 {
		t.Error("the pin rule must cover what the alias rule stopped reporting")
	}

	// The pre-pin-binding shape: same rules, unchanged behaviour.
	noPins := check.NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil,
		param.ParamSet{"ACME-33": ldoSpec("ACME-33", 3.6)})
	if n := len(supplyExceedsAbsMax.Findings(noPins)); n != 1 {
		t.Errorf("a spec with no pin data keeps today's behaviour; want 1 alias finding, got %d", n)
	}
	if n := len(pinExceedsAbsMax.Findings(noPins)); n != 0 {
		t.Errorf("the pin rule has nothing to bind to; want 0 findings, got %d", n)
	}
}

// Skip-not-false-pass across every missing input.
func TestPinRatingRulesSilentWithoutTheirInputs(t *testing.T) {
	cases := []struct {
		name string
		m    check.Model
	}{
		{"no params tier at all", check.NewModel(xlatDesign("ACME-XLAT", "+5V", "+5V"))},
		{"no seeded spec for the mpn", check.NewModelWithParams(
			xlatDesign("ACME-XLAT", "+5V", "+5V"), nil, param.ParamSet{})},
		{"no voltage evidence on the rails", xlatModel(t, "VDD_MAIN", "VDD_AUX")},
	}
	for _, tc := range cases {
		if n := len(pinExceedsAbsMax.Findings(tc.m)); n != 0 {
			t.Errorf("%s: want 0 abs-max findings, got %d", tc.name, n)
		}
		if n := len(pinOutOfRecommended.Findings(tc.m)); n != 0 {
			t.Errorf("%s: want 0 recommended findings, got %d", tc.name, n)
		}
	}
}

// A design pin the spec cannot resolve unambiguously must produce nothing. Two spec pins share the
// name the design uses and no package is identified, so ResolvePin refuses; refusing is the whole
// safety property, and a rule that fell back to the first match would report on a guessed terminal.
func TestPinRatingSkipsAnUnresolvablePin(t *testing.T) {
	spec := xlatSpec("ACME-XLAT")
	spec.Pins[1].Name = "VCCA" // both terminals now print VCCA; the name no longer separates them
	spec.Packages = nil        // and no package is declared, so the number cannot break the tie
	for _, p := range spec.Pins {
		p.Numbers = nil
	}
	m := check.NewModelWithParams(xlatDesign("ACME-XLAT", "+5V", "+5V"), nil,
		param.ParamSet{"ACME-XLAT": spec})

	if n := len(pinExceedsAbsMax.Findings(m)); n != 0 {
		t.Errorf("an ambiguous pin must be skipped, not guessed; got %d findings", n)
	}
}
