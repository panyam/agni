package check

import (
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
			return externalSignalNet(m, n) && !TVSReachable(m, n) && !ICESDRated(m, n) && !ZenerReachable(m, n)
		})
		return Report(bad, NetFinding("externally-exposed signal net has no ESD protection"))
	},
}

// externalSignalNet reports the scope esd-protection and esd-clamp-not-tvs share: a
// connector-facing signal net that is not a rail/ground (name or fact), not a deliberately
// unconnected pad, and not on a power path (input-protection's turf, WS3-011). The two rules
// partition these nets by what protects them (nothing / a Zener clamp).
func externalSignalNet(m Model, n *ir.Net) bool {
	a := n.Attributes
	if a[netgraph.AttrExternal] == "true" || a[netgraph.AttrGlobal] == "true" ||
		a[netgraph.AttrPowerDriven] == "true" || IsGroundName(n.Name) || IsPowerRailName(n.Name) {
		return false
	}
	if IntentionallyUnconnected(m, n) {
		return false
	}
	hasConn := Exists(n.Connections, func(c *ir.Connection) bool {
		return m.HasClass(c.ComponentRef, ClassConnector)
	})
	if !hasConn {
		return false
	}
	return !PowerPinReachable(m, n)
}

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
