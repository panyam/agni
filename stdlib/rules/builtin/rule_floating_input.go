package builtin

import (
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
	Reads:      []string{"component.class", "net.attributes", "net.pin_count", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("floating-input"),
	Eval:                floatingInputVerdicts,
	StatesConsideredSet: true,
}

// floatingInputVerdicts decides every net that carries a LOGIC INPUT and more than one pin, and that
// is the considered set. A net with no logic input is not a subject of a floating-input rule, and a
// one-pin net belongs to single-pin-net, so neither yields a verdict: reporting them as passes would
// claim a check that was never made, and reporting the stub twice would put one defect under two
// names.
//
// THE TWO EXEMPTIONS BECOME NotConsidered RATHER THAN PASSES, and the distinction is the substance
// of the conversion. Both used to leave through the same silent `return false` a driven net did, so
// downstream they were one answer. They are three:
//
//   - An EXTERNAL net continues onto a sheet this read did not open, so its driver may exist and be
//     invisible here. The rule has not cleared it; it cannot see it.
//   - A net carrying a PASSIVE part is not provably floating, which is a weaker claim than not
//     floating. The resistor may be the pull-up that fixes it, or a series element with the driver on
//     the far side, or a footprint nobody stuffed. The rule cannot tell those apart, so it says so and
//     names the part in Context.
//   - A net with a pin that is neither an input nor a no-connect genuinely passes: something on it
//     can drive.
//
// The pass witness counts the non-input pins, so it tracks the fact rather than restating the
// outcome. Retype the driver as an input and the count falls to zero and the verdict flips.
func floatingInputVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		// Count LOGIC inputs (INPUT direction, excluding diode terminals): a diode/LED/TVS terminal is
		// typed INPUT by some libraries but is not a logic input, so a pure diode network must not read
		// as floating. Excluding it per-pin (not per-net) keeps a real floating IC input that merely
		// carries a clamp diode firing.
		logicInputs, ncOrIn := 0, 0
		for _, c := range n.Connections {
			switch check.ConnDir(m, c) {
			case ir.PinDirection_PIN_DIRECTION_INPUT:
				ncOrIn++
				if !m.HasClass(c.ComponentRef, check.ClassDiode) {
					logicInputs++
				}
			case ir.PinDirection_PIN_DIRECTION_NO_CONNECT:
				ncOrIn++
			}
		}
		if logicInputs < 1 || len(n.Connections) < 2 {
			continue // not this rule's subject: nothing to float, or single-pin-net's finding
		}

		v := check.Verdict{Kind: check.KindNet, Subject: n.Name, NetID: n.GetId()}
		passive := firstPassive(m, n)
		switch {
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the net continues onto a sheet this read did not open, so a driver may exist outside it"
		case passive != "":
			v.Outcome = check.NotConsidered
			v.Reason = "the net carries a passive part, which may be the pull that fixes it or a series element hiding the driver, so it is not provably floating"
			v.Context = compContext(passive, "passive on the net")
		case ncOrIn == len(n.Connections):
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("all %d of the net's pins are inputs or no-connects, and %d of them are logic inputs", len(n.Connections), logicInputs),
				Terms: []check.WitnessTerm{
					{Label: "pins", Value: strconv.Itoa(len(n.Connections))},
					{Label: "logic inputs", Value: strconv.Itoa(logicInputs)},
				},
			}
			f := check.NetFinding("net carries only input pins; nothing drives it")(n)
			v.Finding = &f
		default:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%d of the net's %d pins are neither input nor no-connect, so the net is not input-only", len(n.Connections)-ncOrIn, len(n.Connections)),
				Terms: []check.WitnessTerm{
					{Label: "pins", Value: strconv.Itoa(len(n.Connections))},
					{Label: "non-input pins", Value: strconv.Itoa(len(n.Connections) - ncOrIn)},
				},
			}
		}
		out = append(out, v)
	}
	return out
}

// floatingInputSpec is the rule's declarative twin (WS3-003): any passive member exempts
// the net (see the Detail), then "all pins are input or no-connect" is the count of such
// pins equalling the net's pin count. A diode/LED/TVS terminal is typed INPUT by some
// libraries but is not a logic input, so it is excluded from the ">= 1 logic input" gate (a
// pure diode network must not read as floating); it still counts toward "all pins input/nc".
var floatingInputSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.In{T: check.Fact{Name: "component.class"}, Set: []string{"resistor", "capacitor", "inductor", "ferrite", "fuse", "test_point"}}}},
		check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: ">=", R: check.Lit{V: 2}},
		check.Cmp{L: check.CountOf{Over: "net.connections", Where: check.And{Xs: []check.Expr{
			check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "input"}},
			check.Not{X: check.In{T: check.Fact{Name: "component.class"}, Set: []string{"diode", "led", "tvs"}}},
		}}}, Op: ">=", R: check.Lit{V: 1}},
		check.Cmp{L: check.CountOf{Over: "net.connections", Where: check.In{T: check.Fact{Name: "pin.electrical_type"}, Set: []string{"input", "no_connect"}}}, Op: "==", R: check.Fact{Name: "net.pin_count"}},
	}},
	Message: "net carries only input pins; nothing drives it",
}
