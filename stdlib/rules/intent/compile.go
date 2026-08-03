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
	for _, s := range d.Subsystems {
		rules = append(rules, subsystemRule(s))
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
