package intent

import "strings"

// Emits reports whether ruleName is a rule the intent compiler can produce from some declaration,
// either one of the fixed names listed in the switch or a member of the subsystem-, protection-,
// property- and sequence- families. It accepts a bare Rule.Name or the composed catalog name
// ("intent/module-missing").
//
// A review runner uses it to tell a REAL-but-undeclared intent rule from a NOT-YET-SHIPPED name a
// manifest pre-bound (WS3-098). With no --intent-path both resolve to zero catalog rules, but the first
// reads needs-design-intent and the second not-automated, so pre-binding a future name never counts as
// covered. A new rule KIND added to Compile updates Emits beside it; the review package never imports
// intent and gets this as an injected RunParams closure. TestEmitsCoversCompiler holds Emits to
// Compile's actual output.
func Emits(ruleName string) bool {
	name := strings.TrimPrefix(ruleName, SourceName+"/")
	switch name {
	case RuleModuleMissing, RuleModuleCount, RuleVoltageDomain,
		// The fixed names below sit under no family prefix, so each must be listed here or every
		// review item bound to it reads not-automated. Rail sizing (WS3-095):
		RuleRailCurrentCapacity, RuleRailCurrentMargin,
		// The load-switch lower bound (WS3-085).
		RuleLoadSwitchTripBelowBudget,
		// The IO-map rules (agni issue 517).
		RuleIOMapPin, RuleIOMapNetAbsent, RuleIOMapFarEnd, RuleIOMapCoverage,
		// Connector exposure (agni issue 831).
		RuleExposureDeclared:
		return true
	}
	return strings.HasPrefix(name, "subsystem-") || strings.HasPrefix(name, "protection-") ||
		strings.HasPrefix(name, "property-") || strings.HasPrefix(name, "sequence-")
}
