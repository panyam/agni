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

// unprotectedPowerReach walks the connector net's series neighborhood (WS3-011) and
// reports whether SOME reached net carries a real power-input pin with neither a fuse
// crossed on the way there nor a TVS on any net along that path. The per-target path
// check matters: a board can have a protected 5V path and an unprotected 3V3 path off
// one connector, and protection on one must not excuse the other.
func unprotectedPowerReach(m Model, n *ir.Net) bool {
	r := m.Reach(n, 3)
	protectorOn := func(net *ir.Net) bool {
		return Exists(net.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassFuse) || m.HasClass(c.ComponentRef, ClassTVS)
		})
	}
	for _, target := range r.Nets {
		hasPowerIn := Exists(target.Connections, func(c *ir.Connection) bool {
			return !isVirtualRef(c.ComponentRef) && connDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
		})
		if !hasPowerIn {
			continue
		}
		protected := false
		for _, ref := range r.ThroughOnPath(target) {
			if m.HasClass(ref, ClassFuse) {
				protected = true // a fuse sits on this path as a series element
				break
			}
		}
		if !protected {
			// A protector as a MEMBER of a path net also counts — the pre-reach rule's
			// (conservative) reading, kept so no previously-quiet board starts firing.
			for _, pn := range r.PathTo(target) {
				if protectorOn(pn) {
					protected = true
					break
				}
			}
		}
		if !protected {
			return true
		}
	}
	return false
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
