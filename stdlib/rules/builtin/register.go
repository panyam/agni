// Package builtin is the standard EE rule catalog: one file per rule (rule_*.go), each a
// check.Rule value carrying its documentation and an Eval, plus the declarative-twin Specs that
// the parity tests hold to those Evals. It moved out of package check in phase 2b of the package
// reorg so the built-in rules register through the same public seam an overlay uses — the core
// engine (package check) owns no rules. Blank-importing this package installs the catalog as the
// anonymous built-in source; a program that omits the import runs with no built-ins.
package builtin

import (
	"github.com/panyam/agni/core/check"
)

// init installs the built-in catalog with the engine by import side effect, the way an overlay
// suite calls check.RegisterSource. It runs before an importing package's own var initializers,
// so any binary that blank-imports this package has the built-ins available at startup.
func init() {
	check.RegisterBuiltins(rules, specs)
}

// rules is the built-in rule set, in evaluation order. This is the single registry: adding a
// rule means adding one rule_*.go file and one line here. check.Run evaluates these; the catalog
// groups them for display.
var rules = []*check.Rule{
	singlePinNet,
	unconnectedComponent,
	unconnectedPin,
	danglingEndpoint,
	wireNoJunction,
	duplicateRefDes,
	outputOutputConflict,
	ncPinConnected,
	unspecifiedPinWithDriver,
	duplicateNetName,
	labelAliasConflict,
	powerTapConflict,
	testPointCoverage,
	floatingInput,
	powerInputNotDriven,
	decouplingPresent,
	bulkCap,
	crystalLoadCaps,
	resonatorRedundantLoadCaps,
	inputProtection,
	esdProtection,
	esdClampNotTVS,
	i2cPullUp,
	supplyExceedsAbsMax,
	regulatorOutputExceedsAbsMax,
	fetVdssBelowRail,
	loadSwitchTripAboveFetRating,
	reverseBlockingAbsent,
	railNominalOutOfRecommended,
	pinExceedsAbsMax,
	pinOutOfRecommended,
	pinTrackingViolated,
	pinTrackingAdvisory,
	railNotClassified,
	capVoltage,
	diffPairNaming,
	trackWidth,
	netclassTrackWidth,
	netclassViaDrill,
	holeSize,
	annularWidth,
	copperClearance,
	ledPolarity,
	pinNetConflict,
	busNotModeled,
	symbolUnresolved,
}

// specs maps each Go-eval'd rule to its declarative twin: the same rule body as a check.Spec value
// (WS3-003, docs/19 "A rule is a value"). For these rules the Go Eval stays canonical — check.Run
// evaluates it, not the twin — while the twin is held to it by TestSpecParity (identical findings
// on every fixture) and TestSpecMetadata (the rule's hand-written Reads/Primitives equal the
// spec's derived ones). Flipping a rule to spec-canonical is replacing its Eval with its twin's,
// gated by the parity test — output-output-conflict was the first flip (rule_pin_matrix.go) and
// therefore no longer appears here. Spec-only rules (the matrix rows, and future rules on proven
// vocabulary per the docs/19 twin discipline) never do: their Eval IS the interpreter. Benchmarks
// comparing the two evaluation paths live in spec_bench_test.go and feed the WS3-004 fact-store
// decision.
var specs = map[string]*check.Spec{
	"single-pin-net":         singlePinNetSpec,
	"unconnected-component":  unconnectedComponentSpec,
	"unconnected-pin":        unconnectedPinSpec,
	"wire-no-junction":       wireNoJunctionSpec,
	"dangling-endpoint":      danglingEndpointSpec,
	"duplicate-ref-des":      duplicateRefDesSpec,
	"floating-input":         floatingInputSpec,
	"power-input-not-driven": powerInputNotDrivenSpec,
	"decoupling-present":     decouplingPresentSpec,
	"bulk-cap":               bulkCapSpec,
	"input-protection":       inputProtectionSpec,
	"esd-protection":         esdProtectionSpec,
	"esd-clamp-not-tvs":      esdClampNotTVSSpec,
	"i2c-pull-up":            i2cPullUpSpec,
	"diff-pair-naming":       diffPairNamingSpec,
}
