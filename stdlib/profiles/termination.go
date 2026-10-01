package profiles

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// validateTermination rejects a termination requirement that does not name both bridged net suffixes.
// Without them the generated datalog matches on the empty suffix, which every net satisfies, so the
// rule would report the whole board as an unterminated bus rather than doing nothing visible.
func validateTermination(params map[string]string) error {
	var missing []string
	for _, k := range []string{"high", "low"} {
		if strings.TrimSpace(params[k]) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		noun := "param"
		if len(missing) > 1 {
			noun = "params"
		}
		return fmt.Errorf("needs the %s %s, naming the two bridged net-name suffixes (e.g. high: _CANH, low: _CANL); got %v",
			strings.Join(missing, " and "), noun, params)
	}
	return nil
}

// terminationRule (WS3-045) fires when an interface is in use but its two designated signal nets are
// not bridged by a termination element, the missing bus-termination case (CAN's 120Ω resistor across
// CANH/CANL, the RS-485 pattern). It is the first requirement type added purely as a registered
// compiler, with NO new engine primitive. The `net.reaches` walk already crosses 2-net series pass
// elements (R/L/ferrite/fuse) but NOT a multi-pin transceiver, so "the high net reaches the low net
// through a passive" finds the terminating resistor and ignores the transceiver that legitimately sits
// on both. A new requirement is new datalog, not new facts (the WS3-034 lesson). Because `net.reaches` is
// transitive it also accepts a split termination (60Ω + 60Ω with a midpoint).
//
// Params "high" and "low" name the two bridged net-name suffixes (e.g. "_CANH" / "_CANL"), required
// and validated by validateTermination before this compiler is reached.
func terminationRule(p Profile, req Requirement) *check.Rule {
	high, low := req.Params["high"], req.Params["low"]
	if high == "" || low == "" {
		// Unreachable for a validated profile (Load rejects this, Compile panics on it); kept so a
		// hand-built Requirement that bypasses both degrades to no rule instead of generating one that
		// matches the empty suffix, i.e. every net on the board.
		return nil
	}
	// The termination-specific rules as legible datalog. terminated(?h) holds for a high-suffix net
	// that reaches a low-suffix net through the series-passive walk, meaning a resistor (or split
	// pair) bridges the pair. reaches does NOT cross the multi-pin transceiver, so this finds the
	// terminator, not the IC that drives both lines. unterminated(?h) holds when a high net exists,
	// the bus is in use, and nothing terminates it. The per-signal presence rules (which define
	// in_use) are generated from the profile's signal list, so they are appended as AST rather than
	// inlined into this text.
	tq := query.MustParse(fmt.Sprintf(
		`terminated(?h) :- component.net(?r, ?h), suffix(?h, %q), net.reaches(?h, ?l), suffix(?l, %q);
		 any_term("x") :- terminated(?h);
		 unterminated(?h) :- component.net(?r, ?h), suffix(?h, %q), in_use(?iu), not any_term("x");
		 unterminated(?h) => ?h`, high, low, high))
	tq.Rules = append(p.presenceRules(), tq.Rules...)
	// The considered set: the high-side nets of a bus the presence gate says is in use, which is
	// `unterminated` without its negated clause. A high net here and absent from the findings is one
	// the series walk bridged to its low twin through a passive, so the pair is terminated.
	//
	// The domain is per HIGH NET, not per bus. Two CAN segments share one `any_term` fact, so a board
	// with two buses and one terminator reports both as terminated. That limit is the rule's own and
	// predates the considered set, which only makes it visible on the coverage line.
	dq := query.MustParse(fmt.Sprintf(
		`term_scope(?h) :- component.net(?r, ?h), suffix(?h, %q), in_use(?iu);
		 term_scope(?h) => ?h`, high))
	dq.Rules = append(append([]query.Rule{}, tq.Rules...), dq.Rules...)
	return query.MustRuleFromQuery(query.FindingQuery{
		Rule: check.Rule{
			Name:     p.lname() + "-termination-missing",
			Severity: "warning",
			Summary:  fmt.Sprintf("A %s bus has no termination across %s/%s.", p.Name, high, low),
			Impact:   "A differential bus with no termination resistor across its pair reflects signals off the unterminated end, corrupting data at speed; the link may pass at low rate and fail intermittently under load.",
			Remedy:   requirementRemedy("termination"),
			Tags:     p.tags(),
			Detail:   ruleDoc("termination"),
		},
		Query:      mustBindHeadFirst(tq),
		Kind:       check.KindNet,
		SubjectVar: "h",
		Message:    fmt.Sprintf("%s bus (net {h}) has no termination resistor bridging %s and %s", p.Name, high, low),
		Domain: &query.Domain{
			Query:   mustBindHeadFirst(dq),
			Witness: fmt.Sprintf("%s bus (net {h}) is bridged to its %s twin by a series passive", p.Name, low),
		},
	})
}
