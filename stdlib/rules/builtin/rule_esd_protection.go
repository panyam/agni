package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// esdProtection flags an externally-exposed signal net with no TVS to clamp it. See Detail.
var esdProtection = &check.Rule{
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
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail: ruleDoc("esd-protection"),
	Eval: func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			// A discrete TVS clamps it, OR an IC on the signal carries a datasheet ESD rating
			// (IC-integrated ESD, the common automotive posture); either protects it (WS3-073).
			// A Zener clamp is NOT counted here — it is characterized separately by
			// esd-clamp-not-tvs (WS3-078), so this rule flags only a truly unprotected net.
			return externalSignalNet(m, n) && !check.TVSReachable(m, n) && !check.ICESDRated(m, n) && !check.ZenerReachable(m, n)
		})
		return check.Report(bad, check.NetFinding("externally-exposed signal net has no ESD protection"))
	},
}

// externalSignalNet reports the scope esd-protection and esd-clamp-not-tvs share: a
// connector-facing signal net that is not a rail/ground (name or fact), not a deliberately
// unconnected pad, and not on a power path (input-protection's turf, WS3-011). The two rules
// partition these nets by what protects them (nothing / a Zener clamp).
func externalSignalNet(m check.Model, n *ir.Net) bool {
	a := n.Attributes
	if a[netgraph.AttrExternal] == "true" || a[netgraph.AttrGlobal] == "true" ||
		a[netgraph.AttrPowerDriven] == "true" || check.IsGroundName(n.Name) || check.IsPowerRailName(n.Name) {
		return false
	}
	if check.IntentionallyUnconnected(m, n) {
		return false
	}
	hasConn := check.Exists(n.Connections, func(c *ir.Connection) bool {
		return m.HasClass(c.ComponentRef, check.ClassConnector)
	})
	if !hasConn {
		return false
	}
	return !check.PowerPinReachable(m, n)
}

// esdProtectionSpec is the rule's declarative twin (WS3-003): the widest guard stack in the
// catalog, every skip an explicit clause.
var esdProtectionSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.global"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "rail_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "intentionally_unconnected"}}},
		check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "connector"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "power_pin_reach"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "tvs_reach"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ic_esd_rated"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "zener_reach"}}},
	}},
	Message: "externally-exposed signal net has no ESD protection",
}
