package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	Detail: ruleDoc("esd-clamp-not-tvs"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			// The same external-signal scope as esd-protection, but the net HAS a Zener clamp in
			// reach and no TVS / IC-ESD rating: the two rules are mutually exclusive on a net.
			return check.ExternalSignalNet(m, n) && check.ZenerReachable(m, n) && !check.TVSReachable(m, n) && !check.ICESDRated(m, n)
		})
		return check.Report(bad, check.NetFinding("externally-exposed signal net is clamped by a Zener, not a fast ESD TVS"))
	}),
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
