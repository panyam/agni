package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// powerInputNotDriven flags a power-input pin on a net with no power source. See Detail.
var powerInputNotDriven = &check.Rule{
	Name:       "power-input-not-driven",
	Severity:   "error",
	Summary:    "A power-input pin sits on a net with no power source (no power-output and no power flag).",
	Impact:     "A power-input pin (a chip's VCC/VDD) that nothing drives means the part is unpowered, or the rail was drawn but never tied to its regulator. The board either does not come up or a whole section is dead, and it is invisible until first power-on.",
	Primitives:         []string{"select", "traverse", "exists", "pin-role"},
	Reads:              []string{"net.attributes", "on_net", "pin.electrical_type"},
	RequiresCapability: []check.Capability{check.CapTypesPowerOut},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("power-input-not-driven"),
	Eval: func(m check.Model) []check.Finding {
		// "No power source" is only conclusive where the format types power OUTPUTS. On EDIF/IPC (which
		// carry no power_out) a rail's driver reads as a plain input, so the absence is not evidence of
		// unpowered — it would false-fire on every switched/derived rail. WS3-072 PR2 stamps the power_IN
		// side (so decoupling / input-protection work); this rule waits for the power_out stamp (PR3).
		typesPowerOut := m.FormatTypesPowerOut()
		bad := check.Select(m.Nets(), func(n *ir.Net) bool {
			if !typesPowerOut {
				return false // gated: the format types no power outputs (see the design.types_power_out note)
			}
			if n.Attributes[netgraph.AttrPowerDriven] == "true" || n.Attributes[netgraph.AttrExternal] == "true" {
				return false // driven by a power flag, or fed from another sheet we did not read
			}
			dirs := check.NetDirs(m, n)
			hasPowerIn := check.CountDir(dirs, func(d ir.PinDirection) bool { return d == ir.PinDirection_PIN_DIRECTION_POWER_IN }) >= 1
			return hasPowerIn && check.CountDir(dirs, check.IsDriver) == 0
		})
		return check.Report(bad, check.NetFinding("net has a power-input pin but no power source"))
	},
}

// powerInputNotDrivenSpec is the rule's declarative twin (WS3-003). The driver set mirrors
// IsDriver: output, power_out, and inout (a bidirectional drives when enabled).
var powerInputNotDrivenSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.IsTrue{T: check.Fact{Name: "design.types_power_out"}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}},
		check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.In{T: check.Fact{Name: "pin.electrical_type"}, Set: []string{"output", "power_out", "inout"}}}},
	}},
	Message: "net has a power-input pin but no power source",
}
