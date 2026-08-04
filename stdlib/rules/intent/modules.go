package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
)

// moduleMissingRule fails once per declared module that no design component satisfies. A module matches
// when ANY component is of its declared class (Model.HasClass, so a family tag matches its specific
// classes) or carries its exact MPN (Model.ComponentMPN, which resolves only on a params-built model).
// The finding is a design-level absence: KindComponent with the module label as Subject and no
// provenance, because an absent module has no source site to cite (the presentResult shape). This is
// the honest guard made concrete — the rule iterates the DECLARATION and probes the design, so a
// missing module fails; it never enumerates modules FROM the netlist (which would always pass).
func moduleMissingRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleModuleMissing,
		Severity: "warning",
		Summary:  "a module the design intent declares required is not present",
		Detail:   intentDoc(RuleModuleMissing),
		Impact:   "a required functional block is missing from the schematic, so the design does not match its declared architecture",
		Reads:    []string{"component.class", "component.mpn"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, mod := range d.Modules {
				if modulePresent(m, mod) {
					continue
				}
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: mod.Name,
					Message: fmt.Sprintf("declared module %q (%s) is not present on the design", mod.Name, moduleCriterion(mod)),
				})
			}
			return out
		},
	}
}

// modulePresent reports whether any component satisfies the module's criterion. Class is checked first
// (available on any netlist); MPN second (resolves only under --params). A module with both set matches
// on EITHER, so an MPN that does not resolve still matches by class.
func modulePresent(m check.Model, mod Module) bool {
	for _, c := range m.Components() {
		if mod.Class != "" && m.HasClass(c.RefDes, check.ComponentClass(mod.Class)) {
			return true
		}
		if mod.MPN != "" && m.ComponentMPN(c.RefDes) == mod.MPN {
			return true
		}
	}
	return false
}

// moduleCountRule fails once per declared module whose EXACT expected Count differs from the actual
// number of design components matching its criterion. It is the complement of moduleMissingRule:
// missing asks "is at least one present", count asks "are there exactly N" (too few OR too many both
// fail). Only modules with Count > 0 are checked, so a declaration that sets no counts compiles to no
// count rule (empty-set-is-silent). Like module-missing, the expectation set comes from the
// DECLARATION, never enumerated from the netlist.
func moduleCountRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleModuleCount,
		Severity: "warning",
		Summary:  "the number of components for a declared module does not match the design intent",
		Detail:   intentDoc(RuleModuleCount),
		Impact:   "the design has too few or too many of a required functional block, so it does not match its declared architecture (a dropped or duplicated channel)",
		Reads:    []string{"component.class", "component.mpn"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, mod := range d.Modules {
				if mod.Count <= 0 {
					continue
				}
				got := moduleCount(m, mod)
				if got == mod.Count {
					continue
				}
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: mod.Name,
					Message: fmt.Sprintf("declared module %q (%s) expects %d, found %d", mod.Name, moduleCriterion(mod), mod.Count, got),
				})
			}
			return out
		},
	}
}

// moduleCount returns how many design components satisfy the module's criterion (class OR mpn). A
// component that matches on both class and mpn is counted once (the || short-circuits).
func moduleCount(m check.Model, mod Module) int {
	n := 0
	for _, c := range m.Components() {
		if (mod.Class != "" && m.HasClass(c.RefDes, check.ComponentClass(mod.Class))) ||
			(mod.MPN != "" && m.ComponentMPN(c.RefDes) == mod.MPN) {
			n++
		}
	}
	return n
}

// moduleCriterion renders a module's match criterion for the finding message ("class soc", "mpn
// MTFC4GACAJCN", or both).
func moduleCriterion(mod Module) string {
	switch {
	case mod.Class != "" && mod.MPN != "":
		return fmt.Sprintf("class %s or mpn %s", mod.Class, mod.MPN)
	case mod.Class != "":
		return "class " + mod.Class
	default:
		return "mpn " + mod.MPN
	}
}
