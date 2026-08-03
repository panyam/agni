package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// unconnectedComponent flags a component that appears on no net. See Detail.
var unconnectedComponent = &check.Rule{
	Name:       "unconnected-component",
	Severity:   "warning",
	Summary:    "A component appears on no net (none of its pins land on any signal).",
	Impact:     "A part on no net is either a placement mistake, a forgotten connection, or dead weight in the BOM. If it was meant to do something, the circuit is missing whatever it was meant to do.",
	Primitives: []string{"select", "traverse"},
	Reads:      []string{"on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("unconnected-component"),
	Eval: func(m check.Model) []check.Finding {
		orphans := check.Select(m.Components(), func(c *ir.Component) bool {
			return c.RefDes != "" && !m.IsConnected(c.RefDes)
		})
		return check.Report(orphans, check.CompFinding("component has no net connections"))
	},
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
