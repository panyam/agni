package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// voltageDomainRule fails when a design deviates from a declared voltage domain: a declared rail net is
// absent (KindNet finding, no provenance), or present but its NAME declares a different nominal than
// the domain does (KindNet finding on the rail, with provenance). A present rail whose name carries no
// parseable voltage token (e.g. "VDD_CORE") is left alone: the name-derived nominal is the only voltage
// evidence a netlist carries, and refusing to guess is the contract (check.NominalVoltageFromName).
func voltageDomainRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleVoltageDomain,
		Severity: "warning",
		Summary:  "a declared voltage domain's rail is absent or on the wrong nominal voltage",
		Detail:   intentDoc(RuleVoltageDomain),
		Impact:   "a power rail is missing or named for a different voltage than its declared domain, so the power tree does not match the declared architecture",
		Remedy:   intentRemedy(RuleVoltageDomain),
		Reads:    []string{"net.nominal_voltage"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, dom := range d.VoltageDomains {
				for _, rail := range dom.Rails {
					n := netByName(m, rail)
					if n == nil {
						out = append(out, check.Finding{
							Kind:    check.KindNet,
							Subject: rail,
							Message: fmt.Sprintf("declared rail %q of voltage domain %q (%gV) is not present on the design", rail, dom.Name, dom.Nominal),
						})
						continue
					}
					if v, ok := check.NominalVoltageFromName(n.GetName()); ok && v != dom.Nominal {
						out = append(out, check.Finding{
							Kind:    check.KindNet,
							Subject: n.GetName(),
							NetID:   n.GetId(),
							Prov:    n.GetProv(),
							Message: fmt.Sprintf("rail %q is declared in voltage domain %q (%gV) but its name declares %gV", n.GetName(), dom.Name, dom.Nominal, v),
						})
					}
				}
			}
			return out
		},
	}
}

// netByName returns the first design net with the given name, or nil when none matches. Rails are
// unique by name in practice; the first match is the rail. It reads Model.Nets (the same slice the IR
// carries), so returning a raw *ir.Net stays inside the C19 read surface.
func netByName(m check.Model, name string) *ir.Net {
	for _, n := range m.Nets() {
		if n.GetName() == name {
			return n
		}
	}
	return nil
}
