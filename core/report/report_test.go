package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

func verdict(rule, subject string, o check.Outcome) check.Verdict {
	v := check.Verdict{Rule: rule, Outcome: o, Kind: check.KindNet, Subject: subject}
	switch o {
	case check.Pass:
		v.Witness = &check.Witness{Statement: subject + " is fine"}
	case check.Fail:
		v.Finding = &check.Finding{Rule: rule, Kind: check.KindNet, Subject: subject, Message: subject + " is wrong"}
	}
	return v
}

func rules() []*check.Rule {
	return []*check.Rule{
		{Name: "converted", Severity: "error", Summary: "s", StatesConsideredSet: true},
		{Name: "legacy", Severity: "warning", Summary: "s"},
	}
}

// THE ORDERING JUDGMENT. A reader opening a report on a board with three problems and two thousand
// passes must meet the three problems, so a rule with something to act on sorts first and, inside a
// rule, so does the row.
func TestFailuresLead(t *testing.T) {
	r := Build(
		[]check.Verdict{
			verdict("converted", "A_PASSES", check.Pass),
			verdict("converted", "Z_FAILS", check.Fail),
		},
		[]check.Finding{{Rule: "converted", Kind: check.KindNet, Subject: "Z_FAILS", Message: "x"}},
		rules(), Report{},
	)
	if got := r.Rules[0].Rows[0].Subject; got != "Z_FAILS" {
		t.Errorf("the failing row must lead its rule, got %q first", got)
	}
	// Alphabetically A_PASSES would win; outcome beats name, deliberately.
	if r.Rules[0].Rows[1].Subject != "A_PASSES" {
		t.Errorf("rows = %+v", r.Rules[0].Rows)
	}
}

// A rule that states a considered set already carried its failures as verdicts. Counting the matching
// findings again would double them, which is why the merge is keyed on the RULE and not the finding.
func TestConvertedRuleDoesNotDoubleCountItsFailures(t *testing.T) {
	r := Build(
		[]check.Verdict{verdict("converted", "N", check.Fail)},
		[]check.Finding{{Rule: "converted", Kind: check.KindNet, Subject: "N", Message: "x"}},
		rules(), Report{},
	)
	if n := len(r.Rules[0].Rows); n != 1 {
		t.Errorf("want one row for one failure, got %d: %+v", n, r.Rules[0].Rows)
	}
	if r.Totals.Fail != 1 {
		t.Errorf("Fail total = %d, want 1", r.Totals.Fail)
	}
}

// THE HONESTY PROPERTY, carried into the report. A rule that reports violations without stating what
// it examined must be visibly that, or the report presents a failure list as coverage. This is the
// same claim StatesConsideredSet makes at the seam, and a report is where it would be most
// convincing and most wrong.
func TestFindingsOnlyRuleIsLabelledAndNotCountedAsCoverage(t *testing.T) {
	r := Build(
		nil,
		[]check.Finding{{Rule: "legacy", Kind: check.KindNet, Subject: "N", Message: "x"}},
		rules(), Report{},
	)
	if r.Totals.Considered != 0 {
		t.Errorf("a findings-only rule considered nothing we can claim, got %d", r.Totals.Considered)
	}
	if r.Totals.RulesFindingsOnly != 1 || r.Totals.RulesReporting != 0 {
		t.Errorf("totals misclassify the rule: %+v", r.Totals)
	}

	var buf bytes.Buffer
	if err := HTML(&buf, r); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "Absence here is not evidence of correctness") {
		t.Error("the rendered report must say that silence from this rule proves nothing")
	}
}

// Subjects and messages come out of design files the engine did not author, so they are untrusted
// text. A net named with a tag must not become one.
func TestSubjectsAreEscapedNotInjected(t *testing.T) {
	r := Build(
		[]check.Verdict{verdict("converted", `<script>alert(1)</script>`, check.Pass)},
		nil, rules(), Report{},
	)
	var buf bytes.Buffer
	if err := HTML(&buf, r); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "<script>alert(1)</script>") {
		t.Error("a net name was rendered as live markup")
	}
	if !strings.Contains(buf.String(), "&lt;script&gt;") {
		t.Error("the net name should still be readable, escaped")
	}
}

// A LINK IS A PROMISE. With no base and no mount there is nothing to promise, so the report says
// nothing rather than assembling a URL that resolves on nobody's server (issue 392).
func TestNoLinksWithoutABase(t *testing.T) {
	vs := []check.Verdict{verdict("converted", "N", check.Pass)}
	bare := Build(vs, nil, rules(), Report{})
	if got := bare.Rules[0].Rows[0].URL; got != "" {
		t.Errorf("want no URL without a base, got %q", got)
	}

	linked := Build(vs, nil, rules(), Report{URLBase: "http://h:1", MountPath: "m/d", ContentHash: "abc"})
	url := linked.Rules[0].Rows[0].URL
	for _, want := range []string{"http://h:1/designs/m/d/view", "verdict=", "hash=abc"} {
		if !strings.Contains(url, want) {
			t.Errorf("url %q missing %q", url, want)
		}
	}
}
