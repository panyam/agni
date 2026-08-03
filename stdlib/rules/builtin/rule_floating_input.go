package builtin

import (
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
	Primitives: []string{"select", "traverse", "count", "exists", "pin-role"},
	Reads:      []string{"component.class", "net.attributes", "net.pin_count", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("floating-input"),
	Eval: func(m check.Model) []check.Finding {
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" {
				return false // the driver may be on another sheet we did not read
			}
			for _, c := range n.Connections {
				if check.IsPassiveClass(m.ComponentClass(c.ComponentRef)) {
					return false // a pull, a filter, or a path we cannot follow: not provably floating
				}
			}
			if len(n.Connections) < 2 {
				return false // a lone input is a single-pin-net finding, not this rule's job
			}
			// Count LOGIC inputs (INPUT direction, excluding diode terminals): a diode/LED/TVS
			// terminal is typed INPUT by some libraries but is not a logic input, so a pure diode
			// network must not read as floating. Excluding it per-pin (not per-net) keeps a real
			// floating IC input that merely carries a clamp diode firing.
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
			return logicInputs >= 1 && ncOrIn == len(n.Connections)
		})
		return check.Report(bad, check.NetFinding("net carries only input pins; nothing drives it"))
	},
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
