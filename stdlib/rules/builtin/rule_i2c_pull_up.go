package builtin

import (
	"context"
	"regexp"
	"strings"

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
	StatesConsideredSet: true,
}

// i2cPullUpVerdicts returns one verdict per I2C net, and that list IS this rule's considered set.
// No step can drop a subject, so nothing here is NotConsidered.
//
// A pass carries its PATH as the witness ("SCL reaches rail +3V3 through R7"), and the same hops are
// the entities the viewer highlights.
func i2cPullUpVerdicts(ctx context.Context, m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if !isI2C(n.Name) {
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

// i2cNamePattern matches SDA/SCL at a TOKEN boundary rather than as a substring, so each side must be
// a non-letter (start/end, a separator, or a channel digit). RE2 has no lookaround, hence the
// explicit character classes. SPI_SCLK does not match because the trailing K is a letter (WS3-037).
// Case is folded before matching, so [^A-Z] is any non-letter.
var i2cNamePattern = regexp.MustCompile(`(^|[^A-Z])(SDA|SCL)([^A-Z]|$)`)

// isI2C matches the SDA/SCL naming convention at a token boundary. Matches SDA, SCL, I2C_SCL, SCL0,
// SDA_1; NOT SPI_SCLK, SCLK, MCLK, or MISCL (SDA/SCL abutting another letter).
func isI2C(name string) bool {
	return i2cNamePattern.MatchString(strings.ToUpper(name))
}

// i2cPullUpSpec is the rule's declarative twin (WS3-003). The walk goes through the
// pullup_reaches_rail FFI, whose registration in core/check/spec_funcs.go says why the spec language
// cannot express it (agni issue 374).
var i2cPullUpSpec = &check.Spec{
	Over: "nets",
	Where: check.And{Xs: []check.Expr{
		check.Match{T: check.Fact{Name: "net.names"}, Pattern: "(?i)(^|[^A-Z])(SDA|SCL)([^A-Z]|$)"},
		check.Not{X: check.IsTrue{T: check.Call{Fn: "pullup_reaches_rail"}}},
	}},
	Message: "I2C net has no pull-up resistor to a rail",
}
