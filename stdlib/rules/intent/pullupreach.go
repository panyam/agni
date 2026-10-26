package intent

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
)

// pullUpReachDeclaredRule decides every net the declaration gives a pull-up reach. The reach itself
// takes effect in the model (check.WithIntent), keyed by the exact net name, so this rule's job is to
// catch a declaration that cannot have: a net name the design does not carry (agni issue 1165).
func pullUpReachDeclaredRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:                RulePullUpReachDeclared,
		Severity:            "warning",
		Summary:             docSummaries[RulePullUpReachDeclared],
		Detail:              intentDoc(RulePullUpReachDeclared),
		Impact:              "a bus whose series network was declared longer is still held to the default reach, so its pull-up reads as missing, or the declaration names a net it says nothing about",
		Remedy:              intentRemedy(RulePullUpReachDeclared),
		Reads:               []string{"net.names"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return pullUpReachVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

func pullUpReachVerdicts(m check.Model, d Declaration) []check.Verdict {
	out := make([]check.Verdict, 0, len(d.PullUpReach))
	for _, r := range d.PullUpReach {
		subject := check.Entity{Kind: check.KindNet, Ref: r.Net}
		n := netByName(m, r.Net)
		if n != nil {
			subject.NetID = n.GetId()
		}
		v := check.Verdict{Subjects: []check.Entity{subject}}
		if n == nil {
			problem := fmt.Sprintf("%s declares a pull-up reach of %d, and the design has no net %s", r.Net, r.Hops, r.Net)
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: problem}
			v.Finding = &check.Finding{Subject: subject, Message: problem}
		} else {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s is a net the design carries, declared a pull-up reach of %d", r.Net, r.Hops),
				Terms:     []check.WitnessTerm{{Label: "hop limit", Value: fmt.Sprintf("%d", r.Hops)}},
			}
		}
		out = append(out, v)
	}
	return out
}
