package profiles

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// terminationRule (WS3-045) fires when an interface is in use but its two designated signal nets are
// not bridged by a termination element — the missing bus-termination case (CAN's 120Ω resistor
// across CANH/CANL, the RS-485 pattern). It is the first requirement type added purely as a
// registered compiler, with NO new engine primitive: the `reaches` walk already crosses 2-net series
// pass elements (R/L/ferrite/fuse) but NOT a multi-pin transceiver, so "the high net reaches the low
// net through a passive" finds the terminating resistor and ignores the transceiver that
// legitimately sits on both — a new requirement is new datalog, not new facts (the WS3-034 lesson).
// Because `reaches` is transitive it also accepts a split termination (60Ω + 60Ω with a midpoint).
//
// Params: "high" and "low" name the two bridged net-name suffixes (e.g. "_CANH" / "_CANL"). A
// requirement missing either param is a profile-authoring error, caught at Compile time.
func terminationRule(p Profile, req Requirement) *check.Rule {
	high, low := req.Params["high"], req.Params["low"]
	if high == "" || low == "" {
		panic(fmt.Sprintf("profiles: termination requirement on %q needs \"high\" and \"low\" params, got %v", p.Name, req.Params))
	}
	// The termination-specific rules as legible datalog. terminated(?h): a high-suffix net that
	// reaches a low-suffix net through the series-passive walk — a resistor (or split pair) bridges
	// the pair. reaches does NOT cross the multi-pin transceiver, so this finds the terminator, not
	// the IC that drives both lines. unterminated(?h): a high net exists, the bus is in use, and
	// nothing terminates it. The per-signal presence rules (which define in_use) are generated from
	// the profile's signal list, so they are appended as AST rather than inlined into this text.
	tq := query.MustParse(fmt.Sprintf(
		`terminated(?h) :- component-on-net(?r, ?h), suffix(?h, %q), reaches(?h, ?l), suffix(?l, %q);
		 any_term("x") :- terminated(?h);
		 unterminated(?h) :- component-on-net(?r, ?h), suffix(?h, %q), in_use(?iu), not any_term("x");
		 unterminated(?h) => ?h`, high, low, high))
	tq.Rules = append(p.presenceRules(), tq.Rules...)
	return query.RuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-termination-missing",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s bus has no termination across %s/%s.", p.Name, high, low),
			Impact:   "A differential bus with no termination resistor across its pair reflects signals off the unterminated end, corrupting data at speed; the link may pass at low rate and fail intermittently under load.",
			Tags:     p.tags(),
			Detail:   ruleDoc("termination"),
		},
		Query:      tq,
		Kind:       check.KindNet,
		SubjectVar: "h",
		Message:    fmt.Sprintf("%s bus (net {h}) has no termination resistor bridging %s and %s", p.Name, high, low),
	})
}
