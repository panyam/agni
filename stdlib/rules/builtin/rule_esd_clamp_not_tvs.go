package builtin

import (
	"github.com/panyam/agni/core/check"
)

// esdClampNotTVS flags an externally-exposed signal net whose only transient protection in reach is
// a Zener clamp, not a fast ESD TVS. It is the softer sibling of esd-protection (WS3-078): the two
// partition the unprotected external-signal nets by what is present, so "clamped by a Zener" reads
// distinctly from "no protection at all" and a review can weigh it per its ESD policy. See Detail.
var esdClampNotTVS = &check.Rule{
	Name:       "esd-clamp-not-tvs",
	Severity:   "info",
	Summary:    "An externally-exposed signal net is clamped by a Zener, not a fast ESD TVS.",
	Impact:     "A Zener clamps slower and at higher energy than a TVS designed for ESD (IEC 61000-4-2 is ns-scale). It gives transient / load-dump protection, but a review targeting ESD specifically may still want a dedicated TVS on the exposed pin. This is informational: the net is not unprotected, it is protected by the wrong device class for ESD.",
	Remedy:     "Replace the Zener with an ESD-rated TVS where the review targets ESD, or record that the Zener is there for transient clamping and that ESD is covered elsewhere.",
	Primitives: []string{"reach", "select", "traverse", "exists", "pin-role", "pattern", "param-join"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type", "pin.no_connect", "param.esd_rating"},
	// The IC-ESD credit is an exemption, not the finding basis (same as esd-protection): the rule
	// stays applicable over external nets with no --params.
	OptionalReads: []string{"param.esd_rating"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("esd-clamp-not-tvs"),
	Eval:                esdClampNotTVSVerdicts,
	StatesConsideredSet: true,
}

// esdClampNotTVSVerdicts decides the same externally-exposed signal nets esd-protection does, and the
// two are exact mirrors of each other. Where this rule declines, that one answers, and the other way
// round: a bare net is esd-protection's finding and this rule's NotConsidered, and a Zener-clamped net
// is this rule's finding and that one's NotConsidered. Stating the decline is what makes the WS3-078
// split legible from outside. Both rules going quiet on the other's turf is how "clamped by the wrong
// device class" and "clamped correctly" became the same silence in the first place.
//
// The question this rule asks is narrow — is the transient protection a Zener where the review wants
// a TVS — so its passes are narrow too, and each names the device it credits.
func esdClampNotTVSVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if !check.ExternalSignalNet(m, n) {
			continue // not a connector-facing signal net, so not a subject of an ESD rule
		}
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		tvs := check.ReachableOfClass(m, n, check.ClassTVS)
		rated, ratedWitness := check.ICESDCredit(m, n)
		zener := check.ReachableOfClass(m, n, check.ClassZener)
		switch {
		case tvs != "":
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: "the net's clamp is TVS " + tvs + ", which is the fast ESD device class this rule asks for",
				Terms:     []check.WitnessTerm{{Label: "clamp", Value: tvs}},
			}
			v.Context = compContext(tvs, "TVS in reach")
		case rated != "":
			v.Outcome = check.Pass
			v.Witness = ratedWitness
			v.Context = compContext(rated, "ESD-rated part")
		case zener != "":
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "Zener " + zener + " is the only clamp in the net's series reach, and no part on it declares a datasheet ESD rating",
				Terms:     []check.WitnessTerm{{Label: "clamp", Value: zener}},
			}
			f := check.NetFinding("externally-exposed signal net is clamped by a Zener, not a fast ESD TVS")(n)
			v.Finding = &f
			v.Context = compContext(zener, "Zener clamp in reach")
		default:
			v.Outcome = check.NotConsidered
			v.Reason = "the net carries no clamp at all, so there is no device class to characterise; esd-protection reports it"
		}
		out = append(out, v)
	}
	return out
}

// esdClampNotTVSSpec is the declarative twin: the esd-protection guard stack with the Zener clause
// flipped positive (this rule fires WHERE esd-protection's zener_reach negation would suppress it).
var esdClampNotTVSSpec = &check.Spec{
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
		check.IsTrue{T: check.Call{Fn: "zener_reach"}},
	}},
	Message: "externally-exposed signal net is clamped by a Zener, not a fast ESD TVS",
}
