package builtin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// powerInputNotDriven flags a power-input pin on a net with no power source. See Detail.
var powerInputNotDriven = &check.Rule{
	Name:               "power-input-not-driven",
	Severity:           "error",
	Summary:            "A power-input pin sits on a net with no power source (no power-output and no power flag).",
	Impact:             "A power-input pin (a chip's VCC/VDD) that nothing drives means the part is unpowered, or the rail was drawn but never tied to its regulator. The board either does not come up or a whole section is dead, and it is invisible until first power-on.",
	Remedy:             "Connect the supply pin to the rail that feeds it, and check that the rail itself reaches its regulator. An undriven VDD is usually a missed wire rather than a missing supply.",
	Primitives:         []string{"select", "traverse", "exists", "pin-role"},
	Reads:              []string{"net.attributes", "on_net", "pin.electrical_type"},
	RequiresCapability: []check.Capability{check.CapTypesPowerOut},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryPower,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("power-input-not-driven"),
	Eval:                powerInputNotDrivenVerdicts,
	Scoping:             check.ScopeSubjects,
	StatesConsideredSet: true,
}

// powerInputNotDrivenVerdicts decides every net carrying a power-INPUT pin, and that set is the
// considered set. A net with no supply pin on it is not a subject of a supply rule and yields no
// verdict, since a pass about a subject the rule never asked about claims a check that did not happen.
//
// THE FORMAT GATE IS NotConsidered. "No power source" is only conclusive where the format types power
// OUTPUTS. EDIF and IPC-2581 carry no power_out, so a rail's own regulator reads as a plain input there
// and the rule would fire on every switched or derived rail. A SILENT gate would read the same as a
// board whose rails are all properly fed, so each gated net gets a verdict saying why. `Run` never
// consults RequiresCapability, so the rule-level declaration reaches only a caller that asks
// Available first, and this reaches every caller.
//
// The EXTERNAL exemption is NotConsidered for the same reason it is in floating-input, since the feed
// may be on a sheet this read did not open. The power-flag exemption is a genuine PASS, because a
// power flag is the designer stating the net is driven.
func powerInputNotDrivenVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	typesPowerOut := m.FormatTypesPowerOut()

	var out []check.Verdict
	for _, n := range check.ScopedNets(ctx, m) {
		dirs := check.NetDirs(m, n)
		powerIns := check.CountDir(dirs, func(d ir.PinDirection) bool { return d == ir.PinDirection_PIN_DIRECTION_POWER_IN })
		if powerIns == 0 {
			continue // no supply pin here, so this net is not a subject of a supply rule
		}
		// On a net carrying a power symbol only a power source feeds its supply pins, as KiCad's ERC
		// holds, because a logic output there is a signal pin tied to the rail. That passed a ground fed
		// by four strap pins until agni issue 1082. Elsewhere any driver counts, since a supply-typed pin
		// on an ordinary net (a module connector's power-good input, say) is fed by whatever drives it.
		feeds := check.IsDriver
		powerNet := powerSymbolNet(m, n)
		if powerNet {
			feeds = func(d ir.PinDirection) bool { return d == ir.PinDirection_PIN_DIRECTION_POWER_OUT }
		}
		drivers := check.CountDir(dirs, feeds)

		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}}
		switch {
		case !typesPowerOut:
			v.Outcome = check.NotConsidered
			v.Reason = "the source format does not type power-output pins, so an absent driver is not evidence that the rail is unfed"
		case n.Attributes[netgraph.AttrExternal] == "true":
			v.Outcome = check.NotConsidered
			v.Reason = "the net continues onto a sheet this read did not open, so its feed may exist outside it"
		case n.Attributes[netgraph.AttrPowerDriven] == "true":
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("a power flag on the net asserts it is driven, feeding %d power-input pin(s)", powerIns),
				Terms: []check.WitnessTerm{
					{Label: "power inputs", Value: strconv.Itoa(powerIns)},
					{Label: "source", Value: "power flag"},
				},
			}
		case drivers > 0:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%d driving pin(s) on the net feed its %d power-input pin(s)", drivers, powerIns),
				Terms: []check.WitnessTerm{
					{Label: "power inputs", Value: strconv.Itoa(powerIns)},
					{Label: "drivers", Value: strconv.Itoa(drivers)},
				},
			}
		case notASupply(m, n) != "":
			// A lone exposed pad and a divider tap carry a power_in pin and draw no supply, so an absent
			// driver says nothing about an unfed rail (agni issue 935).
			v.Outcome = check.NotConsidered
			v.Reason = notASupply(m, n)
		default:
			v.Outcome = check.Fail
			statement := fmt.Sprintf("%d power-input pin(s) sit on the net and no pin on it drives", powerIns)
			if powerNet {
				statement = fmt.Sprintf("%d power-input pin(s) sit on a net carrying a power symbol, and no power-output pin or power flag feeds them", powerIns)
			}
			v.Witness = &check.Witness{
				Statement: statement,
				Terms: []check.WitnessTerm{
					{Label: "power inputs", Value: strconv.Itoa(powerIns)},
					{Label: "drivers", Value: "0"},
				},
			}
			f := check.NetFinding("net has a power-input pin but no power source")(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// powerSymbolNet reports whether a net carries a power symbol, a virtual power-input pin such as a
// KiCad #PWR, which makes it a named supply net that only a power source can feed.
func powerSymbolNet(m check.Model, n *ir.Net) bool {
	return check.Exists(n.Connections, func(c *ir.Connection) bool {
		return check.IsVirtualRef(c.ComponentRef) && check.ConnDir(m, c) == ir.PinDirection_PIN_DIRECTION_POWER_IN
	})
}

// powerInputNotDrivenSpec is the rule's declarative twin (WS3-003). A net carrying a power symbol
// needs a power_out pin; any other net needs any driver, as the Go rule decides.
var powerInputNotDrivenSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.IsTrue{T: check.Fact{Name: "design.types_power_out"}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.power_driven"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "net.attr.external"}}},
		check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}},
		check.Or{Xs: []check.Expr{
			check.And{Xs: []check.Expr{
				check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "conn.virtual"}}, check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}}}},
				check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_out"}}}},
			}},
			check.And{Xs: []check.Expr{
				check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.And{Xs: []check.Expr{check.IsTrue{T: check.Fact{Name: "conn.virtual"}}, check.Cmp{L: check.Fact{Name: "pin.electrical_type"}, Op: "==", R: check.Lit{V: "power_in"}}}}}},
				check.Not{X: check.ExistsIn{Over: "net.connections", Where: check.In{T: check.Fact{Name: "pin.electrical_type"}, Set: []string{"output", "power_out", "inout", "tri_state", "open_collector", "open_emitter"}}}},
			}},
		}},
	}},
	Message: "net has a power-input pin but no power source",
}
