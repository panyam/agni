// package check_test (black-box): imports check the way an out-of-module overlay does, so it
// proves the WS12-004 acceptance — an external caller registers a Go rule suite and it flows
// into the catalog the CLI and serve wire (DefaultCatalog / CatalogWith), namespaced and
// runnable.
package check_test

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// widgetPresent fires one finding on any design that has a component — a stand-in for a
// house-style rule an overlay would author.
func widgetRule() *check.Rule {
	return &check.Rule{
		Name:     "widget-present",
		Severity: "info",
		Summary:  "an external house-style rule",
		Reads:    []string{"component.ref_des"},
		Tags:     map[string]string{check.KeyCategory: "house-style"},
		Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, c := range m.Components() {
				out = append(out, check.Finding{Kind: check.KindComponent, Subject: c.RefDes, Message: "seen by the overlay rule"})
			}
			return out
		}),
	}
}

// TestExternalRuleRegistrationEndToEnd registers a source from outside the package and asserts
// it reaches the catalog the engine surfaces wire (DefaultCatalog and CatalogWith), namespaced
// with its source tag, and actually runs to produce a finding.
func TestExternalRuleRegistrationEndToEnd(t *testing.T) {
	if check.DefaultCatalog().Lookup("acme/widget-present") != nil {
		t.Fatal("acme/widget-present already present; pick an unused synthetic source")
	}

	check.RegisterSource(check.NewSource("acme", []*check.Rule{widgetRule()}))

	for _, cat := range map[string]*check.Catalog{"DefaultCatalog": check.DefaultCatalog(), "CatalogWith": check.CatalogWith()} {
		r := cat.Lookup("acme/widget-present")
		if r == nil {
			t.Fatalf("registered rule missing from the composed catalog")
		}
		if r.Tags[check.KeySource] != "acme" {
			t.Errorf("source tag = %q, want acme (composition stamps KeySource)", r.Tags[check.KeySource])
		}
	}

	// End-to-end eval: the registered rule runs over a design and produces its finding.
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1"}}}
	rules := check.DefaultCatalog().Filter(check.Facets{Names: []string{"acme/widget-present"}})
	if len(rules) != 1 {
		t.Fatalf("Filter for the registered rule returned %d rules, want 1", len(rules))
	}
	findings := check.Run(check.NewModel(d), rules)
	if len(findings) != 1 || findings[0].Rule != "acme/widget-present" || findings[0].Subject != "U1" {
		t.Errorf("findings = %+v, want one acme/widget-present finding on U1", findings)
	}
}

// TestRegisterSourceRejectsBadSources pins the RegisterSource contract: nil, anonymous, a
// bad-grammar name, and a duplicate all panic (programming errors at startup).
func TestRegisterSourceRejectsBadSources(t *testing.T) {
	check.RegisterSource(check.NewSource("dup-guard", nil)) // claim a name to test the duplicate case

	cases := map[string]check.RuleSource{
		"nil source":     nil,
		"anonymous":      check.NewSource("", nil),
		"uppercase name": check.NewSource("Acme", nil),
		"space in name":  check.NewSource("acme corp", nil),
		"duplicate name": check.NewSource("dup-guard", nil),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("RegisterSource did not panic; want a panic")
				} else if !strings.Contains(r.(string), "check:") {
					t.Errorf("panic = %v, want a check: message", r)
				}
			}()
			check.RegisterSource(src)
		})
	}
}
