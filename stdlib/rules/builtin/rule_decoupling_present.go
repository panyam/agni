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

// notARail reports why a net carrying a power-input pin is nonetheless not a supply rail, and "" when
// nothing says so. Both shapes were measured rather than guessed: on one multi-thousand-component
// board every one of this rule's 14 findings was a false positive, and eight were these two
// (agni issue 382).
//
// THE GATE, NOT THE TRANSISTOR. A gate-drive net carries a controller output and a FET's GATE, and it
// is a control node however the parts on it are typed. Disqualifying any net with a transistor on it
// would be simpler and wrong: a high-side load switch's OUTPUT carries the FET's source, feeds
// downstream supply pins, and wants decoupling exactly as much as a regulator output does. The role
// vocabulary separates the two, so the rule asks the narrow question.
//
// THE INDUCTOR ONLY ALONGSIDE A TRANSISTOR. A buck's switch node carries the switching FET and the
// inductor, and advising a capacitor there shorts the switch to ground every cycle, which is worse
// than saying nothing. An inductor on its own is an LC or ferrite FILTER, and a filtered supply is a
// rail that wants decoupling on the far side. The committed reach.fires fixture is that shape, so an
// inductor-alone guard would silence a real finding to catch a case the gate guard already catches.
//
// BOTH ARE A PROXY for a question the rule cannot ask yet. "This is the switching node of a buck
// converter" is a topology question, an inductor between a switch node and an output with the
// capacitor beyond it, which is what agni issue 374 is for. Whoever lands that should come back and
// replace these class checks with the pattern.
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
// and that set is the considered set. Four kinds of net yield no verdict at all, because none is a
// subject of a decoupling rule rather than being one the rule cleared:
//
//   - A net with no power-input pin on it decouples nothing. The rule is about rails that feed a
//     chip, and a signal net is not a rail with no capacitor; it is not a rail.
//   - A GROUND net is the reference the decoupling is measured against, not the thing decoupled.
//     Reporting it as a pass would state that ground is adequately decoupled, which is not a claim
//     this rule makes or could support.
//   - A net driving a transistor's GATE is a control node (agni issue 382).
//   - A net carrying an inductor beside a transistor is a switching node (agni issue 382).
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

// decouplingPresentSpec is the rule's declarative twin (WS3-003), and it is DELIBERATELY NARROWER
// than the Go body: it carries the capacitor test and not the not-a-rail scope guards.
//
// The guards read a transistor pin's ROLE, and the spec AST has no pin-role term. Expressing them
// would mean a new FFI whose only caller is this rule, which is the shape the WS3-003 earn-it guard
// exists to refuse. So the Go body stays canonical here, as it does for the reach-walk rules, and the
// twin states the comparison rather than the scope.
//
// TestSpecParity compares the two over the fixtures and passes because no committed fixture carries a
// gate-drive or switching node. That is the honest reading of a narrower twin rather than a
// coincidence to rely on: a fixture of either shape added later would make them diverge, and the test
// failing is the correct outcome, pointing whoever adds it at this comment.
//
// Reads therefore stays as the twin derives it and does NOT list pin.role, which the Go guard does
// consult. TestSpecMetadata validates that field against the twin, so declaring the extra read fails
// the build. The gap is real and safe in the direction that matters: pin.role degrades to
// RoleUnknown on a format carrying no pin data, so the guard simply does not fire there rather than
// gating the rule off a design it could still answer. It is the same drift the deferred-work ledger
// already records for i2c-pull-up's Primitives, and it wants the same fix, which is for the field to
// be validated against the Go body rather than against the twin.
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
