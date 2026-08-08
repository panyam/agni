package intent

import "strings"

// Emits reports whether ruleName is a rule the intent compiler can produce from some declaration: the
// fixed-name rules (module-missing, module-count, voltage-domain-mismatch, rail-current-capacity,
// rail-current-margin) and the dynamically named subsystem-<slug>, protection-<kind>,
// property-<kind> and sequence-<slug> families. It accepts either a bare Rule.Name or the composed
// catalog name ("intent/module-missing").
//
// It exists so a review runner can tell a REAL-but-undeclared intent rule from a NOT-YET-SHIPPED intent
// rule name a manifest pre-bound (WS3-098). Both resolve to zero catalog rules when no --intent-path is
// supplied, but they must read differently: a real intent rule is needs-design-intent (supply a
// declaration and it flips to pass/fail), while a name the compiler cannot produce (a future rule kind
// like intent/power-sequence) is not-automated, so pre-binding it is safe and does not falsely count as
// covered. Keeping the predicate here gives the intent name space one owner: a new rule KIND added to
// Compile updates Emits beside it, not a string list in the review package (which never imports intent —
// the predicate reaches it as an injected RunParams closure). TestEmitsCoversCompiler holds Emits to
// Compile's actual output so the two cannot drift.
func Emits(ruleName string) bool {
	name := strings.TrimPrefix(ruleName, SourceName+"/")
	switch name {
	case RuleModuleMissing, RuleModuleCount, RuleVoltageDomain,
		// The rail-sizing rules (WS3-095) are fixed names with no shared prefix that the families below
		// would cover, so they have to be listed. Omitting them costs nothing at build time and leaves
		// every item bound to them reading not-automated forever, which is the trap this predicate exists
		// to close.
		RuleRailCurrentCapacity, RuleRailCurrentMargin,
		// Same for the load-switch lower bound (WS3-085): a fixed name under no family prefix, so it has
		// to be listed or a manifest binding it reads not-automated forever.
		RuleLoadSwitchTripBelowBudget:
		return true
	}
	return strings.HasPrefix(name, "subsystem-") || strings.HasPrefix(name, "protection-") ||
		strings.HasPrefix(name, "property-") || strings.HasPrefix(name, "sequence-")
}
