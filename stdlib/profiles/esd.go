package profiles

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// esdRule fires when one of the profile's signal nets leaves the board through a connector and
// nothing clamps it. It is the idiomatic form of a per-interface ESD ask. It is scoped to the
// profile's signals and gated by the profile's presence machinery, so a binding says `profile: CAN`
// instead of filtering a design-wide rule's findings down to an interface (the WS3-058 stopgap this
// supersedes).
//
// SCOPE COMES FROM THE DESIGN, NOT THE PROTOCOL. The requirement applies to whichever of the
// profile's nets net.connector_signal selects, rather than to a per-signal `esd:` flag the way pull-up
// works. On CAN, _TXD and _RXD run to the MCU and never leave the board, while _CANH and _CANL do,
// so a blanket per-signal application would fail the two lines that were never exposed. Whether a
// line is connector-facing is a property of the board in hand. A profile whose bus is entirely
// on-board therefore selects nothing and reports nothing, so declaring the requirement is safe even
// where it usually stays quiet.
//
// PARITY WITH THE CORE RULE IS BY CONSTRUCTION. The scope is the same projected predicate
// esd-protection uses, and the three protection clauses are the same three exemptions, at the same
// radius (check.ProtectionReachHops, interpolated rather than written as a literal so the two cannot
// drift).
func esdRule(p Profile, _ Requirement) *check.Rule {
	rules := p.presenceRules()
	signals := 0
	for _, s := range p.Signals {
		body := append([]query.Literal{query.Pos(query.Rel("net.connector_signal", query.V("n")))},
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

	// Three ways a net counts as protected, spelled as three rules sharing the esd_ok head. A discrete
	// TVS is the real answer. An IC on the net carrying a datasheet ESD rating is the common posture
	// (WS3-073). A Zener is NOT adequate ESD protection but is deliberately exempt here, because
	// esd-clamp-not-tvs (WS3-078) characterizes that case separately and the two rules partition these
	// nets between them. Crediting it keeps this requirement from double-reporting a net the catalog
	// already speaks about.
	//
	// Every clause carries needs_esd(?n), which binds the head variable for the walk. Without it the
	// walk has nothing to start from and runs from every net on the board, which is quadratic and made
	// `agni check` non-terminating on a real design (WS3-114). The guard restricts nothing, because
	// unprotected already conjoins needs_esd. It is written first for the reader; query.Default plans
	// the body, so the order no longer decides where the walk starts.
	for _, c := range []struct{ v, class string }{{"t", "tvs"}, {"z", "zener"}} {
		rules = append(rules, query.Def(query.Rel("esd_ok", query.V("n")),
			query.Pos(query.Rel("needs_esd", query.V("n"))),
			query.Pos(query.Rel("net.reaches", query.V("n"), query.V("rn"), query.V("h"))),
			query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
			query.Pos(query.Rel("component.net", query.V(c.v), query.V("rn"))),
			query.Pos(query.Rel("component.class", query.V(c.v), query.Str(c.class)))))
	}
	rules = append(rules, query.Def(query.Rel("esd_ok", query.V("n")),
		query.Pos(query.Rel("needs_esd", query.V("n"))),
		query.Pos(query.Rel("net.reaches", query.V("n"), query.V("rn"), query.V("h"))),
		query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
		query.Pos(query.Rel("component.net", query.V("u"), query.V("rn"))),
		query.Pos(query.Rel("component.esd_rated", query.V("u")))))

	rules = append(rules, query.Def(query.Rel("unprotected", query.V("n")),
		query.Pos(query.Rel("needs_esd", query.V("n"))),
		query.Pos(query.Rel("in_use", query.V("iu"))),
		query.Neg(query.Rel("esd_ok", query.V("n")))))

	// esd_scope is the considered set, every net this requirement APPLIED to, protected or not. It is
	// `unprotected` without the negated clause, so it holds the nets the profile selected as exposed on
	// a bus the presence gate says is in use. A net that is not here was never judged, and a net that
	// is here but absent from the findings is a net with a clamp in reach.
	//
	// Written out rather than derived from the goal above. It is derivable HERE, but signal-dangling
	// ends in a comparison instead of a negation, and a derivation that handles four of the six
	// requirement shapes would report the failures as the coverage on the other two.
	rules = append(rules, query.Def(query.Rel("esd_scope", query.V("n")),
		query.Pos(query.Rel("needs_esd", query.V("n"))),
		query.Pos(query.Rel("in_use", query.V("iu")))))

	// esd_by names the clamp for a passing net. esd_ok binds the clamping part but projects only the
	// net, so this head carries the part a reviewer wants named on a pass (agni issue 662).
	//
	// A SECOND head rather than a widening of esd_ok, because unprotected negates that relation and a
	// negated atom must stay unary. Giving esd_ok an arity of two would change what `not esd_ok(?n)`
	// means. The clauses below mirror the three above.
	for _, c := range []struct{ v, class string }{{"t", "tvs"}, {"z", "zener"}} {
		rules = append(rules, query.Def(query.Rel("esd_by", query.V("n"), query.V(c.v)),
			query.Pos(query.Rel("needs_esd", query.V("n"))),
			query.Pos(query.Rel("net.reaches", query.V("n"), query.V("rn"), query.V("h"))),
			query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
			query.Pos(query.Rel("component.net", query.V(c.v), query.V("rn"))),
			query.Pos(query.Rel("component.class", query.V(c.v), query.Str(c.class)))))
	}
	rules = append(rules, query.Def(query.Rel("esd_by", query.V("n"), query.V("u")),
		query.Pos(query.Rel("needs_esd", query.V("n"))),
		query.Pos(query.Rel("net.reaches", query.V("n"), query.V("rn"), query.V("h"))),
		query.Cmp(query.V("h"), "<=", query.Num(check.ProtectionReachHops)),
		query.Pos(query.Rel("component.net", query.V("u"), query.V("rn"))),
		query.Pos(query.Rel("component.esd_rated", query.V("u")))))

	q := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("unprotected", query.V("n")))}, query.V("n"))
	domain := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("esd_scope", query.V("n")))}, query.V("n"))
	evidence := query.Build(rules,
		[]query.Literal{query.Pos(query.Rel("esd_by", query.V("n"), query.V("clamp")))},
		query.V("n"), query.V("clamp"))
	return query.MustRuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-esd-missing",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s signal leaves the board through a connector with no ESD clamp.", p.Name),
			Impact:   "A signal that leaves the board through a connector is a direct ESD path into the transceiver behind it. Human contact discharges kilovolts, and an unclamped bus pin takes the hit. Failures are intermittent, latent, and show up in the field as a port that works until it does not.",
			Remedy:   requirementRemedy("esd"),
			Tags:     p.tags(),
			Detail:   ruleDoc("esd"),
		},
		Query:      mustBeInjective(q),
		Kind:       check.KindNet,
		SubjectVar: "n",
		Message:    fmt.Sprintf("%s signal net {n} is exposed on a connector with no ESD protection in reach", p.Name),
		// ContextVars applies to the finding path and the evidence path alike, so a pass names the
		// clamp as an entity a reader can click rather than only in prose. The finding goal binds no
		// clamp, and a context var that does not bind contributes nothing.
		ContextVars: []query.ContextVar{{Var: "clamp", Kind: check.KindComponent, Role: "clamp"}},
		Domain: &query.Domain{
			Query: mustBeInjective(domain),
			Witness: fmt.Sprintf("%s signal net {n} is exposed on a connector and reaches ESD protection within %d hops",
				p.Name, check.ProtectionReachHops),
			Evidence: &evidence,
		},
	})
}
