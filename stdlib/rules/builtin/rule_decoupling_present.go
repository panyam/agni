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

// notARail reports why a net carrying a power-input pin is still not a supply rail, and "" when
// nothing says so. On one multi-thousand-component board all 14 of this rule's findings were false
// positives and eight were these two shapes (agni issue 382). The rule's Detail page covers both.
//
// It checks for a transistor GATE rather than any transistor, because a high-side load switch's
// output carries the FET's source and is a rail that wants decoupling. An inductor disqualifies only
// beside a transistor, since an inductor alone is an LC or ferrite FILTER in front of a rail. The
// committed reach.fires fixture is that shape, so an inductor-alone guard would silence a real finding.
//
// Both checks are class proxies for the buck switch-node topology question in agni issue 374. Replace
// them with that pattern when it lands.
func notARail(m check.Model, n *ir.Net) string {
	var gate, inductor, transistor string
	for _, c := range n.Connections {
		ref := c.ComponentRef
		if m.ComponentClass(ref) == check.ClassTransistor {
			if transistor == "" {
				transistor = ref
			}
			if gate == "" && m.PinRole(ref, c.PinRef) == check.RoleGate {
				gate = ref
			}
		}
		if inductor == "" && m.HasClass(ref, check.ClassInductor) {
			inductor = ref
		}
	}
	switch {
	case gate != "":
		return "the net drives " + gate + "'s gate, so it is a control node rather than a supply rail"
	case inductor != "" && transistor != "":
		return "the net carries inductor " + inductor + " beside transistor " + transistor + ", the shape of a switching node"
	}
	return ""
}

// decouplingPresentVerdicts decides every net that feeds a supply pin AND is a plausible supply rail,
// and that set is the considered set. Four kinds of net get no verdict, because none is a subject of
// a decoupling rule:
//
//   - A net with no power-input pin on it decouples nothing, so it is not a rail.
//   - A GROUND net is the reference the decoupling is measured against. A pass on it would claim
//     ground is adequately decoupled, which this rule cannot support.
//   - A net driving a transistor's GATE is a control node (agni issue 382).
//   - A net carrying an inductor beside a transistor is a switching node (agni issue 382).
//
// An EXTERNAL net is NotConsidered instead. It is a rail that feeds supply pins, and its capacitor may
// be drawn on a sheet this read did not open.
//
// A pass NAMES the capacitor in the witness and in Context, so a reviewer can go and see where it
// sits, and deleting that capacitor changes the witness or flips the verdict.
func decouplingPresentVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		hasPowerIn := check.Exists(n.Connections, func(c *ir.Connection) bool {
			return !check.IsVirtualRef(c.ComponentRef) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
		})
		if !hasPowerIn || m.IsGroundNet(n) {
			continue // not a rail feeding a supply pin, or the reference the decoupling returns to
		}
		if notARail(m, n) != "" {
			continue // a control or switching node, which is not a rail however its pins are typed
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

// decouplingPresentSpec is the rule's declarative twin (WS3-003). It is DELIBERATELY NARROWER than the
// Go body, carrying the capacitor test without the not-a-rail guards, because those read a pin ROLE
// and the spec AST has no pin-role term. Adding one would be an FFI with this rule as its only caller.
//
// TestSpecParity passes only because no committed fixture carries a gate-drive or switching node. A
// fixture of either shape will make the two diverge and fail that test, and that failure is correct.
//
// Reads follows the twin and does NOT list pin.role, although the Go guard consults it, because
// TestSpecMetadata validates Reads against the twin and would fail. The gap is safe, since pin.role
// is RoleUnknown on a format with no pin data and the guard then does not fire. i2c-pull-up's
// Primitives drift the same way (the deferred-work ledger records it), and the fix for both is to
// validate the field against the Go body.
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
