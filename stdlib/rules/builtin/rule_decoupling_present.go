package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// decouplingPresent flags a power rail with no decoupling capacitor. See Detail.
var decouplingPresent = &check.Rule{
	Name:       "decoupling-present",
	Severity:   "warning",
	Summary:    "A power rail feeds power-input pins but has no decoupling capacitor on it.",
	Impact:     "Chips draw current in sharp transients; without a local capacitor the rail sags and bounces at the pin. The board often works on the bench and then fails intermittently in the field (resets, corrupted logic, EMC failures), which is why decoupling review is a fixture of every design checklist.",
	Remedy:     "Add a decoupling capacitor from the rail to ground at each supply pin, and place it at the pin in layout. A capacitor drawn on the rail but placed across the board does not decouple it.",
	Primitives: []string{"select", "traverse", "exists", "pin-role", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("decoupling-present"),
	Eval:                decouplingPresentVerdicts,
	StatesConsideredSet: true,
}

// decouplingPresentVerdicts decides every net that actually feeds a supply pin, and that set is the
// considered set. Two kinds of net yield no verdict at all, because neither is a subject of a
// decoupling rule rather than being one the rule cleared:
//
//   - A net with no power-input pin on it decouples nothing. The rule is about rails that feed a
//     chip, and a signal net is not a rail with no capacitor; it is not a rail.
//   - A GROUND net is the reference the decoupling is measured against, not the thing decoupled.
//     Reporting it as a pass would state that ground is adequately decoupled, which is not a claim
//     this rule makes or could support.
//
// The EXTERNAL net is different and becomes NotConsidered: it is a rail, it does feed supply pins,
// and its capacitor may simply be drawn on a sheet this read did not open. That is the rule failing
// to see rather than the design failing to decouple, and the two used to be the same silence.
//
// The pass NAMES the capacitor, in the witness and in Context. "A capacitor exists" is not something
// a reviewer can act on; "C14 is the capacitor on this rail" is, because they can go and look at
// where it sits. It also makes the witness track the fact: delete C14 and the statement names a
// different part or the verdict flips.
func decouplingPresentVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		hasPowerIn := check.Exists(n.Connections, func(c *ir.Connection) bool {
			return !check.IsVirtualRef(c.ComponentRef) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
		})
		if !hasPowerIn || m.IsGroundNet(n) {
			continue // not a rail feeding a supply pin, or the reference the decoupling returns to
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		decap := firstOnNet(m, n, check.ClassCapacitor)
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the rail continues onto a sheet this read did not open, so its decoupling may be drawn outside it"
		case decap != "":
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: "capacitor " + decap + " sits on the rail",
				Terms:     []check.WitnessTerm{{Label: "decoupling capacitor", Value: decap}},
			}
			v.Context = compContext(decap, "capacitor on the rail")
		default:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "no capacitor sits on the rail, which feeds at least one power-input pin",
			}
			f := check.NetFinding("power rail has no decoupling capacitor")(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// decouplingPresentSpec is the rule's declarative twin (WS3-003).
var decouplingPresentSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "ground_name", Args: []check.Term{check.Fact{Name: "net.names"}}}}},
		check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}, check.Not{X: check.IsTrue{T: check.Fact{Name: "conn.virtual"}}}}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "capacitor"}}}},
	}},
	Message: "power rail has no decoupling capacitor",
}
