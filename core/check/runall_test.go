package check

import (
	"context"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// countingRule is a rule that counts its evaluations and states a considered set.
func countingRule(name string, reads []string, n *int) *Rule {
	return &Rule{
		Name:                name,
		Reads:               reads,
		StatesConsideredSet: true,
		Eval: func(context.Context, Model) []Verdict {
			*n++
			return []Verdict{
				{Subjects: []Entity{NetNameEntity("OK")}, Outcome: Pass, Witness: &Witness{Statement: "fine"}},
				{Subjects: []Entity{NetNameEntity("BAD")}, Outcome: Fail, Finding: &Finding{Subject: NetNameEntity("BAD"), Message: "bad"}},
			}
		},
	}
}

// RunAll evaluates each rule once for both contracts (agni issue 810), where Run then RunVerdicts
// evaluated it twice.
func TestRunAllEvaluatesEachRuleOnce(t *testing.T) {
	n := 0
	r := countingRule("counted", nil, &n)
	fs, vs, err := RunAll(context.Background(), NewModel(&ir.Design{}), []*Rule{r})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("RunAll evaluated the rule %d times, want 1", n)
	}
	if len(fs) != 1 || len(vs) != 2 {
		t.Errorf("RunAll returned %d findings and %d verdicts, want 1 and 2", len(fs), len(vs))
	}
}

// On a design with an unresolved symbol, a connectivity rule's FINDINGS are the gate's single
// inconclusive, as Run has always reported, while its VERDICTS are still evaluated, as RunVerdicts
// has always done. RunAll keeps both, evaluating the rule once, and only because a verdict needs it.
func TestRunAllKeepsTheGateOnFindingsOnly(t *testing.T) {
	d := &ir.Design{InputDiagnostics: &ir.InputDiagnostics{UnresolvedSymbols: []*ir.UnresolvedSymbol{{Symref: "lib:Missing"}}}}
	n := 0
	r := countingRule("gated", []string{"pin.net"}, &n)
	if TierOf("pin.net") != TierConnectivity {
		t.Fatalf("pin.net is not a connectivity fact, so this rule would not be gated and the test proves nothing")
	}
	fs, vs, err := RunAll(context.Background(), NewModel(d), []*Rule{r})
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || !fs[0].Inconclusive {
		t.Errorf("findings = %+v, want the gate's one inconclusive", fs)
	}
	if len(vs) != 2 || n != 1 {
		t.Errorf("verdicts = %d after %d evaluations, want 2 after 1", len(vs), n)
	}
	n = 0
	if _, err := Run(context.Background(), NewModel(d), []*Rule{r}); err != nil || n != 0 {
		t.Errorf("Run evaluated a gated rule %d times, want 0 (it needs no verdicts)", n)
	}
}

// Memo builds a value once per model and key; a model that is not a Memoizer builds every time.
func TestMemoBuildsOncePerModelAndKey(t *testing.T) {
	type key struct{ s string }
	builds := 0
	build := func() any { builds++; return builds }
	m := NewModel(&ir.Design{})
	a, b := Memo(m, key{"x"}, build), Memo(m, key{"x"}, build)
	if a != b || builds != 1 {
		t.Errorf("two asks for one key built %d times and gave %v, %v", builds, a, b)
	}
	if Memo(m, key{"y"}, build) == a {
		t.Error("a second key shared the first key's value")
	}
	if Memo(NewModel(&ir.Design{}), key{"x"}, build) == a {
		t.Error("a second model shared the first model's value")
	}
	builds = 0
	var plain struct{ Model }
	Memo(plain, key{"x"}, build)
	Memo(plain, key{"x"}, build)
	if builds != 2 {
		t.Errorf("a model with no memo built %d times for two asks, want 2", builds)
	}
}
