package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/ident"
)

// How much of the netlist the IO map covers at all (agni issue 517).
//
// The interesting number here is not the failure count. A design with sixteen hundred nets and a map
// declaring two hundred of them has two hundred checked and fourteen hundred UNEXAMINED, which is not
// the same as clean, and a run reporting only the two hundred shows a green result whose denominator
// is invisible.
//
// This is the OTHER half of the pair io-map-net-absent owns, and they are opposite defects that get
// confused constantly: a net the map declares and the netlist lacks is usually a real disconnection,
// where a net the netlist has and the map does not declare is usually an incomplete map. The first is
// a fail next door. The second is this rule, and it is NOT a fail, because an undeclared net is not a
// defect in the design; it is a question nobody asked.

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

// ioMapCoverageVerdicts emits one verdict per net in the DESIGN, which is the one rule in this family
// whose considered set is the netlist rather than the declaration. That inversion is the point: every
// other io-map rule asks whether the design keeps a promise the map made, and this asks which parts of
// the design the map never spoke about.
//
// It builds a canonical index of the declared nets once and looks each design net up, rather than
// comparing every net against every row. Two hundred rows against sixteen hundred nets is 320,000
// comparisons however fast one of them is, and ident.Canonical is the shared key precisely so a
// caller at this scale can turn the search into a lookup (see the note on ident.Compare).
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
		// NOT a fail. An undeclared net is a question nobody asked, so the rule applied to the subject
		// and never reached a comparison, which is exactly what NotConsidered means.
		v.Outcome = check.NotConsidered
		v.Reason = "the IO map does not declare this net"
		// A rail or a ground is reported the same way and counted the same way, and only the REASON
		// differs. Carving them out of the denominator would be the tool deciding which absences are
		// acceptable, which is the judgment that lets a real gap hide: a map that forgot a whole
		// peripheral bank would read as well-covered if the arithmetic quietly excused a third of the
		// board. Saying which nets are rails gives a reader the discrimination without the tool making
		// the call for them.
		if m.IsGroundNet(n) || m.IsPowerRail(n.Name) {
			v.Reason += ", and it is a rail or ground, which a pin map does not usually name"
		}
		out = append(out, v)
	}
	return out
}

// coverageStatement says the net is declared, and says so differently when the map spells it
// differently, so a reader is never left wondering which row matched.
func coverageStatement(net, spelled string) string {
	if net == spelled {
		return fmt.Sprintf("the IO map declares net %q", net)
	}
	return fmt.Sprintf("the IO map declares net %q, spelled %q there", net, spelled)
}
