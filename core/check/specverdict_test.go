package check

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func scopeDesign() *ir.Design {
	net := func(name string, conns ...string) *ir.Net {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			p := strings.SplitN(c, ".", 2)
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: p[0], PinRef: p[1]})
		}
		return n
	}
	return &ir.Design{Nets: []*ir.Net{
		net("STUB", "U1.1"),       // in scope (>=1 pin), violates (<2)
		net("OK", "U1.2", "U2.1"), // in scope, passes
		net("EMPTY"),              // OUT of scope: no connections at all
	}}
}

// THE PROPERTY Scope exists for. An out-of-scope element must produce NO verdict, because a pass
// there would claim the rule checked something it was never about. This is the difference between
// Scope and Where in one assertion: both are predicates, and only one of them can make a subject
// disappear.
func TestScopeExcludesRatherThanPasses(t *testing.T) {
	s := &Spec{
		Over:    "nets",
		Scope:   Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 1}},
		Where:   Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
		Message: "stub",
	}
	bySubject := map[string]Outcome{}
	for _, v := range s.Verdicts(NewModel(scopeDesign())) {
		bySubject[EntityRef(v.Subjects[0])] = v.Outcome
	}
	if _, present := bySubject["EMPTY"]; present {
		t.Errorf("an out-of-scope net must produce no verdict at all, got %s", bySubject["EMPTY"])
	}
	for subject, want := range map[string]Outcome{"STUB": Fail, "OK": Pass} {
		if bySubject[subject] != want {
			t.Errorf("%s: want %s, got %s", subject, want, bySubject[subject])
		}
	}
}

// Splitting a conjunction cannot change which subjects FAIL: old Where is exactly Scope AND Where.
// This is what makes the split a relabelling rather than a behaviour change, and it is the property
// the conformance fixtures check end to end.
func TestScopeAndWhereProjectToTheSameFindings(t *testing.T) {
	m := NewModel(scopeDesign())
	merged := &Spec{
		Over: "nets",
		Where: And{Xs: []Expr{
			Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 1}},
			Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
		}},
		Message: "stub",
	}
	split := &Spec{
		Over:    "nets",
		Scope:   Cmp{L: Fact{Name: "net.pin_count"}, Op: ">=", R: Lit{V: 1}},
		Where:   Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
		Message: "stub",
	}
	a, b := merged.Eval(m), split.Eval(m)
	if len(a) != len(b) || len(a) != 1 || a[0].Subject != b[0].Subject {
		t.Errorf("the split changed the failure set\n merged: %+v\n  split: %+v", a, b)
	}
}

// A pass must name the clause that decided it. A witness reading the same on every passing subject
// in the catalog would track no fact, which build/evidence.md calls decoration.
func TestSpecPassWitnessNamesTheDecidingClause(t *testing.T) {
	s := &Spec{
		Over: "nets",
		Where: And{Xs: []Expr{
			Cmp{L: Fact{Name: "net.pin_count"}, Op: "<", R: Lit{V: 2}},
			Match{T: Fact{Name: "net.names"}, Pattern: "^X"},
		}},
		Message: "stub",
	}
	var okWitness string
	for _, v := range s.Verdicts(NewModel(scopeDesign())) {
		if EntityRef(v.Subjects[0]) == "OK" {
			if v.Outcome != Pass {
				t.Fatalf("OK has two pins, want pass, got %s", v.Outcome)
			}
			okWitness = v.Witness.Statement
		}
	}
	// The FIRST false conjunct is the one that refutes the condition, and it is the pin count here,
	// not the name pattern.
	if !strings.Contains(okWitness, "net.pin_count") {
		t.Errorf("witness must name the clause that decided the pass, got %q", okWitness)
	}
	if strings.Contains(okWitness, "net.names") {
		t.Errorf("witness names a clause that did not decide anything: %q", okWitness)
	}
}
