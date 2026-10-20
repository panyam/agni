package builtin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// floatingInput flags a net whose only pins are inputs, so nothing drives them. See Detail.
var floatingInput = &check.Rule{
	Name:       "floating-input",
	Severity:   "warning",
	Summary:    "An input pin sits on a net with no driver and no pull, so its level is undefined.",
	Impact:     "A floating logic input drifts, picks up noise, and can oscillate or sit in the forbidden region where both transistors of a CMOS stage conduct. Behavior is non-deterministic and often temperature- and board-dependent, the worst kind of intermittent bug.",
	Remedy:     "Tie the input to its inactive level through a pull-up or pull-down, or drive it from the logic that was meant to. A CMOS input is never safe to leave floating, including on a pin the firmware does not use.",
	Primitives: []string{"select", "traverse", "count", "exists", "pin-role"},
	Reads:      []string{"component.class", "component.dnp", "net.attributes", "net.pin_count", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("floating-input"),
	Eval:                floatingInputVerdicts,
	Scoping:             check.ScopeSubjects,
	StatesConsideredSet: true,
}

// floatingInputVerdicts decides every net that carries a LOGIC INPUT and more than one pin. A net
// with no logic input is not this rule's subject, and a one-pin net belongs to single-pin-net, so
// neither yields a verdict.
//
// Two exemptions are NotConsidered rather than passes (#400):
//
//   - An EXTERNAL net continues onto a sheet this read did not open, so its driver may exist and be
//     invisible here.
//   - A net carrying a PASSIVE part is not provably floating, which is a weaker claim than not
//     floating. The part may be the pull-up that fixes it, a series element with the driver on the
//     far side, or a footprint nobody stuffed. Context names it.
//
// Any other net with a pin that is neither an input nor a no-connect passes. The pass witness counts
// those pins, so retyping the driver as an input drops the count to zero and flips the verdict.
func floatingInputVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range check.ScopedNets(ctx, m) {
		// A diode/LED/TVS terminal is typed INPUT by some libraries but is not a logic input, so it
		// counts toward ncOrIn and not logicInputs. Excluding it per pin rather than per net keeps a
		// floating IC input that carries a clamp diode firing.
		//
		// A part marked do-not-populate has no pins on the assembled board, so only fitted parts are
		// counted (agni issue 1082). An input whose only pull is unfitted is then alone on its net,
		// which is the floating input this rule exists for. A connector counts either way, because its
		// pads are the board's interface whether or not a header is soldered to them, and a board
		// shipped with its header loose marks it DNP.
		logicInputs, ncOrIn, pins, unfittedInputs := 0, 0, 0, 0
		for _, c := range n.Connections {
			dir := check.ConnDir(m, c)
			if !m.IsFitted(c.ComponentRef) && !m.HasClass(c.ComponentRef, check.ClassConnector) {
				if dir == ir.PinDirection_PIN_DIRECTION_INPUT && !m.HasClass(c.ComponentRef, check.ClassDiode) {
					unfittedInputs++
				}
				continue
			}
			pins++
			switch dir {
			case ir.PinDirection_PIN_DIRECTION_INPUT:
				ncOrIn++
				if !m.HasClass(c.ComponentRef, check.ClassDiode) {
					logicInputs++
				}
			case ir.PinDirection_PIN_DIRECTION_NO_CONNECT:
				ncOrIn++
			}
		}
		if len(n.Connections) < 2 || logicInputs+unfittedInputs < 1 {
			continue // single-pin-net's finding, or nothing to float
		}

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		passive := firstPassive(m, n)
		switch {
		case logicInputs < 1:
			v.Outcome = check.NotConsidered
			v.Reason = "every logic input on the net belongs to a part marked do-not-populate, so nothing on the assembled board listens to it"
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the net continues onto a sheet this read did not open, so a driver may exist outside it"
		case passive != "":
			v.Outcome = check.NotConsidered
			v.Reason = "the net carries a passive part, which may be the pull that fixes it or a series element hiding the driver, so it is not provably floating"
			v.Context = compContext(passive, "passive on the net")
		case ncOrIn == pins:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("all %d of the net's fitted pins are inputs or no-connects, and %d of them are logic inputs", pins, logicInputs),
				Terms: []check.WitnessTerm{
					{Label: "pins", Value: strconv.Itoa(pins)},
					{Label: "logic inputs", Value: strconv.Itoa(logicInputs)},
				},
			}
			f := check.NetFinding("net carries only input pins; nothing drives it")(n)
			v.Finding = &f
		default:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%d of the net's %d fitted pins are neither input nor no-connect, so the net is not input-only", pins-ncOrIn, pins),
				Terms: []check.WitnessTerm{
					{Label: "pins", Value: strconv.Itoa(pins)},
					{Label: "non-input pins", Value: strconv.Itoa(pins - ncOrIn)},
				},
			}
		}
		out = append(out, v)
	}
	return out
}

// notUnfitted holds for a part on the assembled board, which is a fitted part or a connector whose
// pads stay the board's interface when its header is not fitted, as floatingInputVerdicts counts.
var notUnfitted = check.Not{X: check.And{Xs: []check.Expr{
	check.IsTrue{T: check.Fact{Name: "component.dnp"}},
	check.Not{X: check.Cmp{L: check.Fact{Name: "component.class"}, Op: "==", R: check.Lit{V: "connector"}}},
}}}

// floatingInputSpec is the rule's declarative twin (WS3-003). Diode terminals are excluded from
// the logic-input count and still count toward "all pins input or no-connect", as in
// floatingInputVerdicts.
var floatingInputSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{
			check.In{T: check.Fact{Name: "component.class"}, Set: []string{"resistor", "capacitor", "inductor", "ferrite", "fuse", "test_point"}},
			notUnfitted,
		}}}},
		check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: ">=", R: check.Lit{V: 2}},
		check.Cmp{L: check.CountOf{Over: "net.connections", Where: check.And{Xs: []check.Expr{
			check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "input"}},
			check.Not{X: check.In{T: check.Fact{Name: "component.class"}, Set: []string{"diode", "led", "tvs"}}},
			notUnfitted,
		}}}, Op: ">=", R: check.Lit{V: 1}},
		check.Cmp{L: check.CountOf{Over: "net.connections", Where: check.And{Xs: []check.Expr{
			check.In{T: check.Fact{Name: "pin.electrical_type"}, Set: []string{"input", "no_connect"}},
			notUnfitted,
		}}}, Op: "==", R: check.CountOf{Over: "net.connections", Where: notUnfitted}},
	}},
	Message: "net carries only input pins; nothing drives it",
}
