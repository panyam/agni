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
	// RuleRailCurrentCapacity and RuleRailCurrentMargin are the two rail-sizing rules (WS3-095): one
	// mechanism at two thresholds, split into two names so the ratings item and the margins item get
	// separate verdicts.
	RuleRailCurrentCapacity = "rail-current-capacity"
	RuleRailCurrentMargin   = "rail-current-margin"
	// RuleLoadSwitchTripBelowBudget is the LOWER bound of load-switch sizing (WS3-085): the switch's
	// current limit against the rail's declared draw. It reads the same rail_budgets the two rules above
	// read, and it is a third rule rather than a threshold on them because it judges a different part,
	// the switch rather than the supply.
	RuleLoadSwitchTripBelowBudget = "load-switch-trip-below-budget"
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
		// The load-switch lower bound needs the budget and nothing else, so it rides the capacity
		// condition rather than a second one. A design with no controller-based switch resolves none
		// and the rule reports nothing. That silence is unavoidable: no declaration field says "this
		// rail is switched", and inventing one would let an author's omission read as a defect.
		rules = append(rules, loadSwitchTripBelowBudgetRule(d))
		// Only when a factor is declared, because the factor IS the rule's threshold. See
		// Declaration.MarginFactor for why there is no default.
		if d.MarginFactor > 1 {
			rules = append(rules, railBudgetMarginRule(d))
		}
	}
	for _, s := range d.Subsystems {
		rules = append(rules, subsystemRule(s))
	}
	// One rule per declared strap group (WS3-120). The collision check is cross-group, so it gets ONE
	// rule for the whole declaration, compiled only when at least two groups share a bus: over fewer
	// it could only ever pass.
	for _, g := range d.StrapGroups {
		rules = append(rules, strapGroupRule(g))
	}
	if collidableGroups(d.StrapGroups) {
		rules = append(rules, strapCollisionRule(d.StrapGroups))
	}
	// One rule per declared sequence (WS3-092). A sequence with no adjacent good/enable pair compiles
	// to NOTHING: its rule would have no link to judge and could only ever pass. Parse already rejects
	// that at load, so this guard only catches a Declaration built in Go; the two share hasGatingPair
	// so they cannot disagree about what is checkable.
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
// is emitted only then, so modules with no counts compile to just module-missing.
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
