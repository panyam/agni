package builtin

import (
	"context"
	"slices"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// bulkCap flags a named power rail with no capacitor anywhere on it. See Detail.
var bulkCap = &check.Rule{
	Name:       "bulk-cap",
	Severity:   "warning",
	Summary:    "A named power rail carries no capacitor at all (no bulk reservoir).",
	Impact:     "A rail with zero capacitance has no charge reservoir: every load transient sags the rail, and the regulator's feedback loop may oscillate. Boards usually survive the lab and fail under real load patterns.",
	Remedy:     "Add a bulk reservoir capacitor where the rail enters, sized from the load step the rail has to absorb and from the regulator's own stability requirement.",
	Primitives: []string{"select", "exists", "traverse", "pattern", "pin-role"},
	Reads:      []string{"component.class", "component.dnp", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("bulk-cap"),
	Eval:                bulkCapVerdicts,
	Scoping:             check.ScopeSubjects,
	StatesConsideredSet: true,
}

// bulkCapVerdicts decides every NAMED power rail, and that set is the considered set. A net counts
// as named when it carries a power symbol or a power flag, or is global and reads as a rail, so an
// ordinary signal net yields no verdict, even one drawn with a global label (agni 1252). Neither does
// a ground, which is the return path rather than the reservoir.
//
// decoupling-present asks the same question about rails that reach a supply PIN, where this one asks
// about rails the design NAMES, so a rail can be a subject of one and not the other (#400). A pass
// names the capacitor, so a reviewer can go and look at it. A capacitor across a zero-ohm link counts,
// since the link joins two names for one rail (agni issue 1015).
func bulkCapVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range check.ScopedNets(ctx, m) {
		named := powerSymbolNet(m, n) || n.Attributes[netgraph.AttrPowerDriven] == "true" ||
			(n.Attributes[netgraph.AttrGlobal] == "true" && m.IsRailNet(n))
		if !named || m.IsGroundNet(n) {
			continue
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		bulk, at := check.FirstOnRail(m, n, check.ClassCapacitor)
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the rail continues onto a sheet this read did not open, so its reservoir may be drawn outside it"
		case check.Unpopulated(m, n):
			v.Outcome = check.NotConsidered
			v.Reason = "every part on the rail is marked do-not-populate, so the assembled board has no rail here to hold up"
		case bulk == "" && len(moduleFedOnly(m, n)) > 0:
			v.Outcome = check.NotConsidered
			v.Reason = "the rail feeds only " + strings.Join(moduleFedOnly(m, n), ", ") + ", declared internal, and no part on this board draws from it, so the module across the connector holds its own reservoir"
		case bulk != "":
			v.Outcome = check.Pass
			v.Witness, v.Context = railCapacitorWitness(bulk, at, "capacitor")
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

// moduleFedOnly returns the internal connectors on n when they are the rail's only load, and nil
// otherwise (agni issue 1914). Jetson's `Net-(D1-C)` carries a power flag and feeds the module's
// backup-battery pin across J1, through a Schottky from +3V3_AON and 2k2 from the coin cell. No IC on
// the board sits on it, so a bulk capacitor there would hold up nothing the board builds. A USB VBUS
// output still has its load across an exposed connector, so it stays judged.
func moduleFedOnly(m check.Model, n *ir.Net) []string {
	var refs []string
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if check.IsVirtualRef(ref) || !m.IsFitted(ref) || check.IsPassiveClass(m.ComponentClass(ref)) ||
			m.HasClass(ref, check.ClassDiode) || m.HasClass(ref, check.ClassTVS) || m.HasClass(ref, check.ClassZener) {
			continue
		}
		if !m.HasClass(ref, check.ClassConnector) || m.ExposedConnector(ref) {
			return nil
		}
		if !slices.Contains(refs, ref) {
			refs = append(refs, ref)
		}
	}
	return refs
}

func init() {
	check.RegisterSpecFunc("module_fed_only", &check.SpecFunc{
		// A rail whose only load sits across an internal connector (see moduleFedOnly).
		Reads:      []string{"component.class", "component.dnp", "on_net"},
		Primitives: []string{"exists", "traverse"},
		Fn: func(m check.Model, ents map[string]any, _ []any) any {
			return len(moduleFedOnly(m, ents["net"].(*ir.Net))) > 0
		},
	})
}

// bulkCapSpec is the rule's declarative twin.
var bulkCapSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Or{Xs: []check.Expr{
			check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "conn.virtual"}}, check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}}}},
			check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}},
			check.And{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "net.attr.global"}}, check.IsTrue{T: check.Call{Fn: "rail_net"}}}},
		}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.unpopulated"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.rail_capacitor"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "module_fed_only"}}},
	}},
	Message: "power rail has no bulk capacitor",
}
