package builtin

import (
	"context"

	"github.com/panyam/agni/core/check"
)

// i2cPullUp flags an I2C net (SDA/SCL) whose bus reaches no rail through a resistor. See Detail.
//
// It walks to the resistor's OTHER end rather than testing membership, so a series termination or
// isolation resistor on the net does not count as a pull-up (agni issue 375).
var i2cPullUp = &check.Rule{
	Name:       "i2c-pull-up",
	Severity:   "error",
	Summary:    "An I2C net (SDA/SCL) reaches no rail through a pull-up resistor.",
	Impact:     "I2C pins are open-drain: they can only pull the line low. With no pull-up the line never returns high, so the bus is stuck and nothing on it communicates. It is a total-function failure and a recurring field bug.",
	Remedy:     "Fit a pull-up resistor from each of SDA and SCL to the bus rail, sized from the bus capacitance and the clock rate the design actually runs at rather than from a habitual value.",
	Primitives: []string{"select", "pattern", "traverse", "exists", "reach"},
	Reads:      []string{"net.names", "on_net", "component.class"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistPublicReference,
	},
	Detail:              ruleDoc("i2c-pull-up"),
	Eval:                i2cPullUpVerdicts,
	Scoping:             check.ScopeSubjects,
	StatesConsideredSet: true,
}

// i2cPullUpVerdicts returns one verdict per I2C net, and that list IS this rule's considered set.
// No step can drop a subject, so nothing here is NotConsidered.
//
// A pass carries its PATH as the witness ("SCL reaches rail +3V3 through R7"), and the same hops are
// the entities the viewer highlights.
func i2cPullUpVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range check.ScopedNets(ctx, m) {
		if !m.IsI2CName(n.Name) {
			continue
		}
		outcome, w, ctx := check.PullUpVerdict(m, n)
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}}, Rule: "i2c-pull-up", Outcome: outcome, Witness: w, Context: ctx}
		if outcome == check.Fail {
			f := check.NetFinding("I2C net has no pull-up resistor to a rail")(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// i2cPullUpSpec is the rule's declarative twin (WS3-003). The walk goes through the
// pullup_reaches_rail FFI, whose registration in core/check/spec_funcs.go says why the spec language
// cannot express it (agni issue 374).
var i2cPullUpSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.IsTrue{T: check.Call{Fn: "i2c_name", Args: []check.Term{check.Fact{Name: "net.names"}}}},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "pullup_reaches_rail"}}},
	}},
	Message: "I2C net has no pull-up resistor to a rail",
}
