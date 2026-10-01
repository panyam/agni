package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// unconnectedPin flags a part-type pin that lands on no net and carries no no-connect
// marking, the per-pin complement of unconnected-component. See Detail.
//
// It keeps a Go Eval beside its twin because it introduced NEW interpreter vocabulary (the pins
// entity set, pin.on_net, pin-scope pin.electrical_type), so the Go side is the bring-up reference
// until that vocabulary has more users. Twin discipline is in docsite/content/build/check-rule.md.
var unconnectedPin = &check.Rule{
	Name:       "unconnected-pin",
	Severity:   "warning",
	Summary:    "A pin lands on no net and is not marked no-connect.",
	Impact:     "A single forgotten pin on an otherwise-wired part is the most common capture slip: an enable left floating, a feedback pin missed, one gate input skipped. unconnected-component only fires when every pin is unwired, so the one-pin miss is invisible to it, and it surfaces at bring-up as a part that almost works.",
	Remedy:     "Connect the pin to its intended net, or mark it no-connect where the datasheet says the pin may be left open.",
	Primitives:         []string{"pin-role", "select", "traverse"},
	Reads:              []string{"net.names", "pin.electrical_type", "pin.no_connect", "pin.on_net"},
	RequiresCapability: []check.Capability{check.CapNoConnectChannel},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:       ruleDoc("unconnected-pin"),
	Eval: unconnectedPinVerdicts,
	StatesConsideredSet: true,
}

// unconnectedPinVerdicts decides every pin in the design and returns one verdict each.
//
// THE TWO SKIPPED PIN TYPES ARE NOT THE SAME ANSWER (#395):
//
//   - NO_CONNECT is a PASS. The symbol declares the pin may be left open, so an unwired pin is the
//     design working as documented.
//   - UNSPECIFIED is NotConsidered. The symbol never states what the pin is, so the rule cannot tell
//     a deliberate open pin from a forgotten one, and a pass would claim assurance the run lacks.
//
// The no-connect CHANNEL gate is left to the caller. RequiresCapability already makes this rule
// not-applicable where the format cannot express a no-connect, and the run reports it in its skipped
// list, so a considered set here would duplicate that and could disagree with it. The nil return
// covers a caller reaching EvalVerdicts directly.
func unconnectedPinVerdicts(m check.Model) []check.Verdict {
	if !m.HasNoConnectChannel() {
		return nil
	}
	var out []check.Verdict
	for _, p := range m.Pins() {
		ref, pin := p.Component.RefDes, p.Designator
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindPin, Ref: ref, Pin: pin}}}
		switch m.PinDir(ref, pin) {
		case ir.PinDirection_PIN_DIRECTION_NO_CONNECT:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{Statement: "pin is marked no-connect, so leaving it unwired is what the symbol asks for"}
		case ir.PinDirection_PIN_DIRECTION_UNSPECIFIED:
			v.Outcome = check.NotConsidered
			v.Reason = "the symbol declares no electrical type for this pin, so a deliberate open pin and a forgotten one read the same"
		default:
			if m.PinConnected(ref, pin) {
				v.Outcome = check.Pass
				v.Witness = &check.Witness{Statement: "pin lands on a net"}
				break
			}
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: "pin lands on no net and the symbol does not mark it no-connect"}
			f := check.Finding{Subject: check.Entity{Kind: check.KindPin, Ref: ref, Pin: pin}, Message: "pin connects to nothing", Prov: p.Component.Prov}
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// unconnectedPinSpec is the rule's declarative twin, and the first spec exercising the pins
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
