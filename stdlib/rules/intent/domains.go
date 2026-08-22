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
		Name:                RuleVoltageDomain,
		Severity:            "warning",
		Summary:             "a declared voltage domain's rail is absent or on the wrong nominal voltage",
		Detail:              intentDoc(RuleVoltageDomain),
		Impact:              "a power rail is missing or named for a different voltage than its declared domain, so the power tree does not match the declared architecture",
		Remedy:              intentRemedy(RuleVoltageDomain),
		Reads:               []string{"net.nominal_voltage"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return voltageDomainVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// voltageDomainVerdicts decides every rail every declared domain names, one verdict each.
//
// THE THIRD OUTCOME IS THE ONE THE OLD SHAPE COULD NOT REACH. A rail whose name states no voltage at
// all is neither a match nor a mismatch: the rule compares a NAME-derived nominal against the declared
// one, and a rail called VBUS or SYS_PWR supplies nothing to compare. It fell through the same silent
// path a correctly-named rail took, so a domain declared over rails nobody named for their voltage
// reported total agreement.
func voltageDomainVerdicts(m check.Model, d Declaration) []check.Verdict {
	var out []check.Verdict
	for _, dom := range d.VoltageDomains {
		for _, rail := range dom.Rails {
			n := netByName(m, rail)
			if n == nil {
				out = append(out, check.Verdict{
					Subjects: []check.Entity{check.NetNameEntity(rail)},
					Outcome:  check.Fail,
					Witness:  &check.Witness{Statement: fmt.Sprintf("the design carries no net named %q", rail)},
					Finding: &check.Finding{
						Subject: check.NetNameEntity(rail),
						Message: fmt.Sprintf("declared rail %q of voltage domain %q (%gV) is not present on the design", rail, dom.Name, dom.Nominal),
					},
				})
				continue
			}
			v := check.Verdict{Subjects: []check.Entity{check.NetEntity(n)}}
			declared := check.WitnessTerm{Label: "declared", Value: fmt.Sprintf("%gV", dom.Nominal)}
			nominal, ok := check.NominalVoltageFromName(n.GetName())
			switch {
			case !ok:
				v.Outcome = check.NotConsidered
				v.Reason = fmt.Sprintf("the rail's name states no voltage, so there is nothing to compare against domain %q's %gV",
					dom.Name, dom.Nominal)
			case nominal == dom.Nominal:
				v.Outcome = check.Pass
				v.Witness = &check.Witness{
					Statement: fmt.Sprintf("the rail's name declares %gV, matching voltage domain %q", nominal, dom.Name),
					Terms:     []check.WitnessTerm{{Label: "from the name", Value: fmt.Sprintf("%gV", nominal)}, declared},
				}
			default:
				v.Outcome = check.Fail
				v.Witness = &check.Witness{
					Statement: fmt.Sprintf("the rail's name declares %gV against domain %q's %gV", nominal, dom.Name, dom.Nominal),
					Terms:     []check.WitnessTerm{{Label: "from the name", Value: fmt.Sprintf("%gV", nominal)}, declared},
				}
				v.Finding = &check.Finding{
					Subject: check.NetEntity(n),
					Prov:    n.GetProv(),
					Message: fmt.Sprintf("rail %q is declared in voltage domain %q (%gV) but its name declares %gV", n.GetName(), dom.Name, dom.Nominal, nominal),
				}
			}
			out = append(out, v)
		}
	}
	return out
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
