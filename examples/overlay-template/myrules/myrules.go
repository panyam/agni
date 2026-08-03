// Package myrules is the rule slot of the overlay template. Copy this package, rename it, and
// replace the example rule with your house-style/private rules. It registers a named rule source
// with the engine's public rule registry (check.RegisterSource); blank-importing it makes the
// rules appear in ListRules and run in CheckDesign, namespaced "myco/..." so they can never
// shadow a built-in.
//
// See docs/OVERLAY_AUTHORING.md for the full walkthrough.
package myrules

import (
	"github.com/panyam/agni/core/check"
)

// init registers the suite by import side effect. To register explicitly from your binary's
// main instead, delete this init and call check.RegisterSource there.
func init() {
	// TODO: your source name (lowercase [a-z0-9-]+) — it becomes the "<name>/<rule>" namespace.
	check.RegisterSource(check.NewSource("myco", []*check.Rule{exampleRule}))
}

// exampleRule is a placeholder house rule: it flags any component with an empty ref-des.
// Replace it with your real policy — it can read any fact the check.Model exposes.
var exampleRule = &check.Rule{
	Name:     "example-rule",
	Severity: "warning",
	Summary:  "TEMPLATE: replace with your own house-style rule",
	Impact:   "describe what goes wrong when this rule is violated",
	Reads:    []string{"component.ref_des"},
	Tags:     map[string]string{check.KeyCategory: "house-style"},
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		for _, c := range m.Components() {
			// TODO: your condition. This placeholder flags an unnamed component.
			if c.RefDes == "" {
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: "(unnamed)",
					Message: "component has no ref-des",
				})
			}
		}
		return out
	},
}
