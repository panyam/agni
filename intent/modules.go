package intent

import (
	"fmt"

	"github.com/panyam/agni/check"
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
