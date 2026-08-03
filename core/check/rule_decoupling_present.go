package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// decouplingPresent flags a power rail with no decoupling capacitor. See Detail.
var decouplingPresent = &Rule{
	Name:       "decoupling-present",
	Severity:   "warning",
	Summary:    "A power rail feeds power-input pins but has no decoupling capacitor on it.",
	Impact:     "Chips draw current in sharp transients; without a local capacitor the rail sags and bounces at the pin. The board often works on the bench and then fails intermittently in the field (resets, corrupted logic, EMC failures), which is why decoupling review is a fixture of every design checklist.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("decoupling-present"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" || isGroundName(n.Name) {
				return false
			}
			hasPowerIn := Exists(n.Connections, func(c *ir.Connection) bool {
				return !isVirtualRef(c.ComponentRef) && connDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
			})
			return hasPowerIn && !Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, ClassCapacitor)
			})
		})
		return Report(bad, netFinding("power rail has no decoupling capacitor"))
	},
}

// decouplingPresentSpec is the rule's declarative twin (WS3-003).
var decouplingPresentSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		Not{X: IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{"net.names"}}}}},
		ExistsIn{Over: "net.connections", Where: And{Xs: []Expr{Cmp{L: Fact{"pin.electrical_type"}, Op: "==", R: Lit{"power_in"}}, Not{X: IsTrue{T: Fact{"conn.virtual"}}}}}},
		Not{X: ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"capacitor"}}}},
	}},
	Message: "power rail has no decoupling capacitor",
}
