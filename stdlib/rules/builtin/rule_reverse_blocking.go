package builtin

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/netgraph"
)

// reverseBlockingAbsent flags a connector-fed power path with no DIRECTIONAL blocking element
// (WS3-094). It is the sibling of input-protection: same walk, different question. That rule asks
// whether a fuse or TVS guards the path; this asks whether anything stops current flowing the WRONG
// WAY.
//
// The two are not substitutes, which is why this ticket exists at all. A fuse opens on current
// MAGNITUDE regardless of sign, and a TVS shunts transients to ground rather than blocking a path, so
// "a fuse or a TVS is present" carries no information about reverse flow. Two CarCo review items were
// bound to input-protection and reading pass with their actual ask never tested.
//
// WHAT IT WILL NOT CLAIM. A P-FET ideal diode is a transistor plus a bias network, and nothing in a
// netlist labels that arrangement — it is the correct modern answer to reverse protection and it is
// structurally indistinguishable from any other FET on the path. So a path crossing a transistor is
// reported as UNCLASSIFIABLE rather than unprotected: the rule stays silent there. Firing anyway
// would false-fail every ORing-FET design, which is worse than the gap it would close, and this
// family of rules is exactly where a confident wrong answer costs the most.
var reverseBlockingAbsent = &check.Rule{
	Name:       "reverse-blocking-absent",
	Severity:   "warning",
	Summary:    "A connector feeds a power input with no directional element blocking reverse flow.",
	Impact:     "Reverse polarity from a miswired connector, or backfeed from a parallel source into a switched-off rail, reaches the board unopposed. ISO 16750-2 makes reverse voltage a qualification requirement on a vehicle, and a fuse does not help: it opens on magnitude, not direction.",
	Primitives: []string{"select", "traverse", "reach", "pin-role"},
	Reads:      []string{"component.class", "net.attributes", "on_net", "pin.electrical_type", "pin.role"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("reverse-blocking-absent"),
	Eval: func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" || m.IsGroundNet(n) {
				return false
			}
			hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
				return m.HasClass(c.ComponentRef, check.ClassConnector)
			})
			return hasConn && unblockedPowerPath(m, n)
		})
		return check.Report(bad, check.NetFinding("connector feeds a power input with no reverse-blocking element in the path"))
	},
}

// unblockedPowerPath reports whether n's power path carries nothing that blocks reverse flow.
//
// THE WALK IS THE MECHANISM, and it works because of what it refuses to cross. check.Reach crosses
// only two-terminal PASSIVES (resistor, inductor, ferrite, fuse) — a diode is deliberately not a pass
// element, "polarity, not a wire", and neither is a transistor. So:
//
//   - A power input reachable through the passive walk means NOTHING directional stands between the
//     connector and the load. That is the finding.
//   - A directional part stops the walk, which is why this rule then has to look at what stopped it
//     rather than concluding from silence. A backwards diode stops the walk exactly as a correct one
//     does, and reading that as protection would be a false pass on the defect this rule exists for.
func unblockedPowerPath(m check.Model, n *ir.Net) bool {
	r := m.Reach(n, check.PowerPathReachHops)
	inReach := map[string]bool{}
	for _, rn := range r.Nets {
		inReach[rn.GetName()] = true
		if hasPowerInput(m, rn) {
			return true // reached a load through passives alone: nothing directional in the way
		}
	}
	// Nothing reachable, so something stopped the walk. Classify each part bridging out of the
	// neighborhood toward a power input.
	unblocked := false
	for _, rn := range r.Nets {
		for _, c := range rn.GetConnections() {
			ref := c.GetComponentRef()
			far := farNet(m, ref, inReach)
			if far == nil || !feedsPowerInput(m, far) {
				continue
			}
			switch {
			case m.ComponentClass(ref) == check.ClassTransistor:
				return false // an ORing FET or ideal diode; unclassifiable, never a finding
			case m.ComponentClass(ref) == check.ClassDiode:
				if pinNetWithRole(m, ref, check.RoleAnode) != rn.GetName() {
					unblocked = true // fitted backwards: it blocks the supply, not the fault
				}
			}
		}
	}
	return unblocked
}

// hasPowerInput reports whether a net carries a real power-input pin (virtual power symbols excluded).
func hasPowerInput(m check.Model, n *ir.Net) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return !check.IsVirtualRef(c.GetComponentRef()) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
	})
}

// feedsPowerInput reports whether a power input sits on n or in its passive neighborhood.
func feedsPowerInput(m check.Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, check.PowerPathReachHops).Nets {
		if hasPowerInput(m, rn) {
			return true
		}
	}
	return false
}

// farNet returns the single net ref touches OUTSIDE the given set, or nil when it touches none or
// several. A two-terminal series part has exactly one far side; anything more is not a simple series
// element and this rule does not reason about it.
func farNet(m check.Model, ref string, inReach map[string]bool) *ir.Net {
	var out *ir.Net
	for _, n := range m.Nets() {
		if inReach[n.GetName()] || !touchesRef(n, ref) {
			continue
		}
		if out != nil {
			return nil
		}
		out = n
	}
	return out
}

// touchesRef reports whether refDes has a connection on n.
func touchesRef(n *ir.Net, refDes string) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
}

// pinNetWithRole returns the net name carrying ref's pin of the given role, or "" when the part
// declares no such pin. A diode whose part type names no anode yields "", so orientation is unknown
// and the caller treats it as backwards rather than crediting protection it cannot see.
func pinNetWithRole(m check.Model, ref string, role check.PinRole) string {
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() != ref {
			continue
		}
		if m.PinRole(ref, p.Designator) == role {
			return m.PinNetName(ref, p.Designator)
		}
	}
	return ""
}
