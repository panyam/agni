package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// bulkCap flags a named power rail with no capacitor anywhere on it. See Detail.
var bulkCap = &Rule{
	Name:       "bulk-cap",
	Severity:   "warning",
	Summary:    "A named power rail carries no capacitor at all (no bulk reservoir).",
	Impact:     "A rail with zero capacitance has no charge reservoir: every load transient sags the rail, and the regulator's feedback loop may oscillate. Boards usually survive the lab and fail under real load patterns.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("bulk-cap"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			named := n.Attributes[netgraph.AttrGlobal] == "true" || n.Attributes[netgraph.AttrPowerDriven] == "true"
			if !named || n.Attributes[netgraph.AttrExternal] == "true" || IsGroundName(n.Name) {
				return false
			}
			return !Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, ClassCapacitor)
			})
		})
		return Report(bad, NetFinding("power rail has no bulk capacitor"))
	},
}

// bulkCapSpec is the rule's declarative twin (WS3-003).
var bulkCapSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Or{Xs: []Expr{IsTrue{T: Fact{"net.attr.global"}}, IsTrue{T: Fact{"net.attr.power_driven"}}}},
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		Not{X: IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{"net.names"}}}}},
		Not{X: ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"capacitor"}}}},
	}},
	Message: "power rail has no bulk capacitor",
}
