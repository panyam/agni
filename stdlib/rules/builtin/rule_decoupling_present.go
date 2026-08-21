package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// decouplingPresent flags a power rail with no decoupling capacitor. See Detail.
var decouplingPresent = &check.Rule{
	Name:       "decoupling-present",
	Severity:   "warning",
	Summary:    "A power rail feeds power-input pins but has no decoupling capacitor on it.",
	Impact:     "Chips draw current in sharp transients; without a local capacitor the rail sags and bounces at the pin. The board often works on the bench and then fails intermittently in the field (resets, corrupted logic, EMC failures), which is why decoupling review is a fixture of every design checklist.",
	Remedy:     "Add a decoupling capacitor from the rail to ground at each supply pin, and place it at the pin in layout. A capacitor drawn on the rail but placed across the board does not decouple it.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail: ruleDoc("decoupling-present"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" || m.IsGroundNet(n) {
				return false
			}
			hasPowerIn := check.Exists(n.Connections, func(c *ir.Connection) bool {
				return !check.IsVirtualRef(c.ComponentRef) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
			})
			return hasPowerIn && !check.Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, check.ClassCapacitor)
			})
		})
		return check.Report(bad, check.NetFinding("power rail has no decoupling capacitor"))
	}),
}

// decouplingPresentSpec is the rule's declarative twin (WS3-003).
var decouplingPresentSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}, check.Not{X: check.IsTrue{T: check.Fact{Name: "conn.virtual"}}}}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "capacitor"}}}},
	}},
	Message: "power rail has no decoupling capacitor",
}
