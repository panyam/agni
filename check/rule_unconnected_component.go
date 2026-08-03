package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// unconnectedComponent flags a component that appears on no net. See Detail.
var unconnectedComponent = &Rule{
	Name:       "unconnected-component",
	Severity:   "warning",
	Summary:    "A component appears on no net (none of its pins land on any signal).",
	Impact:     "A part on no net is either a placement mistake, a forgotten connection, or dead weight in the BOM. If it was meant to do something, the circuit is missing whatever it was meant to do.",
	Primitives: []string{"select", "traverse"},
	Reads:      []string{"on_net"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("unconnected-component"),
	Eval: func(m Model) []Finding {
		orphans := Select(m.Components(), func(c *ir.Component) bool {
			return c.RefDes != "" && !m.IsConnected(c.RefDes)
		})
		return Report(orphans, compFinding("component has no net connections"))
	},
}

// unconnectedComponentSpec is the rule's declarative twin (WS3-003).
var unconnectedComponentSpec = &Spec{
	Over: "components",
	Where: And{Xs: []Expr{
		Cmp{L: Fact{"component.ref_des"}, Op: "!=", R: Lit{""}},
		Not{X: IsTrue{T: Fact{"on_net"}}},
	}},
	Message: "component has no net connections",
}
