package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
)

// unconnectedComponent flags a component that appears on no net. See Detail.
var unconnectedComponent = &check.Rule{
	Name:       "unconnected-component",
	Severity:   "warning",
	Summary:    "A component appears on no net (none of its pins land on any signal).",
	Impact:     "A part on no net is either a placement mistake, a forgotten connection, or dead weight in the BOM. If it was meant to do something, the circuit is missing whatever it was meant to do.",
	Remedy:     "Wire the part into the circuit it belongs to, or delete it from the schematic if it is left over from an earlier revision. Placed and unwired is neither.",
	Primitives: []string{"select", "traverse"},
	Reads:      []string{"on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("unconnected-component"),
	Eval:                unconnectedComponentVerdicts,
	StatesConsideredSet: true,
}

// unconnectedComponentVerdicts decides every component in the design and returns one verdict each.
//
// THE EMPTY REF-DES GUARD BECOMES NotConsidered, not a skip. A part carrying no designator has no
// identity: there is nothing to key connectivity on and nothing to name in a report, so the rule
// genuinely cannot judge it. Dropping it silently, which is what the old `c.RefDes != ""` filter did,
// reported the same nothing as a part the rule cleared. Under an addressable model that is worse than
// unhelpful, because the subject answers 404 as though it did not exist. Saying so costs one verdict
// and is the distinction the considered set exists to carry.
//
// The pass witness counts the nets the part's pins land on, derived from the SAME net walk that
// backs Model.IsConnected, so the evidence cannot disagree with the outcome it justifies. Counting
// pins instead would introduce exactly that risk: a design read with no symbol library has no pin
// records, so a connected part would witness "0 pins" beside a Pass.
//
// The nets themselves are deliberately NOT carried as Context. On a large part that is dozens of
// entries, and a viewer renders each as a chip, so the proof would be less readable than the count
// it replaces. Nothing is hidden by the choice: the count moves the moment connectivity does.
func unconnectedComponentVerdicts(m check.Model) []check.Verdict {
	netsOn := map[string]int{}
	for _, n := range m.Nets() {
		seen := map[string]bool{}
		for _, c := range n.Connections {
			if seen[c.ComponentRef] {
				continue // one net counts once for a part, however many of its pins land on it
			}
			seen[c.ComponentRef] = true
			netsOn[c.ComponentRef]++
		}
	}

	var out []check.Verdict
	for _, c := range m.Components() {
		v := check.Verdict{
			Kind:    check.KindComponent,
			Subject: c.RefDes,
		}
		switch {
		case c.RefDes == "":
			v.Outcome = check.NotConsidered
			v.Reason = "the part carries no reference designator, so there is no identity to judge its connectivity by"
		case m.IsConnected(c.RefDes):
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("component's pins land on %d net(s)", netsOn[c.RefDes]),
				Terms:     []check.WitnessTerm{{Label: "nets reached", Value: strconv.Itoa(netsOn[c.RefDes])}},
			}
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "none of the component's pins land on any net",
				Terms:     []check.WitnessTerm{{Label: "nets reached", Value: "0"}},
			}
			f := check.CompFinding("component has no net connections")(c)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// unconnectedComponentSpec is the rule's declarative twin (WS3-003).
var unconnectedComponentSpec = &check.Spec{
	Over: "components",
	Where: check.And{Xs: []check.Expr{
		check.Cmp{L: check.Fact{Name: "component.ref_des"}, Op: "!=", R: check.Lit{V: ""}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "on_net"}}},
	}},
	Message: "component has no net connections",
}
