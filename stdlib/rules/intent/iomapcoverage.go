package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/ident"
)

// How much of the netlist the IO map covers at all (agni issue 517). A design with sixteen hundred
// nets and a map declaring two hundred has fourteen hundred UNEXAMINED, which is not clean, and a run
// reporting only the two hundred hides that denominator.
//
// This is the OTHER half of io-map-net-absent, and the two are opposite defects. A net the map
// declares and the netlist lacks is usually a real disconnection, and fails over there. A net the
// netlist has and the map does not declare is usually an incomplete map, and lands here as NOT a
// fail, since it is a question nobody asked rather than a design defect.

// ioMapCoverageRule reports, per net, whether the IO map declares it.
func ioMapCoverageRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleIOMapCoverage,
		Severity: "info",
		Summary:  "how much of the netlist the design intent's IO map declares",
		Detail:   intentDoc(RuleIOMapCoverage),
		Impact: "a map covering a fraction of the netlist gives a green IO-map result about that fraction " +
			"alone. The nets it does not name are unexamined rather than correct, and nothing else in the " +
			"run distinguishes the two.",
		Remedy:              intentRemedy(RuleIOMapCoverage),
		Reads:               []string{"on_net"},
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return ioMapCoverageVerdicts(m, d) },
		StatesConsideredSet: true,
	}
}

// ioMapCoverageVerdicts emits one verdict per net in the DESIGN. It is the one rule in this family
// whose considered set is the netlist rather than the declaration, so it reports which parts of the
// design the map never mentions.
//
// It indexes the declared nets by ident.Canonical once and looks each design net up, since two
// hundred rows against sixteen hundred nets is 320,000 comparisons (see the note on ident.Compare).
func ioMapCoverageVerdicts(m check.Model, d Declaration) []check.Verdict {
	declared := make(map[string]string, len(d.IOMap)) // canonical form -> the map's own spelling
	for _, a := range d.IOMap {
		declared[ident.Canonical(a.Net)] = a.Net
	}
	nets := m.Nets()
	out := make([]check.Verdict, 0, len(nets))
	for _, n := range nets {
		v := check.Verdict{Subjects: []check.Entity{check.NetEntity(n)}}
		if spelled, ok := declared[ident.Canonical(n.Name)]; ok {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{Statement: coverageStatement(n.Name, spelled)}
			out = append(out, v)
			continue
		}
		// NOT a fail. The rule applied to the subject and never reached a comparison, so NotConsidered.
		v.Outcome = check.NotConsidered
		v.Reason = "the IO map does not declare this net"
		// A rail or ground is counted the same way and only the REASON differs. Excusing rails from the
		// denominator would let a map that forgot a whole peripheral bank read as well-covered.
		if m.IsGroundNet(n) || m.IsPowerRail(n.Name) {
			v.Reason += ", and it is a rail or ground, which a pin map does not usually name"
		}
		out = append(out, v)
	}
	return out
}

// coverageStatement says the net is declared, and names the map's spelling when it differs so a
// reader can see which row matched.
func coverageStatement(net, spelled string) string {
	if net == spelled {
		return fmt.Sprintf("the IO map declares net %q", net)
	}
	return fmt.Sprintf("the IO map declares net %q, spelled %q there", net, spelled)
}
