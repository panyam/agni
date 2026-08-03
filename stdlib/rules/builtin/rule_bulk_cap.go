package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// bulkCap flags a named power rail with no capacitor anywhere on it. See Detail.
var bulkCap = &check.Rule{
	Name:       "bulk-cap",
	Severity:   "warning",
	Summary:    "A named power rail carries no capacitor at all (no bulk reservoir).",
	Impact:     "A rail with zero capacitance has no charge reservoir: every load transient sags the rail, and the regulator's feedback loop may oscillate. Boards usually survive the lab and fail under real load patterns.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail: ruleDoc("bulk-cap"),
	Eval: func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			named := n.Attributes[netgraph.AttrGlobal] == "true" || n.Attributes[netgraph.AttrPowerDriven] == "true"
			if !named || n.Attributes[netgraph.AttrExternal] == "true" || check.IsGroundName(n.Name) {
				return false
			}
			return !check.Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, check.ClassCapacitor)
			})
		})
		return check.Report(bad, check.NetFinding("power rail has no bulk capacitor"))
	},
}

// bulkCapSpec is the rule's declarative twin (WS3-003).
var bulkCapSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Or{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "net.attr.global"}}, check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "capacitor"}}}},
	}},
	Message: "power rail has no bulk capacitor",
}
