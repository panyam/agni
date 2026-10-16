package check

import (
	"context"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/ir"
)

// tupleRule fails one verdict keyed on a TWO-entity tuple whose finding names only the first, and
// passes another, which is the shape a finding cannot reconstruct its verdict from (agni issue 485).
func tupleRule(states bool) *Rule {
	return &Rule{
		Name:                "tuple",
		Severity:            "warning",
		StatesConsideredSet: states,
		Eval: func(context.Context, Model) []Verdict {
			return []Verdict{
				{Outcome: Pass, Subjects: []Entity{{Kind: KindNet, Ref: "A"}, {Kind: KindNet, Ref: "C"}}, Witness: &Witness{Statement: "apart"}},
				{Outcome: Fail, Subjects: []Entity{{Kind: KindNet, Ref: "A"}, {Kind: KindNet, Ref: "B"}}, Witness: &Witness{Statement: "too close"},
					Finding: &Finding{Subject: Entity{Kind: KindNet, Ref: "A"}, Message: "A is too close to B"}},
			}
		},
	}
}

// A finding from a rule that states its considered set names the verdict it came from, and that id is
// one of the run's own verdicts.
func TestAFindingNamesItsVerdict(t *testing.T) {
	fs, vs, err := RunAll(context.Background(), NewModel(&ir.Design{}), []*Rule{tupleRule(true)})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	want := "tuple:(net:A,net:B)"
	if len(fs[0].VerdictIDs) != 1 || fs[0].VerdictIDs[0] != want {
		t.Fatalf("verdict ids = %v, want [%s]", fs[0].VerdictIDs, want)
	}
	found := false
	for _, v := range vs {
		if VerdictID(v) == want {
			found = true
		}
	}
	if !found {
		t.Errorf("the finding names %s, which is not among the run's verdicts", want)
	}
	// Run alone stamps the same id, since it is the path a review takes.
	if only, err := Run(context.Background(), NewModel(&ir.Design{}), []*Rule{tupleRule(true)}); err != nil || len(only) != 1 || len(only[0].VerdictIDs) != 1 || only[0].VerdictIDs[0] != want {
		t.Errorf("Run's finding = %+v (err %v), want one carrying %s", only, err, want)
	}
}

// A rule that states no considered set publishes no verdicts, so its findings name none.
func TestAFindingWithoutAConsideredSetNamesNoVerdict(t *testing.T) {
	fs, err := Run(context.Background(), NewModel(&ir.Design{}), []*Rule{tupleRule(false)})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("findings = %d, want 1", len(fs))
	}
	if len(fs[0].VerdictIDs) != 0 {
		t.Errorf("verdict ids = %v, want none for a rule with no considered set", fs[0].VerdictIDs)
	}
}
