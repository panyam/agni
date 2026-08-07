package profiles

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// esdRule fires when one of the profile's signal nets leaves the board through a connector and
// nothing clamps it. It is the idiomatic form of a per-interface ESD ask: scoped to the profile's
// signals, gated by the profile's presence machinery, so a binding says `profile: CAN` instead of
// filtering a design-wide rule's findings down to an interface (the WS3-058 stopgap this supersedes).
//
// SCOPE COMES FROM THE DESIGN, NOT THE PROTOCOL. The requirement applies to whichever of the
// profile's nets external_signal_net selects, rather than to a per-signal `esd:` flag the way pull-up
// works. On CAN that matters: _TXD and _RXD run to the MCU and never leave the board, while _CANH and
// _CANL do, so a blanket per-signal application would fail the two lines that were never exposed.
// Whether a line is connector-facing is a property of the board in hand. A profile whose bus is
// entirely on-board therefore selects nothing and reports nothing, which is why declaring the
// requirement is safe even where it usually stays quiet.
//
// PARITY WITH THE CORE RULE IS BY CONSTRUCTION, not by a matching hand-written guard stack. The scope
// is the same projected predicate esd-protection uses, and the three protection clauses are the same
// three exemptions, at the same radius (check.ProtectionReachHops, interpolated rather than written
// as a literal so the two cannot drift).
func esdRule(p Profile, _ Requirement) *check.Rule {
	rules := p.presenceRules()
	signals := 0
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("external_signal_net", query.V("n")))},
			netMatch(query.V("n"), s)...)
		if len(body) == 1 {
			continue // a signal with no matcher would select every exposed net
		}
		signals++
		rules = append(rules, query.Def(query.Rel("needs_esd", query.V("n")), body...))
	}
	if signals == 0 {
		return nil
	}

	// Three ways a net counts as protected, which is what two rules sharing a head spells in datalog.
	// A discrete TVS is the real answer; an IC on the net carrying a datasheet ESD rating is the
	// common automotive posture (WS3-073); a Zener is NOT adequate ESD protection but is deliberately
	// exempt here, because esd-clamp-not-tvs (WS3-078) characterizes that case separately and the two
	// rules partition these nets between them. Crediting it here is what keeps this requirement from
	// double-reporting a net the catalog already speaks about.
	for _, c := range []struct{ v, class string }{{"t", "tvs"}, {"z", "zener"}} {
		rules = append(rules, query.Def(query.Rel("esd_ok", query.V("n")),
			query.Pos(query.Rel("reaches", query.V("n"), query.V("rn"), query.V("h"))),
			query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
			query.Pos(query.Rel("component-on-net", query.V(c.v), query.V("rn"))),
			query.Pos(query.Rel("component.class", query.V(c.v), query.Str(c.class)))))
	}
	rules = append(rules, query.Def(query.Rel("esd_ok", query.V("n")),
		query.Pos(query.Rel("reaches", query.V("n"), query.V("rn"), query.V("h"))),
		query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
		query.Pos(query.Rel("component-on-net", query.V("u"), query.V("rn"))),
		query.Pos(query.Rel("component.esd_rated", query.V("u")))))

	rules = append(rules, query.Def(query.Rel("unprotected", query.V("n")),
		query.Pos(query.Rel("needs_esd", query.V("n"))),
		query.Pos(query.Rel("in_use", query.V("iu"))),
		query.Neg(query.Rel("esd_ok", query.V("n")))))

	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("unprotected", query.V("n")))}, query.V("n"))
	return query.RuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-esd-missing",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s signal leaves the board through a connector with no ESD clamp.", p.Name),
			Impact:   "A signal that leaves the board through a connector is a direct ESD path into the transceiver behind it. Human contact discharges kilovolts, and an unclamped bus pin takes the hit. Failures are intermittent, latent, and show up in the field as a port that works until it does not.",
			Tags:     p.tags(),
			Detail:   ruleDoc("esd"),
		},
		Query:      q,
		Kind:       check.KindNet,
		SubjectVar: "n",
		Message:    fmt.Sprintf("%s signal net {n} is exposed on a connector with no ESD protection in reach", p.Name),
	})
}
