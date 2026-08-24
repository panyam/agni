package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// THE HONESTY GUARD for the part-converted catalog.
//
// Every rule now returns verdicts, so an unconverted rule's output is a list of Fail verdicts, which
// is structurally identical to a considered set whose every subject happened to fail. Nothing in the
// data distinguishes them. If RunVerdicts collected both, `check --verdicts` would present ~55 rules'
// failure lists as though they were coverage, which is a stronger claim than the run has earned and
// the exact silence-reads-as-data mistake the verdict work exists to remove.
//
// So the claim is a DECLARATION (StatesConsideredSet), and this pins that RunVerdicts honours it.
func TestRunVerdictsExcludesRulesThatOnlyReportFailures(t *testing.T) {
	failing := func(m Model) []Finding {
		return []Finding{{Subject: NetNameEntity("SIG"), Message: "bad"}}
	}
	unconverted := &Rule{Name: "unconverted", Eval: FailuresOnly(failing)}
	converted := &Rule{
		Name:                "converted",
		StatesConsideredSet: true,
		Eval: func(m Model) []Verdict {
			return []Verdict{
				{Subjects: []Entity{NetNameEntity("CLEAN")}, Outcome: Pass, Witness: &Witness{Statement: "fine"}},
				{Subjects: []Entity{NetNameEntity("SIG")}, Outcome: Fail, Finding: &Finding{Subject: NetNameEntity("SIG"), Message: "bad"}},
			}
		},
	}

	m := NewModel(&ir.Design{})
	rules := []*Rule{unconverted, converted}

	// Both rules reach the FINDINGS contract: not stating a considered set is not a reason to drop a
	// violation, and this is what keeps the migration safe for every existing consumer.
	if got := len(Run(m, rules)); got != 2 {
		t.Errorf("both rules must contribute findings, got %d", got)
	}

	// Only the declaring rule reaches the VERDICT contract.
	vs := RunVerdicts(m, rules)
	for _, v := range vs {
		if v.Rule == "unconverted" {
			t.Errorf("a failures-only rule must not be reported as a considered set, got %+v", v)
		}
	}
	if len(vs) != 2 {
		t.Fatalf("the converted rule's two verdicts must survive, got %d: %+v", len(vs), vs)
	}
	// And the pass survives, which is the whole point of the verdict table.
	var passes int
	for _, v := range vs {
		if v.Outcome == Pass {
			passes++
		}
	}
	if passes != 1 {
		t.Errorf("want the one Pass verdict, got %d", passes)
	}
}

// FailuresOnly must not invent evidence. A Witness manufactured by restating the message would make
// an unconverted rule look converted to anything reading verdicts, which is the decoration
// build/evidence.md warns about.
func TestFailuresOnlyCarriesNoWitness(t *testing.T) {
	eval := FailuresOnly(func(m Model) []Finding {
		return []Finding{{Subject: NetNameEntity("SIG"), Message: "bad"}}
	})
	for _, v := range eval(NewModel(&ir.Design{})) {
		if v.Witness != nil {
			t.Errorf("an unconverted rule must not carry a witness, got %+v", v.Witness)
		}
		if v.Finding == nil {
			t.Error("a Fail verdict must still carry the finding it came from")
		}
	}
}
