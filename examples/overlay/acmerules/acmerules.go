// Package acmerules is a demonstration out-of-module rule suite for the open-core overlay
// skeleton (WS12-001). It registers a private "house-style" rule with the engine's public rule
// registry (WS12-004). Blank-importing it (import _ ".../acmerules") makes the rule appear in
// ListRules and run in CheckDesign on the engine's own CLI and serve, namespaced "acme/..." so
// it can never shadow a built-in.
//
// A real overlay's rules encode a company's private design policy it does not release; the point
// here is only the wiring, so the rule is deliberately simple.
package acmerules

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// init registers the suite as a named RuleSource. An overlay uses import-side-effect
// registration so a consumer wires the suite in with one blank import; the alternative is an
// explicit RegisterSource call from the composing binary's main (see WS12-004).
//
// The suite carries one rule of each authoring style, which is the point of the pair: a Go rule
// with an Eval closure (below) and a datalog rule declared as a query (acmedatalog.go). They
// register identically, so the choice is the author's and the engine does not care.
func init() {
	check.RegisterSource(check.NewSource("acme", []*check.Rule{
		noExperimentalRefDes,
		query.RuleFromQuery(experimentalOnPowerNet),
	}))
}

// noExperimentalRefDes is a house rule: an X-prefixed ref-des marks an experimental/breadboard
// part that must not reach a production design. It reads only component ref-des, so it runs on
// any format the engine ingests, including the overlay's own .acme reader.
var noExperimentalRefDes = &check.Rule{
	Name:     "no-experimental-refdes",
	Severity: "warning",
	Summary:  "ACME house rule: an X-prefixed ref-des is an experimental part, not for production",
	Impact:   "an experimental part reaches a production build; give it a real ref-des before release",
	Remedy:   "give the part a production ref-des, or take it out of the design before release",
	Reads:    []string{"component.ref_des"},
	Tags:     map[string]string{check.KeyCategory: "house-style"},
	// This rule states its full considered set: every component gets a verdict, so a reviewer can see
	// which parts were cleared rather than only which ones failed.
	StatesConsideredSet: true,
	Eval: func(m check.Model) []check.Verdict {
		var out []check.Verdict
		for _, c := range m.Components() {
			if strings.HasPrefix(c.RefDes, "X") {
				out = append(out, check.Verdict{
					Outcome: check.Fail,
					Kind:    check.KindComponent,
					Subject: c.RefDes,
					Finding: &check.Finding{
						Kind:    check.KindComponent,
						Subject: c.RefDes,
						Message: "experimental (X-prefixed) part in a production design",
					},
				})
				continue
			}
			out = append(out, check.Verdict{
				Outcome: check.Pass,
				Kind:    check.KindComponent,
				Subject: c.RefDes,
				Witness: &check.Witness{Statement: fmt.Sprintf("ref-des %q does not carry the experimental X prefix", c.RefDes)},
			})
		}
		return out
	},
}
