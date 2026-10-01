package builtin

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/netgraph"
)

// bulkCap flags a named power rail with no capacitor anywhere on it. See Detail.
var bulkCap = &check.Rule{
	Name:       "bulk-cap",
	Severity:   "warning",
	Summary:    "A named power rail carries no capacitor at all (no bulk reservoir).",
	Impact:     "A rail with zero capacitance has no charge reservoir: every load transient sags the rail, and the regulator's feedback loop may oscillate. Boards usually survive the lab and fail under real load patterns.",
	Remedy:     "Add a bulk reservoir capacitor where the rail enters, sized from the load step the rail has to absorb and from the regulator's own stability requirement.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("bulk-cap"),
	Eval:                bulkCapVerdicts,
	StatesConsideredSet: true,
}

// bulkCapVerdicts decides every NAMED power rail, and that set is the considered set. A net counts
// as named when the design declares it global or drives it with a power flag, so an ordinary signal
// net yields no verdict. Neither does a ground, which is the return path rather than the reservoir.
//
// decoupling-present asks the same question about rails that reach a supply PIN, where this one asks
// about rails the design NAMES, so a rail can be a subject of one and not the other (#400). A pass
// names the capacitor, so a reviewer can go and look at it.
func bulkCapVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		named := n.Attributes[netgraph.AttrGlobal] == "true" || n.Attributes[netgraph.AttrPowerDriven] == "true"
		if !named || m.IsGroundNet(n) {
			continue
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		bulk := firstOnNet(m, n, check.ClassCapacitor)
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the rail continues onto a sheet this read did not open, so its reservoir may be drawn outside it"
		case bulk != "":
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: "capacitor " + bulk + " sits on the rail",
				Terms:     []check.WitnessTerm{{Label: "capacitor", Value: bulk}},
			}
			v.Context = compContext(bulk, "capacitor on the rail")
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "the rail carries no capacitor at all",
			}
			f := check.NetFinding("power rail has no bulk capacitor")(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// bulkCapSpec is the rule's declarative twin.
var bulkCapSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Or{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "net.attr.global"}}, check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "capacitor"}}}},
	}},
	Message: "power rail has no bulk capacitor",
}
