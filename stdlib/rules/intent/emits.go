package intent

import "strings"

// Emits reports whether ruleName is a rule the intent compiler can produce from some declaration,
// either one of the fixed names listed in the switch or a member of the subsystem-, protection-,
// property-, sequence- and strap-group- families. It accepts a bare Rule.Name or the composed catalog name
// ("intent/module-missing").
//
// A review runner uses it to tell a REAL-but-undeclared intent rule from a NOT-YET-SHIPPED name that
// a manifest pre-bound (WS3-098). With no --intent-path both resolve to zero catalog rules. The first
// reads needs-design-intent and the second not-automated, so pre-binding a future name never counts
// as covered. When Compile gains a new rule KIND, Emits gains it in the same change. The review
// package never imports intent, and it gets this as an injected RunParams closure.
// TestEmitsCoversCompiler holds Emits to Compile's actual output.
func Emits(ruleName string) bool {
	name := strings.TrimPrefix(ruleName, SourceName+"/")
	switch name {
	case RuleModuleMissing, RuleModuleCount, RuleVoltageDomain,
		// The fixed names below sit under no family prefix, so each must be listed here or every
		// review item bound to it reads not-automated. The rail-sizing pair (WS3-095) follows.
		RuleRailCurrentCapacity, RuleRailCurrentMargin,
		// The load-switch lower bound (WS3-085).
		RuleLoadSwitchTripBelowBudget,
		// The IO-map rules (agni issue 517).
		RuleIOMapPin, RuleIOMapNetAbsent, RuleIOMapFarEnd, RuleIOMapCoverage,
		// The pin-function check (agni issue 667) and the strap address collision.
		RuleIOMapFunction, RuleStrapAddressCollision,
		// Connector exposure (agni issue 831) and pull-up reach (agni issue 1165).
		RuleExposureDeclared, RulePullUpReachDeclared:
		return true
	}
	return strings.HasPrefix(name, "subsystem-") || strings.HasPrefix(name, "protection-") ||
		strings.HasPrefix(name, "property-") || strings.HasPrefix(name, "sequence-") ||
		strings.HasPrefix(name, "strap-group-")
}
