package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// unconnectedPin flags a part-type pin that lands on no net and carries no no-connect
// marking — the per-pin complement of unconnected-component. See Detail.
//
// This rule carries a Go Eval AND a declarative twin although the twin discipline
// (docs/19) says proven-vocabulary rules go spec-only: it introduced NEW interpreter
// vocabulary (the pins entity set, pin.on_net, pin-scope pin.electrical_type), so the Go
// side is the bring-up reference until that vocabulary has more users. Drop it then.
var unconnectedPin = &check.Rule{
	Name:       "unconnected-pin",
	Severity:   "warning",
	Summary:    "A pin lands on no net and is not marked no-connect.",
	Impact:     "A single forgotten pin on an otherwise-wired part is the most common capture slip: an enable left floating, a feedback pin missed, one gate input skipped. unconnected-component only fires when every pin is unwired, so the one-pin miss is invisible to it, and it surfaces at bring-up as a part that almost works.",
	Primitives:         []string{"pin-role", "select", "traverse"},
	Reads:              []string{"net.names", "pin.electrical_type", "pin.no_connect", "pin.on_net"},
	RequiresCapability: []check.Capability{check.CapNoConnectChannel},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("unconnected-pin"),
	Eval: func(m check.Model) []check.Finding {
		bad := []check.PinInst{}
		if m.HasNoConnectChannel() {
			bad = check.Select(m.Pins(), func(p check.PinInst) bool {
				switch m.PinDir(p.Component.RefDes, p.Designator) {
				case ir.PinDirection_PIN_DIRECTION_NO_CONNECT, ir.PinDirection_PIN_DIRECTION_UNSPECIFIED:
					return false
				}
				return !m.PinConnected(p.Component.RefDes, p.Designator)
			})
		}
		return check.Report(bad, func(p check.PinInst) check.Finding {
			return check.Finding{
				Kind:    check.KindPin,
				Subject: p.Component.RefDes,
				Pin:     p.Designator,
				Message: "pin connects to nothing",
				Prov:    p.Component.Prov,
			}
		})
	},
}

// unconnectedPinSpec is the rule's declarative twin; the first spec exercising the pins
// entity set.
var unconnectedPinSpec = &check.Spec{
	Over: "pins",
	Where: check.And{Xs: []check.Expr{
		check.IsTrue{T: check.Fact{Name: "design.nc_channel"}},
		check.Not{X: check.In{T: check.Fact{Name: "pin.electrical_type"}, Set: []string{"no_connect", "unspecified"}}},
		check.Not{X: check.IsTrue{T: check.Fact{Name: "pin.on_net"}}},
	}},
	Message: "pin connects to nothing",
}
