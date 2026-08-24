package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Tests for the considered set (stage 2). The failure mode here is the mirror of stage 1's: stage 1
// asked whether a pass carries evidence, this asks whether a terminal the rule could NOT answer for
// is reported at all. A rule that silently drops such a terminal produces output indistinguishable
// from one that judged it clean, which is the coverage claim build/evidence.md warns about.

// twoRowAbsMaxSpec puts TWO absolute-maximum rows on VCCB, as a datasheet stating one limit per
// condition does. The caller gives both maxima and their order, so a test can assert the answer is
// the tighter row rather than whichever was enumerated first.
func twoRowAbsMaxSpec(first, second float64) *parampb.PartSpec {
	f := func(v float64) *float64 { return &v }
	spec := absMaxSpec("ACME-XLAT", f(first))
	prov := &parampb.ParamProvenance{DocRef: "ds", Page: 5, TableOrFigure: "Absolute Maximum Ratings", Method: "hand", Confidence: 1}
	spec.Parameters = append(spec.Parameters, &parampb.Parameter{
		Name: "VCCB supply voltage (second condition)", Symbol: "VCCB2",
		LimitKind:         parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
		Value:             &parampb.RangeValue{Min: f(-0.5), Max: f(second)},
		Unit:              "V",
		Conditions:        []*parampb.Condition{{Symbol: "TA", Min: f(-40), Max: f(125), Unit: "C"}},
		ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
		PinRefs:           []string{"vccb"}, Prov: prov,
	})
	return spec
}

func twoRowModel(t *testing.T, first, second float64) check.Model {
	t.Helper()
	return check.NewModelWithParams(xlatDesign("ACME-XLAT", "+3V3", "+5V"), nil,
		param.ParamSet{"ACME-XLAT": twoRowAbsMaxSpec(first, second)})
}

// ONE TERMINAL, ONE VERDICT. A verdict is pin-scoped, so a pin carrying several rows of one kind
// must still answer once. The row-shaped enumerator this replaces returned one verdict per ROW, so
// "what does this rule say about U1.14" had two answers that disagreed about the limit.
func TestOneTerminalProducesOneVerdict(t *testing.T) {
	vs := pinAbsMaxVerdicts(twoRowModel(t, 6.5, 5.0))

	seen := map[string]int{}
	for _, v := range vs {
		seen[v.Subjects[0].Kind+":"+check.EntityRef(v.Subjects[0])+"."+v.Subjects[0].Pin]++
	}
	for key, n := range seen {
		if n > 1 {
			t.Errorf("%s produced %d verdicts; a pin-scoped verdict must answer once per terminal", key, n)
		}
	}
	// Positive control: the fixture really does bind two rows to one pin, so the assertion above had
	// something to catch.
	if len(twoRowAbsMaxSpec(6.5, 5.0).GetParameters()) != 3 {
		t.Fatal("fixture must carry two VCCB rows or this test proves nothing")
	}
}

// THE BINDING ROW IS THE TIGHTEST, NOT THE FIRST. Asserted in both orders, so an implementation
// that took rows[0] fails exactly one of them. That is the red-check built into the test rather
// than performed beside it.
func TestVerdictRestsOnTheMostRestrictiveRow(t *testing.T) {
	for _, c := range []struct {
		name          string
		first, second float64
	}{
		{"tighter row second", 6.5, 5.0},
		{"tighter row first", 5.0, 6.5},
	} {
		v := verdictFor(t, pinAbsMaxVerdicts(twoRowModel(t, c.first, c.second)), "14")
		if v.Witness == nil {
			t.Fatalf("%s: a pass must carry its evidence", c.name)
		}
		if !strings.Contains(v.Witness.Statement, "5") || strings.Contains(v.Witness.Statement, "6.5") {
			t.Errorf("%s: +5V is governed by the 5.0V row, not the 6.5V one; witness says %q",
				c.name, v.Witness.Statement)
		}
	}
}

// A row the design VIOLATES governs over one it clears, whatever the margins. Without this, a
// terminal over its tightest limit could report a pass against a looser row and be believed.
func TestAViolatedRowWinsOverAClearedOne(t *testing.T) {
	v := verdictFor(t, pinAbsMaxVerdicts(twoRowModel(t, 6.5, 4.0)), "14")

	if v.Outcome != check.Fail {
		t.Fatalf("+5V exceeds the 4.0V row, so the terminal is not fine; got %s (%v)", v.Outcome, v.Witness)
	}
	if v.Finding == nil {
		t.Error("a failing verdict must carry the finding the check path reports")
	}
}

// THE CONSIDERED SET IS TOTAL. Every supply terminal in scope answers, including the ones the rule
// cannot judge. Here no recommended-operating row is bound to either terminal, which used to
// produce an empty result identical to a design with nothing wrong.
func TestUnjudgeableTerminalIsReportedNotDropped(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	// absMaxSpec binds ABSOLUTE_MAX rows only, so the recommended-range rule can judge neither pin.
	vs := pinRecommendedVerdicts(absMaxModel(t, f(6.5), "+3V3", "+5V"))

	if len(vs) == 0 {
		t.Fatal("both supply terminals are in scope and neither has a recommended row; " +
			"reporting nothing is the silent drop this closes")
	}
	for _, v := range vs {
		if v.Outcome != check.NotConsidered {
			t.Errorf("pin %s: want %s, got %s", v.Subjects[0].Pin, check.NotConsidered, v.Outcome)
		}
		if v.Reason == "" {
			t.Errorf("pin %s: NotConsidered without a reason is the same silence in a new shape", v.Subjects[0].Pin)
		}
		if v.Witness != nil {
			t.Errorf("pin %s: nothing was compared, so there is nothing to witness", v.Subjects[0].Pin)
		}
	}
	// The reason has to name what was missing, or it cannot be acted on.
	if !strings.Contains(vs[0].Reason, "recommended") {
		t.Errorf("reason must name the row kind that was absent, got %q", vs[0].Reason)
	}
}

// A NotConsidered verdict must not reach the check path. The considered set is a REPORT; the
// findings contract is unchanged, and a terminal nobody could judge is not a defect.
func TestNotConsideredProjectsToNoFinding(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	m := absMaxModel(t, f(6.5), "+3V3", "+5V")

	vs := pinRecommendedVerdicts(m)
	if len(vs) == 0 {
		t.Fatal("fixture must produce NotConsidered verdicts or this proves nothing")
	}
	if got := check.VerdictsToFindings(vs); len(got) != 0 {
		t.Errorf("NotConsidered is not a defect, want 0 findings, got %d: %+v", len(got), got)
	}
	if got := pinOutOfRecommended.Findings(m); len(got) != 0 {
		t.Errorf("Eval must be unchanged by the considered set, want 0 findings, got %d", len(got))
	}
}
