package intent

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
)

// subsystemRule builds the rule for ONE declared subsystem. It fails when the subsystem's required
// source component is absent (a KindComponent finding with no provenance, the presentResult shape) or
// any of its required nets is missing (a KindNet finding on the missing net). One rule per subsystem,
// named intent/subsystem-<slug>, unlike Modules which share one rule.
func subsystemRule(s Subsystem) *check.Rule {
	return &check.Rule{
		Name:                "subsystem-" + slug(s.Name),
		Severity:            "warning",
		Summary:             fmt.Sprintf("the %q subsystem the design intent declares is incomplete", s.Name),
		Detail:              intentDoc(docKeySubsystem),
		Impact:              "an architectural subsystem (clock, reset, power tree) the design was intended to contain is missing a required part or net",
		Remedy:              intentRemedy(docKeySubsystem),
		Reads:               []string{"component.class", "component.mpn"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return subsystemVerdicts(m, s) },
		StatesConsideredSet: true,
	}
}

// subsystemVerdicts decides every part the declaration NAMES: the source component when one is
// declared, and each required net. The considered set is the declaration itself, which is what makes
// an intent rule worth converting at all — the rule already knows exactly what it was asked to look
// for, so "five of the six things this subsystem declares are here, and the sixth is not" is a
// sentence it can produce with no extra machinery.
//
// Before, a subsystem that was entirely present reported nothing, which is what a subsystem whose
// declaration nobody wrote reports too.
func subsystemVerdicts(m check.Model, s Subsystem) []check.Verdict {
	var out []check.Verdict
	if s.Source != nil {
		v := check.Verdict{Subjects: []check.Entity{check.ComponentEntity(s.Name)}}
		if modulePresent(m, *s.Source) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the design carries a part satisfying the subsystem's source criterion (%s)", moduleCriterion(*s.Source)),
				Terms:     []check.WitnessTerm{{Label: "source criterion", Value: moduleCriterion(*s.Source)}},
			}
		} else {
			v.Outcome = check.Fail
			v.Finding = &check.Finding{
				Subject: check.ComponentEntity(s.Name),
				Message: fmt.Sprintf("subsystem %q is missing its source component (%s)", s.Name, moduleCriterion(*s.Source)),
			}
			v.Witness = &check.Witness{Statement: fmt.Sprintf("no part on the design satisfies %s", moduleCriterion(*s.Source))}
		}
		out = append(out, v)
	}
	for _, net := range s.Nets {
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(net)}}
		if n := netByName(m, net); n != nil {
			v.Subjects = []check.Entity{check.NetEntity(n)}
			v.Outcome = check.Pass
			v.Witness = &check.Witness{Statement: fmt.Sprintf("the design carries a net named %q, as subsystem %q requires", net, s.Name)}
		} else {
			v.Outcome = check.Fail
			v.Finding = &check.Finding{
				Subject: check.NetNameEntity(net),
				Message: fmt.Sprintf("subsystem %q requires net %q, which is not present on the design", s.Name, net),
			}
			v.Witness = &check.Witness{Statement: fmt.Sprintf("the design carries no net named %q", net)}
		}
		out = append(out, v)
	}
	return out
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
