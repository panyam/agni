package check

import "testing"

// A pass must state the VALUE that decided it, not the syntax of the test. "claims >= 2 does not
// hold" reads identically on every passing subject a rule sees and tracks no fact, which
// build/evidence.md calls decoration; "claims is 1, not >= 2" is a claim about THIS subject that a
// reader can go and check against the design.
func TestSpecPassWitnessCarriesTheValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		let   map[string]Term
		where Expr
		want  string // the witness of the passing subject "OK"
	}{
		{
			// The idiom of duplicate-net-name and output-output-conflict: a Let-bound count
			// against a threshold. The threshold survives, so a reader can see what would fire.
			name:  "an ordering names the value and the threshold it missed",
			where: Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 9}},
			want:  "passes because net.pin_count is 2, not >= 9",
		},
		{
			// The idiom of label-alias-conflict and power-tap-conflict: a joined-names string
			// tested for non-emptiness. "!=" false means the two are EQUAL, so there is no unmet
			// threshold to report and naming one would contradict the value just stated.
			name:  "non-emptiness reads as the absence it is",
			let:   map[string]Term{"labels": Lit{V: ""}},
			where: Cmp{L: Var{Name: "labels"}, Op: "!=", R: Lit{V: ""}},
			want:  "passes because labels is empty",
		},
		{
			name:  "equality drops the operator",
			where: Cmp{L: Fact{Name: "net.pin_count"}, Op: "==", R: Lit{V: 9}},
			want:  "passes because net.pin_count is 2, not 9",
		},
		{
			name:  "a failed pattern states what the value actually was",
			where: Match{T: Fact{Name: "net.names"}, Pattern: "^X"},
			want:  `passes because net.names is "OK", so it does not match /^X/`,
		},
		{
			name:  "a failed membership states what the value actually was",
			where: In{T: Fact{Name: "net.names"}, Set: []string{"A", "B"}},
			want:  `passes because net.names is "OK", so it is not one of [A, B]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Spec{Over: "nets", Let: tc.let, Where: tc.where, Message: "stub"}
			got := ""
			for _, v := range s.Verdicts(NewModel(scopeDesign())) {
				if v.Subject == "OK" {
					if v.Outcome != Pass {
						t.Fatalf("want pass for OK, got %s", v.Outcome)
					}
					got = v.Witness.Statement
				}
			}
			if got != tc.want {
				t.Errorf("witness\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}
