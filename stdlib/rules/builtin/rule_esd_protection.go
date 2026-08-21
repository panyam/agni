package builtin

import (
	"github.com/panyam/agni/core/check"
)

// esdProtection flags an externally-exposed signal net with no TVS to clamp it. See Detail.
var esdProtection = &check.Rule{
	Name:       "esd-protection",
	Severity:   "info",
	Summary:    "An externally-exposed signal net (on a connector) has no TVS device.",
	Impact:     "A signal that leaves the board through a connector is a direct ESD path into the IC behind it. Human contact discharges kilovolts; an unclamped data pin takes the hit. Failures are intermittent, latent, and appear in the field as flaky ports and dead interfaces.",
	Remedy:     "Fit an ESD TVS from the exposed signal to ground on the connector side of everything it protects. A clamp placed behind the transceiver protects nothing.",
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
	Detail:              ruleDoc("esd-protection"),
	Eval:                esdProtectionVerdicts,
	StatesConsideredSet: true,
}

// esdProtectionVerdicts decides every externally-exposed signal net, which is `ExternalSignalNet`'s
// scope and is deliberately narrow: a rail, a ground, a deliberately unconnected pad and a net that
// reaches no connector are none of them subjects of an ESD rule, so none gets a verdict. That scope
// is shared with esd-clamp-not-tvs and with the external_signal_net query relation, so all three
// answer about the same nets rather than three hand-assembled approximations of one set.
//
// THE ZENER CASE BECOMES NotConsidered, and it is the conversion's whole point on this rule. A Zener
// clamps, slower and at higher energy than an ESD TVS, so the net is neither unprotected (which this
// rule would report) nor clamped by the right device class (which it would otherwise pass). WS3-078
// split that off into esd-clamp-not-tvs precisely so the two read distinctly, and then both rules
// went silent on the other's turf, which put them back together downstream. Declining by name, and
// pointing at the rule that does answer, is what keeps the split visible.
//
// The two passes rest on different evidence and say so. A discrete TVS is a part on the board, so the
// witness names it. An IC-integrated rating is a datasheet claim, so the witness carries the value
// and cites the document it was read from, which is the difference between "protected" and
// "protected, and here is the page that says so".
func esdProtectionVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if !check.ExternalSignalNet(m, n) {
			continue // not a connector-facing signal net, so not a subject of an ESD rule
		}
		v := check.Verdict{Kind: check.KindNet, Subject: n.Name, NetID: n.GetId()}
		tvs := check.ReachableOfClass(m, n, check.ClassTVS)
		rated, ratedWitness := check.ICESDCredit(m, n)
		zener := check.ReachableOfClass(m, n, check.ClassZener)
		switch {
		case tvs != "":
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: "TVS " + tvs + " clamps the net",
				Terms:     []check.WitnessTerm{{Label: "clamp", Value: tvs}},
			}
			v.Context = compContext(tvs, "TVS in reach")
		case rated != "":
			// IC-integrated ESD, the common industrial posture (WS3-073): the part behind the pin
			// carries its own system-level rating, so a discrete clamp is not required.
			v.Outcome = check.Pass
			v.Witness = ratedWitness
			v.Context = compContext(rated, "ESD-rated part")
		case zener != "":
			v.Outcome = check.NotConsidered
			v.Reason = "the net is clamped by Zener " + zener + " rather than left bare, which esd-clamp-not-tvs characterises rather than this rule"
			v.Context = compContext(zener, "Zener clamp in reach")
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "no TVS or Zener is in the net's series reach and no part on it declares a datasheet ESD rating",
			}
			f := check.NetFinding("externally-exposed signal net has no ESD protection")(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
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
