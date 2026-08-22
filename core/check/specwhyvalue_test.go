package check

import (
	"strings"
	"testing"
)

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
				if EntityRef(v.Subjects[0]) == "OK" {
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

// The mirror of the test above, on the branches PR 401 did not reach (agni issue 412).
//
// A false Not means its operand HOLDS, and the operand is where the value lives. Rendering the
// operand's syntax instead produced a statement identical on every passing subject: on the tutorial
// board's naming rule, 13 consecutive rows reading "net.name_leaf matches /^(PMIC|...)/ holds".
func TestSpecPassWitnessExplainsWhyANegatedTestHeld(t *testing.T) {
	for _, tc := range []struct {
		name  string
		let   map[string]Term
		where Expr
		want  string
	}{
		{
			// The naming-rule shape: an allow-list is an Or of patterns, and the one that MATCHED is
			// what a reader wants named, along with the name that matched it.
			name:  "a negated allow-list names the value and the pattern it satisfied",
			where: Not{X: Or{Xs: []Expr{Match{T: Fact{Name: "net.names"}, Pattern: "^Z"}, Match{T: Fact{Name: "net.names"}, Pattern: "^O"}}}},
			want:  `passes because net.names is "OK", which matches /^O/`,
		},
		{
			name:  "a negated ordering names the value and the bound it cleared",
			where: Not{X: Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 2}}},
			want:  "passes because net.pin_count is 2, which is >= 2",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := passWitnessFor(t, tc.let, tc.where, "OK"); got != tc.want {
				t.Errorf("witness\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// The FFI hands back a bare bool and has discarded what it looked at, but its ARGUMENT is a term the
// interpreter can read. That is what lets 12 of the catalog's 14 IsTrue sites state a value without
// the SpecFunc contract changing at all.
//
// The subject here is GND, because the clause has to HOLD for the rule to pass: a spec fires where its
// Where is true, so a negated call only explains a pass when the call accepted.
func TestSpecPassWitnessNamesWhatACallAccepted(t *testing.T) {
	where := Not{X: IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{Name: "net.names"}}}}}
	want := `passes because net.names is "GND", which ground_name accepts`
	if got := passWitnessFor(t, nil, where, "GND"); got != want {
		t.Errorf("witness\n got: %q\nwant: %q", got, want)
	}
}

// The residue, pinned so it stays a recorded gap rather than becoming an oversight. An argument-less
// call takes the whole scope and returns a verdict, so there is nothing here to read and nothing
// honest to print beyond the fact that it accepted. Closing it needs a SpecFunc able to hand back what
// it observed, which cap-voltage needs for the same reason (OUT_OF_SCOPE.md).
func TestSpecPassWitnessAdmitsAnArgumentLessCallStatesNoValue(t *testing.T) {
	where := Not{X: IsTrue{T: Call{Fn: "intentionally_unconnected"}}}
	want := "passes because intentionally_unconnected accepts this subject, which states no value it read"
	if got := passWitnessFor(t, nil, where, "NC_STUB"); got != want {
		t.Errorf("witness\n got: %q\nwant: %q", got, want)
	}
}

// An absent match must say how much it looked at. "no connection is a no-connect" is true of a net
// with four pins and of a net with none, and only one of those is evidence.
func TestSpecPassWitnessCountsWhatAnAbsenceExamined(t *testing.T) {
	where := ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{Name: "pin.electrical_type"}, Op: "==", R: Lit{V: "no_connect"}}}
	got := passWitnessFor(t, nil, where, "OK")
	if !strings.Contains(got, "(2 examined)") {
		t.Errorf("the statement must say how many members it looked at, got %q", got)
	}
}

// passWitnessFor runs a one-clause spec over the scope fixture and returns the named subject's
// passing witness.
func passWitnessFor(t *testing.T, let map[string]Term, where Expr, subject string) string {
	t.Helper()
	s := &Spec{Over: "nets", Let: let, Where: where, Message: "stub"}
	for _, v := range s.Verdicts(NewModel(scopeDesign())) {
		if EntityRef(v.Subjects[0]) != subject {
			continue
		}
		if v.Outcome != Pass {
			t.Fatalf("want pass for %s, got %s", subject, v.Outcome)
		}
		return v.Witness.Statement
	}
	t.Fatalf("no verdict for %s", subject)
	return ""
}

// The property behind every case above, asserted over the whole spec catalog rather than one clause
// at a time: a pass statement must be ABOUT its subject.
//
// The test is not "no two statements are identical", which would be wrong. `labels is empty` on
// fifteen nets is fifteen honest readings of the same fact, and a rule whose value genuinely repeats
// should say so. What must not happen is a statement that CANNOT differ, because it renders the rule's
// syntax and never reads the design. This checks the weaker, correct thing: every passing statement
// either carries a value the subject supplied, or admits it read none.
//
// It is the guard that would have caught the original defect. `net.name_leaf matches /^(PMIC|...)/
// holds` passes any uniqueness test with one subject and fails this one with any.
func TestEverySpecPassStatementReadsSomething(t *testing.T) {
	// A statement is about its subject when it names a value ("... is X"), counts what it examined,
	// or says plainly that the FFI it rests on states no value. Anything else is the rule's syntax.
	statesSomething := func(s string) bool {
		return strings.Contains(s, " is ") || strings.Contains(s, " examined)") ||
			strings.Contains(s, "states no value it read")
	}
	specs := []Expr{
		Not{X: Or{Xs: []Expr{Match{T: Fact{Name: "net.names"}, Pattern: "^Z"}, Match{T: Fact{Name: "net.names"}, Pattern: "^O"}}}},
		Not{X: Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 2}}},
		ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{Name: "pin.electrical_type"}, Op: "==", R: Lit{V: "no_connect"}}},
		Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 9}},
		Match{T: Fact{Name: "net.names"}, Pattern: "^X"},
		In{T: Fact{Name: "net.names"}, Set: []string{"A", "B"}},
	}
	var checked int
	for _, where := range specs {
		s := &Spec{Over: "nets", Where: where, Message: "stub"}
		for _, v := range s.Verdicts(NewModel(scopeDesign())) {
			if v.Outcome != Pass {
				continue
			}
			checked++
			if !statesSomething(v.Witness.Statement) {
				t.Errorf("statement reads the rule's syntax rather than the subject: %q\n  (clause %s)",
					v.Witness.Statement, renderExpr(where))
			}
		}
	}
	// Positive control: a run producing no passing verdict asserts nothing.
	if checked == 0 {
		t.Fatal("no passing verdict was examined, so this test compared nothing")
	}
}
