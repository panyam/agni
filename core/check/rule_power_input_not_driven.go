package check

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// powerInputNotDriven flags a power-input pin on a net with no power source. See Detail.
var powerInputNotDriven = &Rule{
	Name:       "power-input-not-driven",
	Severity:   "error",
	Summary:    "A power-input pin sits on a net with no power source (no power-output and no power flag).",
	Impact:     "A power-input pin (a chip's VCC/VDD) that nothing drives means the part is unpowered, or the rail was drawn but never tied to its regulator. The board either does not come up or a whole section is dead, and it is invisible until first power-on.",
	Primitives: []string{"select", "traverse", "exists", "pin-role"},
	Reads:      []string{"net.attributes", "on_net", "pin.electrical_type"},
	Tags: map[string]string{
		KeyCategory:     CategoryPower,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("power-input-not-driven"),
	Eval: func(m Model) []Finding {
		// "No power source" is only conclusive where the format types power OUTPUTS. On EDIF/IPC (which
		// carry no power_out) a rail's driver reads as a plain input, so the absence is not evidence of
		// unpowered — it would false-fire on every switched/derived rail. WS3-072 PR2 stamps the power_IN
		// side (so decoupling / input-protection work); this rule waits for the power_out stamp (PR3).
		typesPowerOut := m.FormatTypesPowerOut()
		bad := Select(m.Nets(), func(n *ir.Net) bool {
			if !typesPowerOut {
				return false // gated: the format types no power outputs (see the design.types_power_out note)
			}
			if n.Attributes[netgraph.AttrPowerDriven] == "true" || n.Attributes[netgraph.AttrExternal] == "true" {
				return false // driven by a power flag, or fed from another sheet we did not read
			}
			dirs := NetDirs(m, n)
			hasPowerIn := CountDir(dirs, func(d ir.PinDirection) bool { return d == ir.PinDirection_PIN_DIRECTION_POWER_IN }) >= 1
			return hasPowerIn && CountDir(dirs, IsDriver) == 0
		})
		return Report(bad, NetFinding("net has a power-input pin but no power source"))
	},
}

// powerInputNotDrivenSpec is the rule's declarative twin (WS3-003). The driver set mirrors
// IsDriver: output, power_out, and inout (a bidirectional drives when enabled).
var powerInputNotDrivenSpec = &Spec{
	Over: "nets",
	Where: And{Xs: []Expr{
		IsTrue{T: Fact{"design.types_power_out"}},
		Not{X: IsTrue{T: Fact{"net.attr.power_driven"}}},
		Not{X: IsTrue{T: Fact{"net.attr.external"}}},
		ExistsIn{Over: "net.connections", Where: Cmp{L: Fact{"pin.electrical_type"}, Op: "==", R: Lit{"power_in"}}},
		Not{X: ExistsIn{Over: "net.connections", Where: In{T: Fact{"pin.electrical_type"}, Set: []string{"output", "power_out", "inout"}}}},
	}},
	Message: "net has a power-input pin but no power source",
}
