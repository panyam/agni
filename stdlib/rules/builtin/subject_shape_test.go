package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

// A rule's subject ARITY is fixed, and a person needs a declared shape to construct an id without
// running the check first.
//
// The undeclared case is the ordinary one and means a 1-tuple. That is checked too, and that half
// catches a mistake. A rule that grew a second subject element without saying so would produce ids
// nothing can index, and nothing else in the build would notice.
func TestSubjectShapeHolds(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    check.Model
	}{
		{"ruleFixture", check.NewModel(ruleFixture())},
		{"parityFixture", check.NewModel(specParityFixture())},
	} {
		for _, r := range rulesByName() {
			for _, v := range r.Eval(context.Background(), tc.m) {
				if len(v.Subjects) == 0 {
					t.Errorf("%s/%s: a verdict with no subject has no identity", tc.name, r.Name)
					continue
				}
				want := r.SubjectShape
				if len(want) == 0 {
					if len(v.Subjects) != 1 {
						t.Errorf("%s/%s: declares no SubjectShape, so its verdicts must name one entity; got %d (%s)",
							tc.name, r.Name, len(v.Subjects), check.SubjectRefs(v))
					}
					continue
				}
				if len(v.Subjects) != len(want) {
					t.Errorf("%s/%s: declares a %d-tuple, emitted %d (%s)",
						tc.name, r.Name, len(want), len(v.Subjects), check.SubjectRefs(v))
					continue
				}
				for i, e := range v.Subjects {
					if e.Kind != want[i] {
						t.Errorf("%s/%s: element %d is %q, declared %q", tc.name, r.Name, i, e.Kind, want[i])
					}
				}
			}
		}
	}
}

// This ties the two subject shapes together. A Verdict names every entity the rule quantified over, and
// a Finding names the ONE a reader has to change. The second must be one of the first.
//
// Without this they are free to drift, and the drift is invisible. A finding pointing at a part the
// verdict never looked at reads perfectly well and sends the reader somewhere the rule made no claim
// about. Kind may legitimately differ, since a pin-scoped verdict carries a part-scoped finding, so
// the check is on the REF.
func TestFindingSubjectComesFromTheVerdictsTuple(t *testing.T) {
	var checked int
	for _, tc := range []check.Model{check.NewModel(ruleFixture()), check.NewModel(specParityFixture())} {
		for _, r := range rulesByName() {
			for _, v := range r.Eval(context.Background(), tc) {
				if v.Finding == nil {
					continue
				}
				checked++
				var refs []string
				for _, e := range v.Subjects {
					refs = append(refs, e.Ref)
				}
				found := false
				for _, ref := range refs {
					if ref == v.Finding.Subject.Ref {
						found = true
					}
				}
				if !found {
					t.Errorf("%s: finding names %q, which is not among the verdict's subjects (%s)",
						r.Name, v.Finding.Subject.Ref, strings.Join(refs, ", "))
				}
			}
		}
	}
	// The positive control. A run with no failing verdict asserts nothing, and this catalog treats
	// that shape as the expensive failure.
	if checked == 0 {
		t.Fatal("no verdict carried a finding, so this test compared nothing")
	}
}

// Every verdict a run emits must answer to its own name. The relation-shaped rules once gave two
// answers under one id, and this test guards the whole catalog against that.
func TestVerdictIDsAreUniqueWithinARun(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    check.Model
	}{
		{"ruleFixture", check.NewModel(ruleFixture())},
		{"parityFixture", check.NewModel(specParityFixture())},
	} {
		seen := map[string]string{}
		for _, v := range check.RunVerdictsBackground(tc.m, rules) {
			id := check.VerdictID(v)
			if prev, dup := seen[id]; dup {
				t.Errorf("%s: id %q names two verdicts (%q and %q)", tc.name, id, prev, v.Witness.Statement)
			}
			seen[id] = check.SubjectRefs(v)
		}
	}
}
