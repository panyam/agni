package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// inputProtection flags a board power input a connector feeds with no fuse or TVS. See Detail.
var inputProtection = &Rule{
	Name:       "input-protection",
	Severity:   "warning",
	Summary:    "A connector feeds a power-input pin directly with no fuse or TVS in the path.",
	Impact:     "An unprotected power input passes every upstream fault into the board: a shorted load takes out the cable or supply instead of a fuse, and a hot-plug transient or miswired adapter reaches the regulator unclamped. A classic review finding with real field consequences.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern", "reach"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("input-protection"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" || isGroundName(n.Name) {
				return false
			}
			hasConn := Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, ClassConnector)
			})
			if !hasConn {
				return false
			}
			return unprotectedPowerReach(m, n)
		})
		return Report(bad, netFinding("connector feeds a power input with no fuse or TVS in the path"))
	},
}

// inputProtectionSpec is the rule's declarative twin (WS3-003). The guard clauses stay
// AST; the reach walk is one declared FFI shared with the Go Eval (the WS3-011
// vocabulary is new, so the Go side stays canonical until it soaks — docs/19).
var inputProtectionSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		Not{X: IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{"net.names"}}}}},
		ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"connector"}}},
		IsTrue{T: Call{Fn: "unprotected_power_reach"}},
	}},
	Message: "connector feeds a power input with no fuse or TVS in the path",
}
