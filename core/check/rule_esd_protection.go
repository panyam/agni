package check

import (
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// esdProtection flags an externally-exposed signal net with no TVS to clamp it. See Detail.
var esdProtection = &Rule{
	Name:       "esd-protection",
	Severity:   "info",
	Summary:    "An externally-exposed signal net (on a connector) has no TVS device.",
	Impact:     "A signal that leaves the board through a connector is a direct ESD path into the IC behind it. Human contact discharges kilovolts; an unclamped data pin takes the hit. Failures are intermittent, latent, and appear in the field as flaky ports and dead interfaces.",
	Primitives: []string{"reach", "select", "traverse", "exists", "pin-role", "pattern", "param-join"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type", "pin.no_connect", "param.esd_rating"},
	// The IC-ESD credit is an exemption, not the finding basis: esd-protection is a netlist
	// rule and stays applicable (reports over external nets) with no --params, unlike a
	// datasheet rule whose finding requires the seeded value.
	OptionalReads: []string{"param.esd_rating"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("esd-protection"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			// A discrete TVS clamps it, OR an IC on the signal carries a datasheet ESD rating
			// (IC-integrated ESD, the common automotive posture); either protects it (WS3-073).
			// A Zener clamp is NOT counted here — it is characterized separately by
			// esd-clamp-not-tvs (WS3-078), so this rule flags only a truly unprotected net.
			return externalSignalNet(m, n) && !tvsReachable(m, n) && !icEsdRated(m, n) && !zenerReachable(m, n)
		})
		return Report(bad, netFinding("externally-exposed signal net has no ESD protection"))
	},
}

// externalSignalNet reports the scope esd-protection and esd-clamp-not-tvs share: a
// connector-facing signal net that is not a rail/ground (name or fact), not a deliberately
// unconnected pad, and not on a power path (input-protection's turf, WS3-011). The two rules
// partition these nets by what protects them (nothing / a Zener clamp).
func externalSignalNet(m Model, n *ir.Net) bool {
	a := n.Attributes
	if a[netgraph.AttrExternal] == "true" || a[netgraph.AttrGlobal] == "true" ||
		a[netgraph.AttrPowerDriven] == "true" || isGroundName(n.Name) || isPowerRailName(n.Name) {
		return false
	}
	if intentionallyUnconnected(m, n) {
		return false
	}
	hasConn := Exists(n.Connections, func(c *ir.Connection) bool {
		return m.HasClass(c.ComponentRef, ClassConnector)
	})
	if !hasConn {
		return false
	}
	return !powerPinReachable(m, n)
}

// isPowerRailName reports whether a net name follows a supply-rail convention (VCC, VDD, VBUS, VIN,
// +3V3, 12V, ...). It is the rail-identity fallback for sources that carry no pin directions or power
// symbols (an EDIF netlist), where the name is the only evidence; rail nets are input-protection's and
// bulk-cap's concern, not ESD's. The pattern set lives in the active naming lexicon (WS3-069), so a
// project can extend it via --conventions; hierarchical sheet prefixes ("/psu/12V") are stripped first.
func isPowerRailName(name string) bool { return classify.ActiveRoleVocab().IsRail(name) }

// esdProtectionSpec is the rule's declarative twin (WS3-003): the widest guard stack in the
// catalog, every skip an explicit clause.
var esdProtectionSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		Not{X: IsTrue{T: Fact{"net.attr.global"}}},
		Not{X: IsTrue{T: Fact{"net.attr.power_driven"}}},
		Not{X: IsTrue{T: Call{Fn: "ground_name", Args: []Term{Fact{"net.names"}}}}},
		Not{X: IsTrue{T: Call{Fn: "rail_name", Args: []Term{Fact{"net.names"}}}}},
		Not{X: IsTrue{T: Call{Fn: "intentionally_unconnected"}}},
		ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"component.class"}, Op: "==", R: Lit{"connector"}}},
		Not{X: IsTrue{T: Call{Fn: "power_pin_reach"}}},
		Not{X: IsTrue{T: Call{Fn: "tvs_reach"}}},
		Not{X: IsTrue{T: Call{Fn: "ic_esd_rated"}}},
		Not{X: IsTrue{T: Call{Fn: "zener_reach"}}},
	}},
	Message: "externally-exposed signal net has no ESD protection",
}

// powerPinReachable reports a power-direction pin on the net or on any net in its 2-hop
// series reach (WS3-011): the esd/input-protection turf split must not depend on whether
// a bead sits between the connector and the regulator.
func powerPinReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if countDir(netDirs(m, rn), func(d ir.PinDirection) bool {
			return d == ir.PinDirection_PIN_DIRECTION_POWER_IN || d == ir.PinDirection_PIN_DIRECTION_POWER_OUT
		}) >= 1 {
			return true
		}
	}
	return false
}

// tvsReachable reports a TVS on the net or on any net in its 2-hop series reach
// (WS3-011): ESD structures commonly put a series resistor between the connector and
// the clamped node, which splits the net and hid the clamp from the pre-reach rule.
func tvsReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassTVS)
		}) {
			return true
		}
	}
	return false
}

// zenerReachable reports a Zener clamp on the net or on any net in its 2-hop series reach — the
// same reach the TVS check walks (WS3-011), so a series-split clamp is not hidden. A Zener is
// distinct from a TVS (a slower clamp), so esd-protection does not count it as ESD protection;
// esd-clamp-not-tvs (WS3-078) reports its presence separately for the review to weigh.
func zenerReachable(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		if Exists(rn.Connections, func(c *ir.Connection) bool {
			return m.HasClass(c.ComponentRef, ClassZener)
		}) {
			return true
		}
	}
	return false
}

// icEsdRated reports whether a component on the net (or within its 2-hop series reach) declares a
// datasheet ESD rating at or above the credit floor — the IC-integrated ESD that protects a
// connector-facing signal without a discrete TVS (WS3-073). Silent without a seeded param set
// (m.PartSpec is nil), so esd behaves exactly as before on a design read with no datasheets.
func icEsdRated(m Model, n *ir.Net) bool {
	for _, rn := range m.Reach(n, 2).Nets {
		for _, c := range rn.Connections {
			if spec := m.PartSpec(c.ComponentRef); spec != nil && len(esdRatingLimits(spec)) > 0 {
				return true
			}
		}
	}
	return false
}
