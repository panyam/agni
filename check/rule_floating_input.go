package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// floatingInput flags a net whose only pins are inputs, so nothing drives them. See Detail.
var floatingInput = &Rule{
	Name:       "floating-input",
	Severity:   "warning",
	Summary:    "An input pin sits on a net with no driver and no pull, so its level is undefined.",
	Impact:     "A floating logic input drifts, picks up noise, and can oscillate or sit in the forbidden region where both transistors of a CMOS stage conduct. Behavior is non-deterministic and often temperature- and board-dependent, the worst kind of intermittent bug.",
	Primitives: []string{"select", "traverse", "count", "exists", "pin-role"},
	Reads:      []string{"component.class", "net.attributes", "net.pin_count", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("floating-input"),
	Eval: func(m Model) []Finding {
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			if n.Attributes[netgraph.AttrExternal] == "true" {
				return false // the driver may be on another sheet we did not read
			}
			for _, c := range n.Connections {
				if isPassiveClass(m.ComponentClass(c.ComponentRef)) {
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
				switch connDir(m, c) {
				case ir.PinDirection_PIN_DIRECTION_INPUT:
					ncOrIn++
					if !m.HasClass(c.ComponentRef, ClassDiode) {
						logicInputs++
					}
				case ir.PinDirection_PIN_DIRECTION_NO_CONNECT:
					ncOrIn++
				}
			}
			return logicInputs >= 1 && ncOrIn == len(n.Connections)
		})
		return Report(bad, netFinding("net carries only input pins; nothing drives it"))
	},
}

// floatingInputSpec is the rule's declarative twin (WS3-003): any passive member exempts
// the net (see the Detail), then "all pins are input or no-connect" is the count of such
// pins equalling the net's pin count. A diode/LED/TVS terminal is typed INPUT by some
// libraries but is not a logic input, so it is excluded from the ">= 1 logic input" gate (a
// pure diode network must not read as floating); it still counts toward "all pins input/nc".
var floatingInputSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		Not{X: ExistsIn{Over: "net.connections", Where: In{T: Fact{"component.class"}, Set: []string{"resistor", "capacitor", "inductor", "ferrite", "fuse", "test_point"}}}},
		Cmp{L: Fact{"net.pin_count"}, Op: ">=", R: Lit{2}},
		Cmp{L: CountOf{Over: "net.connections", Where: And{Xs: []Expr{
			Cmp{L: Fact{"pin.electrical_type"}, Op: "==", R: Lit{"input"}},
			Not{X: In{T: Fact{"component.class"}, Set: []string{"diode", "led", "tvs"}}},
		}}}, Op: ">=", R: Lit{1}},
		Cmp{L: CountOf{Over: "net.connections", Where: In{T: Fact{"pin.electrical_type"}, Set: []string{"input", "no_connect"}}}, Op: "==", R: Fact{"net.pin_count"}},
	}},
	Message: "net carries only input pins; nothing drives it",
}
