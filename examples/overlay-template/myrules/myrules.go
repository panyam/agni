// Package myrules is the rule slot of the overlay template. Copy this package, rename it, and
// replace the example rule with your house-style/private rules. It registers a named rule source
// with the engine's public rule registry (check.RegisterSource); blank-importing it makes the
// rules appear in ListRules and run in CheckDesign, namespaced "myco/..." so they can never
// shadow a built-in.
//
// See docs/OVERLAY_AUTHORING.md for the full walkthrough.
package myrules

import (
	"fmt"

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
	Remedy:   "describe what to DO about it, in the imperative, as one engineer would say it to another",
	Reads:    []string{"component.ref_des"},
	Tags:     map[string]string{check.KeyCategory: "house-style"},
	// StatesConsideredSet says Eval below returns EVERY subject the rule looked at, not just the ones
	// that failed. Leave it false while your Eval only reports violations, or `check --verdicts` will
	// present your failure list as though it were coverage.
	StatesConsideredSet: true,
	// Eval MAPS each subject onto a verdict rather than filtering the design down to what failed. A
	// pass carries the proof it rests on, so a reader can tell a part you cleared from one nobody
	// checked. The findings your rule contributes are the projection of this (Rule.Findings), so
	// there is no second body to keep in step.
	Eval: func(m check.Model) []check.Verdict {
		var out []check.Verdict
		for _, c := range m.Components() {
			// TODO: your condition. This placeholder flags an unnamed component.
			if c.RefDes == "" {
				out = append(out, check.Verdict{
					Outcome: check.Fail,
					Kind:    check.KindComponent,
					Subject: "(unnamed)",
					Finding: &check.Finding{
						Kind:    check.KindComponent,
						Subject: "(unnamed)",
						Message: "component has no ref-des",
					},
				})
				continue
			}
			out = append(out, check.Verdict{
				Outcome: check.Pass,
				Kind:    check.KindComponent,
				Subject: c.RefDes,
				// Say what the pass RESTS ON. A statement that would read the same on a design where
				// the rule concluded the opposite proves nothing.
				Witness: &check.Witness{Statement: fmt.Sprintf("component carries the ref-des %q", c.RefDes)},
			})
		}
		return out
	},
}
