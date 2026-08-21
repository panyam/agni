package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ldoRecommendedSpec hand-builds a seeded part whose recommended-operating VDD range is
// [min,max], as a machine-comparable row (structured TA condition — the shape a
// datasheet's Recommended Operating Conditions table yields).
func ldoRecommendedSpec(mpn string, min, max float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	return &parampb.PartSpec{
		Mpn:          mpn,
		Manufacturer: "Acme",
		Docs:         []*parampb.SourceDoc{{Id: "ds", Title: "ACME-LDO Rev B", Vendor: "Acme"}},
		Parameters: []*parampb.Parameter{{
			Name: "Supply voltage, recommended", Symbol: "VDD",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:             &parampb.RangeValue{Min: f(min), Max: f(max)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 6, TableOrFigure: "Recommended Operating Conditions",
				Method: "hand", Confidence: 1,
			},
		}},
	}
}

func runRailNominal(t *testing.T, d *ir.Design, set param.ParamSet) []check.Finding {
	t.Helper()
	return check.Run(check.NewModelWithParams(d, nil, set), []*check.Rule{railNominalOutOfRecommended})
}

func TestRailNominalFiresOverAndUnder(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)}

	over := runRailNominal(t, supplyDesign("+5V", false, "ACME-33"), set)
	if len(over) != 1 {
		t.Fatalf("5V on a 3.0..3.6V part: want 1 finding, got %v", over)
	}
	f := over[0]
	if f.Subject.Kind != check.KindComponent || check.EntityRef(f.Subject) != "U1" {
		t.Errorf("subject = %s/%s, want component/U1", f.Subject.Kind, f.Subject)
	}
	for _, want := range []string{"+5V", "exceeds recommended maximum 3.6V", "VDD",
		"ACME-LDO Rev B", "page 6", "Recommended Operating Conditions", "confidence 1"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("over-voltage message missing %q: %s", want, f.Message)
		}
	}

	under := runRailNominal(t, supplyDesign("+1V8", false, "ACME-33"), set)
	if len(under) != 1 || !strings.Contains(under[0].Message, "is below recommended minimum 3V") {
		t.Fatalf("1.8V on a 3.0..3.6V part: want 1 under-voltage finding, got %v", under)
	}
}

func TestRailNominalPassesInRange(t *testing.T) {
	set := param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)}
	// Interior of the range and both closed bounds (> / <, not >= / <=).
	for _, rail := range []string{"+3V3", "+3V0", "+3V6"} {
		if fs := runRailNominal(t, supplyDesign(rail, false, "ACME-33"), set); len(fs) != 0 {
			t.Errorf("%s within 3.0..3.6V inclusive: want silent, got %v", rail, fs)
		}
	}
}

// TestRailNominalOneSidedRange proves a row that states only a max (many recommended
// tables print a ceiling with no explicit floor) checks the over side and never
// under-fires for want of a min.
func TestRailNominalOneSidedRange(t *testing.T) {
	maxOnly := ldoRecommendedSpec("ACME-33", 0, 3.6)
	maxOnly.Parameters[0].Value.Min = nil
	set := param.ParamSet{"ACME-33": maxOnly}
	if fs := runRailNominal(t, supplyDesign("+5V", false, "ACME-33"), set); len(fs) != 1 {
		t.Errorf("max-only row, 5V rail: want 1 over-voltage finding, got %v", fs)
	}
	if fs := runRailNominal(t, supplyDesign("+1V8", false, "ACME-33"), set); len(fs) != 0 {
		t.Errorf("max-only row, low rail: no floor to violate, want silent, got %v", fs)
	}
}

func TestRailNominalSkipsNotFalsePasses(t *testing.T) {
	good := param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)}

	if fs := runRailNominal(t, supplyDesign("+5V", false, ""), good); len(fs) != 0 {
		t.Errorf("no MPN: skip, got %v", fs)
	}
	if fs := runRailNominal(t, supplyDesign("+5V", false, "OTHER"), good); len(fs) != 0 {
		t.Errorf("unseeded MPN: skip, got %v", fs)
	}
	if fs := runRailNominal(t, supplyDesign("+5V", false, "ACME-33"), nil); len(fs) != 0 {
		t.Errorf("no seeded set: silent by construction, got %v", fs)
	}
	if fs := runRailNominal(t, supplyDesign("VBOOST", false, "ACME-33"), good); len(fs) != 0 {
		t.Errorf("rail name with no parseable nominal: skip, got %v", fs)
	}

	mut := func(name string, f func(*parampb.PartSpec)) {
		s := ldoRecommendedSpec("ACME-33", 3.0, 3.6)
		f(s)
		if fs := runRailNominal(t, supplyDesign("+5V", false, "ACME-33"), param.ParamSet{"ACME-33": s}); len(fs) != 0 {
			t.Errorf("%s: want skip, got %v", name, fs)
		}
	}
	mut("abs-max row, not recommended", func(s *parampb.PartSpec) {
		s.Parameters[0].LimitKind = parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX
	})
	mut("non-supply symbol", func(s *parampb.PartSpec) { s.Parameters[0].Symbol = "TJ" })
	mut("a unit the parameter layer does not recognize", func(s *parampb.PartSpec) {
		s.Parameters[0].Unit = "dBm"
	})
	mut("under-specified row", func(s *parampb.PartSpec) {
		s.Parameters[0].ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_PARTIAL
	})
	mut("text-only condition", func(s *parampb.PartSpec) {
		s.Parameters[0].Conditions = []*parampb.Condition{{Symbol: "TA", Raw: "over operating range"}}
	})
	// Two recommended supply rows: the pin-to-supply mapping is ambiguous, so the whole
	// part is skipped rather than risk a false over/under finding — even though +5V would
	// sit inside the second (VDDA 4.5..5.5) row.
	mut("ambiguous multi-supply", func(s *parampb.PartSpec) {
		f := func(v float64) *float64 { return &v }
		s.Parameters = append(s.Parameters, &parampb.Parameter{
			Name: "Analog supply", Symbol: "VDDA",
			LimitKind:         parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
			Value:             &parampb.RangeValue{Min: f(4.5), Max: f(5.5)},
			Unit:              "V",
			Conditions:        []*parampb.Condition{{Symbol: "TA", Eq: f(25), Unit: "C", Raw: "TA = 25C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
			Prov: &parampb.ParamProvenance{
				DocRef: "ds", Page: 6, TableOrFigure: "Recommended Operating Conditions",
				Method: "hand", Confidence: 1,
			},
		})
	})
}

// TestRailNominalReadsMillivoltRows (agni issue 148): 3000..3600 mV and 3.0..3.6 V are ONE
// recommended-operating row written two ways, and a +5V rail sits outside it either way. The millivolt
// spelling used to fail the extractor's unit gate, so the rule had no range to compare against and the
// item scored a PASS.
//
// This rule is the two-sided one, so the conversion has to carry BOTH bounds. A conversion that scaled
// only the max would leave a 3000V minimum in place and report the rail as under-volted instead.
func TestRailNominalReadsMillivoltRows(t *testing.T) {
	d := supplyDesign("+5V", false, "ACME-33")
	volts := runRailNominal(t, d, param.ParamSet{"ACME-33": ldoRecommendedSpec("ACME-33", 3.0, 3.6)})
	if len(volts) == 0 {
		t.Fatal("a +5V rail against a 3.0..3.6V recommended range must fire; the fixture no longer exercises the rule")
	}

	milli := ldoRecommendedSpec("ACME-33", 3000, 3600)
	milli.Parameters[0].Unit = "mV"
	got := runRailNominal(t, d, param.ParamSet{"ACME-33": milli})
	if len(got) != len(volts) {
		t.Fatalf("millivolt spelling produced %d findings, want the %d the volt spelling produces", len(got), len(volts))
	}
	if got[0].Message != volts[0].Message {
		t.Errorf("millivolt spelling reports:\n  %s\nwant the volt spelling's:\n  %s", got[0].Message, volts[0].Message)
	}
}

// TestRailNominalGating pins the catalog-level behavior: a param-prefixed read gates the
// rule to not-applicable without a seeded set, so a review item bound to it reads n/a
// (not a hollow pass) until params are supplied.
func TestRailNominalGating(t *testing.T) {
	if ok, reason := check.Available(railNominalOutOfRecommended, nil); ok || !strings.Contains(reason, "--params") {
		t.Errorf("rail-nominal must gate on the params layer at catalog level, got %v %q", ok, reason)
	}
}
