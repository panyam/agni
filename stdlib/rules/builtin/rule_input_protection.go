package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// inputProtection flags a board power input a connector feeds with no fuse or TVS. See Detail.
var inputProtection = &check.Rule{
	Name:       "input-protection",
	Severity:   "warning",
	Summary:    "A connector feeds a power-input pin directly with no fuse or TVS in the path.",
	Impact:     "An unprotected power input passes every upstream fault into the board: a shorted load takes out the cable or supply instead of a fuse, and a hot-plug transient or miswired adapter reaches the regulator unclamped. A classic review finding with real field consequences.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern", "reach"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail: ruleDoc("input-protection"),
	Eval: func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" || check.IsGroundName(n.Name) {
				return false
			}
			hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, check.ClassConnector)
			})
			if !hasConn {
				return false
			}
			return check.UnprotectedPowerReach(m, n)
		})
		return check.Report(bad, check.NetFinding("connector feeds a power input with no fuse or TVS in the path"))
	},
}

// inputProtectionSpec is the rule's declarative twin (WS3-003). The guard clauses stay
// AST; the reach walk is one declared FFI shared with the Go Eval (the WS3-011
// vocabulary is new, so the Go side stays canonical until it soaks — docs/19).
var inputProtectionSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "connector"}}},
		check.IsTrue{T: check.Call{Fn: "unprotected_power_reach"}},
	}},
	Message: "connector feeds a power input with no fuse or TVS in the path",
}
