package intent

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
)

// subsystemRule builds the rule for ONE declared subsystem. It fails when the subsystem's required
// source component is absent (a KindComponent finding, no provenance — the presentResult shape) or any
// of its required nets is missing (a KindNet finding on the missing net). One rule per subsystem —
// named intent/subsystem-<slug> — so distinct review items (clock vs reset architecture) bind their
// own subsystem and report independently, rather than sharing one rule the way Modules do. Like the
// other intent rules it iterates the DECLARATION and probes the design; it never derives the expected
// subsystem set from the netlist.
func subsystemRule(s Subsystem) *check.Rule {
	return &check.Rule{
		Name:     "subsystem-" + slug(s.Name),
		Severity: "warning",
		Summary:  fmt.Sprintf("the %q subsystem the design intent declares is incomplete", s.Name),
		Detail:   intentDoc(docKeySubsystem),
		Impact:   "an architectural subsystem (clock, reset, power tree) the design was intended to contain is missing a required part or net",
		Reads:    []string{"component.class", "component.mpn"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			if s.Source != nil && !modulePresent(m, *s.Source) {
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: s.Name,
					Message: fmt.Sprintf("subsystem %q is missing its source component (%s)", s.Name, moduleCriterion(*s.Source)),
				})
			}
			for _, net := range s.Nets {
				if netByName(m, net) == nil {
					out = append(out, check.Finding{
						Kind:    check.KindNet,
						Subject: net,
						Message: fmt.Sprintf("subsystem %q requires net %q, which is not present on the design", s.Name, net),
					})
				}
			}
			return out
		},
	}
}

// slug turns a subsystem name into the kebab-case suffix of its rule name: lower-cased, runs of
// non-alphanumeric characters collapsed to a single "-", and leading/trailing "-" trimmed. So "main
// clock" -> "main-clock" and the rule is intent/subsystem-main-clock. Load validates that names
// slugify uniquely, so two subsystems never collide on one rule name.
func slug(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
