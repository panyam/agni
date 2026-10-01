// Package myrules is the rule slot of the extension template. Copy it, rename it, and replace the
// example rule with your own. It registers a named source through check.RegisterSource, so a blank
// import puts the rules in ListRules and CheckDesign, namespaced "myco/..." so none can shadow a
// built-in.
//
// Walkthrough: docsite/content/build/extending.md#register-private-rules-with-checkregistersource
package myrules

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
)

// init registers the suite by import side effect. To register from your binary's main instead,
// delete this init and call check.RegisterSource there.
func init() {
	// TODO: your source name (lowercase [a-z0-9-]+). It becomes the "<name>/<rule>" namespace.
	check.RegisterSource(check.NewSource("myco", []*check.Rule{exampleRule}))
}

// exampleRule is a placeholder house rule that flags any component with an empty ref-des. Replace
// it with your real policy, which can read any fact the check.Model exposes.
var exampleRule = &check.Rule{
	Name:     "example-rule",
	Severity: "warning",
	Summary:  "TEMPLATE: replace with your own house-style rule",
	Impact:   "describe what goes wrong when this rule is violated",
	Remedy:   "describe what to DO about it, in the imperative, as one engineer would say it to another",
	Reads:    []string{"component.ref_des"},
	Tags:     map[string]string{check.KeyCategory: "house-style"},
	// StatesConsideredSet says Eval returns EVERY subject the rule looked at, not just the failures.
	// Leave it false while your Eval reports only violations, or `check --verdicts` presents the
	// failure list as coverage.
	StatesConsideredSet: true,
	// Eval MAPS each subject onto a verdict rather than filtering down to what failed. A pass
	// carries the proof it rests on, so a reader can tell a part you cleared from one nobody
	// checked. The rule's findings are projected from this (Rule.Findings).
	Eval: func(ctx context.Context, m check.Model) []check.Verdict {
		var out []check.Verdict
		for _, c := range m.Components() {
			// TODO: your condition. This placeholder flags an unnamed component.
			if c.RefDes == "" {
				out = append(out, check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindComponent, Ref: "(unnamed)"}}, Outcome: check.Fail, Finding: &check.Finding{Subject: check.Entity{Kind: check.KindComponent, Ref: "(unnamed)"}, Message: "component has no ref-des"}})
				continue
			}
			out = append(out, check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindComponent, Ref: c.RefDes}}, Outcome: check.Pass, // Say what the pass RESTS ON. A statement that would read the same on a design where
				// the rule concluded the opposite proves nothing.
				Witness: &check.Witness{Statement: fmt.Sprintf("component carries the ref-des %q", c.RefDes)}})
		}
		return out
	},
}
