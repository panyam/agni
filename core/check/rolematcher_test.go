package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestEveryRoleHasANameMatcher is step 2 of agni 692, and it is a TEST rather than a compiler feature
// because Go does not check switch exhaustiveness over an enum. nameMatcherFor's default returns a
// never-matches function, so a role added to the vocabulary and forgotten there would answer false for
// every net that skipped the ingestion stamp, silently.
//
// The fallback only fires for an unstamped net (a hand-authored IR), which is precisely why nothing
// louder would notice: a normal read stamps roles, so the gap would sit unexercised until someone hit
// it in a test fixture.
func TestEveryRoleHasANameMatcher(t *testing.T) {
	m := NewModel(&ir.Design{Nets: []*ir.Net{
		{Name: "12V_OUT"}, {Name: "GND"}, {Name: "12V_FB"},
		{Name: "12V_SW"}, {Name: "12V_MODE1"}, {Name: "12V_VDRV"},
	}}).(*irModel)

	for _, role := range classify.AllNetRoles() {
		matcher := m.nameMatcherFor(role)
		var matchedSomething bool
		for _, n := range m.Nets() {
			if matcher(n.Name) {
				matchedSomething = true
				break
			}
		}
		if !matchedSomething {
			t.Errorf("no name matches role %q: nameMatcherFor has no case for it, so an unstamped net can never carry it", classify.RoleToken(role))
		}
	}
}
