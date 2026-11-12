package intent

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
)

// exposureDeclaredRule decides each component that has a declared exposure. The model applies the
// declaration itself (check.WithIntent), so this rule catches a declaration that cannot take effect.
// It names a ref-des the design does not carry, or a part that is not a connector.
func exposureDeclaredRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:                RuleExposureDeclared,
		Severity:            "warning",
		Summary:             docSummaries[RuleExposureDeclared],
		Detail:              intentDoc(RuleExposureDeclared),
		Impact:              "a connector meant to be declared internal is still checked as facing the field, or the declaration names a part it says nothing about",
		Remedy:              intentRemedy(RuleExposureDeclared),
		Reads:               []string{"component.class"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return exposureVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

func exposureVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.Exposures))
	for _, e := range d.Exposures {
		v := check.Verdict{Subjects: []check.Entity{check.ComponentEntity(e.Ref)}}
		var problem string
		switch {
		case !m.HasComponent(e.Ref):
			problem = fmt.Sprintf("%s is declared %s, and the design has no component %s", e.Ref, e.Exposure, e.Ref)
		case !m.HasClass(e.Ref, check.ClassConnector):
			problem = fmt.Sprintf("%s is declared %s, and it is a %s rather than a connector, so the declaration changes nothing", e.Ref, e.Exposure, m.ComponentClass(e.Ref))
		}
		if problem != "" {
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: problem}
			v.Finding = &check.Finding{Subject: check.ComponentEntity(e.Ref), Message: problem}
		} else {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s is a connector the design carries, declared %s", e.Ref, e.Exposure),
				Terms:     []check.WitnessTerm{{Label: "exposure", Value: e.Exposure}},
			}
		}
		out = append(out, v)
	}
	return out
}
