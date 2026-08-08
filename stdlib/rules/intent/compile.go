package intent

import "github.com/panyam/agni/core/check"

// Rule names the intent rules emit under. These are the BARE names (Rule.Name may not contain the
// catalog's "/" namespace separator); Source("intent", ...) exposes them in the catalog as
// "intent/module-missing" and "intent/voltage-domain-mismatch", which is what a review manifest binds
// to (rule: intent/module-missing). They are stable so an item flips to pass/fail once a declaration
// is loaded.
const (
	RuleModuleMissing = "module-missing"
	RuleModuleCount   = "module-count"
	RuleVoltageDomain = "voltage-domain-mismatch"
	// RuleRailCurrentCapacity and RuleRailCurrentMargin are the two rail-sizing rules (WS3-095). They
	// run one mechanism at two thresholds and are two rules so that a "regulator output ratings" item
	// and a "current capability margins" item report independently (WS3-058).
	RuleRailCurrentCapacity = "rail-current-capacity"
	RuleRailCurrentMargin   = "rail-current-margin"
	// SourceName is the namespace Source uses; the composed catalog names are SourceName + "/" + the
	// bare rule name.
	SourceName = "intent"
)

// Compile turns a Declaration into the set of check rules that verify a design against it. It mirrors
// profiles.Compile: the declaration is config (per-design data), the rules are code. Each sub-check is
// emitted only when the declaration carries the data it needs, so a declaration with modules but no
// voltage domains compiles to just the module rule (no empty rule that silently passes). The rules
// close over the declaration and read the design through check.Model (C19), so they are
// netlist-format-neutral.
func Compile(d Declaration) []*check.Rule {
	var rules []*check.Rule
	if len(d.Modules) > 0 {
		rules = append(rules, moduleMissingRule(d))
	}
	if anyModuleHasCount(d) {
		rules = append(rules, moduleCountRule(d))
	}
	if len(d.VoltageDomains) > 0 {
		rules = append(rules, voltageDomainRule(d))
	}
	if len(d.RailBudgets) > 0 {
		rules = append(rules, railBudgetCapacityRule(d))
		// The margin rule is emitted only when a factor is declared, the same shape anyModuleHasCount
		// gives the count rule and for a sharper reason: the factor IS the rule's threshold. With no
		// factor there is no question to ask, and compiling the rule anyway would let a bound item read
		// PASS against a policy number nobody stated. Not compiling it leaves the item
		// needs-design-intent, which names the missing input.
		if d.MarginFactor > 1 {
			rules = append(rules, railBudgetMarginRule(d))
		}
	}
	for _, s := range d.Subsystems {
		rules = append(rules, subsystemRule(s))
	}
	// One rule per declared sequence (WS3-092), the subsystem shape and the same WS3-058 reason.
	// A sequence with no adjacent good/enable pair compiles to NOTHING: its rule would have no link
	// to judge and could only ever pass. Parse rejects that at load with a message that teaches, so
	// this guard only catches a Declaration built in Go; the two share hasGatingPair so they cannot
	// disagree about what is checkable.
	for _, s := range d.Sequences {
		if hasGatingPair(s) {
			rules = append(rules, sequenceRule(s))
		}
	}
	// One rule per protection KIND present (kinds appear in declaration order so rule order is stable).
	seen := map[string]bool{}
	for _, p := range d.Protections {
		if seen[p.Kind] {
			continue
		}
		seen[p.Kind] = true
		rules = append(rules, protectionRule(p.Kind, d.Protections))
	}
	// One rule per property KIND present, same shape and same reason as protections above.
	seenProp := map[string]bool{}
	for _, np := range d.NetProperties {
		if seenProp[np.Property] {
			continue
		}
		seenProp[np.Property] = true
		rules = append(rules, propertyRule(np.Property, d.NetProperties))
	}
	return rules
}

// anyModuleHasCount reports whether the declaration sets an exact count on any module. The count rule
// is emitted only then, so a declaration with modules but no counts compiles to just module-missing (no
// count rule that would silently pass).
func anyModuleHasCount(d Declaration) bool {
	for _, mod := range d.Modules {
		if mod.Count > 0 {
			return true
		}
	}
	return false
}

// intentTags is the classification every intent rule carries. Category integrity marks a design-intent
// deviation as a "the design does not match what was declared" tripwire, and distribution open marks it
// as derivable from the authored declaration (no proprietary source in the mechanism itself).
func intentTags() map[string]string {
	return map[string]string{
		check.KeyCategory:     check.CategoryIntegrity,
		check.KeyDistribution: check.DistOpen,
	}
}
