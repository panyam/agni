package intent

import "strings"

// Emits reports whether ruleName is a rule the intent compiler can produce from some declaration: the
// three fixed-name rules (module-missing, module-count, voltage-domain-mismatch) and the dynamically
// named subsystem-<slug> and protection-<kind> families. It accepts either a bare Rule.Name or the
// composed catalog name ("intent/module-missing").
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
	case RuleModuleMissing, RuleModuleCount, RuleVoltageDomain:
		return true
	}
	return strings.HasPrefix(name, "subsystem-") || strings.HasPrefix(name, "protection-") ||
		strings.HasPrefix(name, "property-")
}
