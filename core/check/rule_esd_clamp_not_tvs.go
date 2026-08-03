package check

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// esdClampNotTVS flags an externally-exposed signal net whose only transient protection in reach is
// a Zener clamp, not a fast ESD TVS. It is the softer sibling of esd-protection (WS3-078): the two
// partition the unprotected external-signal nets by what is present, so "clamped by a Zener" reads
// distinctly from "no protection at all" and a review can weigh it per its ESD policy. See Detail.
var esdClampNotTVS = &Rule{
	Name:       "esd-clamp-not-tvs",
	Severity:   "info",
	Summary:    "An externally-exposed signal net is clamped by a Zener, not a fast ESD TVS.",
	Impact:     "A Zener clamps slower and at higher energy than a TVS designed for ESD (IEC 61000-4-2 is ns-scale). It gives transient / load-dump protection, but a review targeting ESD specifically may still want a dedicated TVS on the exposed pin. This is informational: the net is not unprotected, it is protected by the wrong device class for ESD.",
	Primitives: []string{"reach", "select", "traverse", "exists", "pin-role", "pattern", "param-join"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type", "pin.no_connect", "param.esd_rating"},
	// The IC-ESD credit is an exemption, not the finding basis (same as esd-protection): the rule
	// stays applicable over external nets with no --params.
	OptionalReads: []string{"param.esd_rating"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistPublicReference,
	},
	Detail: ruleDoc("esd-clamp-not-tvs"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			// The same external-signal scope as esd-protection, but the net HAS a Zener clamp in
			// reach and no TVS / IC-ESD rating: the two rules are mutually exclusive on a net.
			return externalSignalNet(m, n) && ZenerReachable(m, n) && !TVSReachable(m, n) && !ICESDRated(m, n)
		})
		return Report(bad, NetFinding("externally-exposed signal net is clamped by a Zener, not a fast ESD TVS"))
	},
}

// esdClampNotTVSSpec is the declarative twin: the esd-protection guard stack with the Zener clause
// flipped positive (this rule fires WHERE esd-protection's zener_reach negation would suppress it).
var esdClampNotTVSSpec = &Spec{
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
		IsTrue{T: Call{Fn: "zener_reach"}},
	}},
	Message: "externally-exposed signal net is clamped by a Zener, not a fast ESD TVS",
}
