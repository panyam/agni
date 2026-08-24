package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Tests for the proof-on-pass prototype. The failure mode these are written against is the one
// build/evidence.md warns about: "a witness is present on a pass" is an assertion that cannot fail,
// because a rule that hardcoded a witness would satisfy it. The property asserted here instead is
// that the witness TRACKS THE FACT, so changing the seeded limit changes what the witness says and
// removing the limit removes the pass.

// absMaxSpec builds the two-supply translator with VCCB's absolute maximum set by the caller, so a
// test can move the limit and watch the witness follow. A nil max omits the bound entirely, which is
// the datasheet row that states no maximum at all.
func absMaxSpec(mpn string, vccbMax *float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	prov := func() *parampb.ParamProvenance {
		return &parampb.ParamProvenance{DocRef: "ds", Page: 5, TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 1}
	}
	pin := func(id, name string) *parampb.Pin {
		return &parampb.Pin{Id: id, Name: name, Function: parampb.PinFunction_PIN_FUNCTION_POWER_INPUT, Prov: prov()}
	}
	row := func(sym string, kind parampb.LimitKind, v *parampb.RangeValue, ref string) *parampb.Parameter {
		return &parampb.Parameter{
			Name: sym + " supply voltage", Symbol: sym, LimitKind: kind, Value: v, Unit: "V",
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
			row("VCCA", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
				&parampb.RangeValue{Min: f(-0.5), Max: f(4.6)}, "vcca"),
			row("VCCB", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
				&parampb.RangeValue{Min: f(-0.5), Max: vccbMax}, "vccb"),
		},
	}
}

func absMaxModel(t *testing.T, vccbMax *float64, netA, netB string) check.Model {
	t.Helper()
	return check.NewModelWithParams(xlatDesign("ACME-XLAT", netA, netB), nil,
		param.ParamSet{"ACME-XLAT": absMaxSpec("ACME-XLAT", vccbMax)})
}

// verdictFor picks the verdict about one pin designator, so a test states which terminal it means.
func verdictFor(t *testing.T, vs []check.Verdict, pin string) check.Verdict {
	t.Helper()
	for _, v := range vs {
		if v.Subjects[0].Pin == pin {
			return v
		}
	}
	t.Fatalf("no verdict for pin %s in %d verdicts", pin, len(vs))
	return check.Verdict{}
}

// THE PROPERTY. A pass must name the limit it passed against, and moving that limit must move what
// the witness says. A witness that did not read the datasheet would pass both halves of this with
// the same string and fail the comparison.
func TestAbsMaxWitnessTracksTheSeededLimit(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	loose := verdictFor(t, pinAbsMaxVerdicts(absMaxModel(t, f(6.5), "+3V3", "+5V")), "14")
	tight := verdictFor(t, pinAbsMaxVerdicts(absMaxModel(t, f(5.5), "+3V3", "+5V")), "14")

	for _, c := range []struct {
		name string
		v    check.Verdict
		want string
	}{{"6.5V limit", loose, "6.5"}, {"5.5V limit", tight, "5.5"}} {
		if c.v.Outcome != check.Pass {
			t.Fatalf("%s: +5V is under it, want pass, got %s", c.name, c.v.Outcome)
		}
		if c.v.Witness == nil {
			t.Fatalf("%s: a pass must carry its evidence", c.name)
		}
		if !strings.Contains(c.v.Witness.Statement, c.want) {
			t.Errorf("%s: witness must state the limit it passed against, want %q in %q",
				c.name, c.want, c.v.Witness.Statement)
		}
	}
	if loose.Witness.Statement == tight.Witness.Statement {
		t.Errorf("witness does not track the datasheet: identical statement %q for a 6.5V and a 5.5V limit",
			loose.Witness.Statement)
	}
}

// THE SPLIT THIS WORK EXISTS FOR. A row stating no maximum constrains nothing, so it must not
// produce a pass. Before the verdict type both this and a genuine "comfortably under the limit" took
// the same silent `return`, which is why a design could read clean against a datasheet that said
// nothing about the terminal.
func TestAbsMaxWithNoStatedMaximumIsNotAPass(t *testing.T) {
	v := verdictFor(t, pinAbsMaxVerdicts(absMaxModel(t, nil, "+3V3", "+5V")), "14")

	if v.Outcome == check.Pass {
		t.Fatal("a row stating no maximum checked nothing; reporting pass is the false pass this work removes")
	}
	if v.Outcome != check.NoLimit {
		t.Errorf("want %s, got %s", check.NoLimit, v.Outcome)
	}
	if v.Witness != nil {
		t.Errorf("NoLimit rests on nothing, so it carries no witness, got %+v", v.Witness)
	}
}

// A pass must cite the document, or it is an assertion rather than evidence. This is the review
// meeting flow: "you say it is fine, against what?"
func TestPassingWitnessCarriesItsCitation(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	v := verdictFor(t, pinAbsMaxVerdicts(absMaxModel(t, f(6.5), "+3V3", "+5V")), "14")

	if len(v.Witness.Datasheet) == 0 {
		t.Fatal("a passing witness must carry the citation it rests on")
	}
	if got := v.Witness.Render(); !strings.Contains(got, "ACME-XLAT Rev A") || !strings.Contains(got, "p.5") {
		t.Errorf("rendered witness must name the document and page, got %q", got)
	}
}

// The two-sided case: a recommended range has to be reported as a range, not as one bound.
func TestRecommendedWitnessStatesBothBounds(t *testing.T) {
	v := verdictFor(t, pinRecommendedVerdicts(xlatModel(t, "+3V3", "+5V")), "14")

	if v.Outcome != check.Pass {
		t.Fatalf("+5V is inside VCCB's 1.65 to 5.5 range, want pass, got %s", v.Outcome)
	}
	for _, want := range []string{"1.65", "5.5"} {
		if !strings.Contains(v.Witness.Statement, want) {
			t.Errorf("witness must state both bounds, want %q in %q", want, v.Witness.Statement)
		}
	}
}

// PROJECTION PARITY. The whole refactor rests on Eval still returning exactly the failures it
// always did, so the verdict list and the finding list must agree about what failed.
func TestVerdictsProjectToTheSameFindings(t *testing.T) {
	m := xlatModel(t, "+5V", "+5V") // over VCCA's limits, inside VCCB's

	for _, c := range []struct {
		name     string
		verdicts []check.Verdict
		findings []check.Finding
	}{
		{"abs-max", pinAbsMaxVerdicts(m), pinExceedsAbsMax.Findings(m)},
		{"recommended", pinRecommendedVerdicts(m), pinOutOfRecommended.Findings(m)},
	} {
		var failed int
		for _, v := range c.verdicts {
			if v.Outcome == check.Fail {
				failed++
			}
		}
		if failed != len(c.findings) {
			t.Errorf("%s: %d failing verdicts but %d findings; the projection dropped or invented one",
				c.name, failed, len(c.findings))
		}
		if failed == 0 {
			t.Errorf("%s: fixture must produce at least one failure or this proves nothing", c.name)
		}
	}
}
