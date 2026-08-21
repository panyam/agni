package builtin

import (
	"reflect"
	"testing"

	"github.com/panyam/agni/core/check"
)

// TestVerdictParity pins WHICH outcomes reach the findings contract.
//
// Its original job is gone and that is a good thing: Eval and Findings were two hand-written bodies
// that could disagree, and now Findings IS VerdictsToFindings(Eval), so per-rule drift is not a state
// a rule can be in. What is left is the projection RULE itself, which is a real decision with real
// consequences (an Inconclusive verdict must still reach a reviewer; a Pass must not).
//
// The expectation is therefore still rebuilt HERE rather than by calling VerdictsToFindings, and the
// reason is sharper than before: Findings() literally is that call, so comparing the two would run
// one function against itself and pass for any projection, broken ones included. (It did. This test
// asserted nothing until a red-check caught it.) Restating the rule independently means a change to
// what projects has to be made deliberately in two places instead of silently in one.
func TestVerdictParity(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    check.Model
	}{
		{"ruleFixture", check.NewModel(ruleFixture())},
		{"parityFixture", check.NewModel(specParityFixture())},
	} {
		var converted int
		for _, r := range rulesByName() {
			if !r.StatesConsideredSet {
				continue
			}
			converted++
			// The expectation is rebuilt HERE rather than by calling VerdictsToFindings, because a
			// converted rule's Eval already IS that call: comparing the two would run one function
			// against itself and pass for any projection, broken ones included. (It did. This test
			// asserted nothing until a red-check caught it.) Restating the rule independently means
			// a change to what projects, such as Inconclusive gaining a finding, has to be made
			// deliberately in both places instead of silently in one.
			want := []check.Finding{}
			for _, v := range r.Eval(tc.m) {
				if v.Outcome == check.Fail && v.Finding != nil {
					want = append(want, *v.Finding)
				}
			}
			if got := r.Findings(tc.m); !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s: the findings projection and the failing verdicts disagree\n got: %+v\n want: %+v",
					tc.name, r.Name, got, want)
			}
		}
		// Positive control: a test that iterates zero converted rules passes while proving nothing,
		// which is precisely the shape this catalog treats as the expensive failure.
		if converted == 0 {
			t.Fatalf("%s: no rule sets EvalVerdicts, so this test asserted nothing", tc.name)
		}
	}
}

// A rule that states a considered set must answer about SOMETHING on a design its subjects appear
// in. This is the guard against a conversion that declares StatesConsideredSet over a body returning
// nil: parity above would hold (nil projects to no findings, and the rule finds none either), and the
// considered set would be silently empty.
func TestConvertedRulesConsiderSomething(t *testing.T) {
	m := check.NewModel(ruleFixture())
	for _, r := range rulesByName() {
		if !r.StatesConsideredSet {
			continue
		}
		// i2c-pull-up's subjects are the fixture's SDA/SCL nets; the datasheet rules need seeded
		// pin-bound params the plain fixture has none of, so only the connectivity rule can answer here.
		if r.Name != "i2c-pull-up" {
			continue
		}
		if got := r.Findings(m); len(got) == 0 {
			t.Errorf("%s: the fixture carries this rule's subjects, so an empty considered set means "+
				"the conversion reports nothing rather than reporting a pass", r.Name)
		}
	}
}

// RunVerdicts is the seam that makes a verdict reachable at all: before it, every verdict died
// inside the function that built it and no caller outside these three rules could obtain one.
func TestRunVerdictsCollectsAcrossRules(t *testing.T) {
	vs := check.RunVerdicts(check.NewModel(ruleFixture()), rules)
	if len(vs) == 0 {
		t.Fatal("no verdicts collected; the considered set is unreachable again")
	}
	for _, v := range vs {
		if v.Rule == "" {
			t.Errorf("verdict about %s/%s carries no rule identity", v.Kind, v.Subject)
		}
		if v.Outcome == "" {
			t.Errorf("%s: verdict about %s has no outcome", v.Rule, v.Subject)
		}
	}
	// Ordering matches Run so a verdict table and a findings table read down the same axis.
	for i := 1; i < len(vs); i++ {
		a, b := vs[i-1], vs[i]
		if a.Rule > b.Rule || (a.Rule == b.Rule && a.Subject > b.Subject) {
			t.Errorf("verdicts out of order at %d: %s/%s before %s/%s",
				i, a.Rule, a.Subject, b.Rule, b.Subject)
		}
	}
}
