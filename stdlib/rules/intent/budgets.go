package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Rail-current sizing (WS3-095): does the part supplying a rail carry the current the architecture
// says that rail draws, and does it carry it with the house margin on top.
//
// These rules join a DECLARED number against a DATASHEET number. The declaration supplies the demand,
// which no design artifact carries, and the regulator's seeded spec supplies the capacity, which no
// declaration should restate. Both halves have to be present for either rule to conclude anything.
//
// TWO rules over one mechanism: a review's "regulator output ratings" ask and its "current capability
// margins" ask are separate items that must report independently. A rail whose supply clears the peak
// but not the margin fires under margin ONLY, so capacity's silence there means "rated for the load",
// not "adequately sized".

// railBudgetCapacityRule: the supply reaching a declared rail is rated below the rail's peak budget.
func railBudgetCapacityRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleRailCurrentCapacity,
		Severity: "error",
		Summary:  "a rail's supply is rated below the peak current the design intent declares for it",
		Detail:   intentDoc(RuleRailCurrentCapacity),
		Impact: "the rail cannot deliver the current the architecture asks of it, so it sags or the regulator " +
			"folds back under the load the design was drawn for. It is a sizing error rather than a wiring " +
			"error, so nothing in the schematic looks wrong and it surfaces at bring-up under load.",
		Remedy:       intentRemedy(RuleRailCurrentCapacity),
		Reads:        []string{"param.output_current", "on_net"},
		ParamSymbols: check.OutputCurrentSymbols(),
		Tags:         intentTags(),
		Eval: func(m check.Model) []check.Verdict {
			return evalRailBudgets(m, d.RailBudgets, 1, func(b RailBudget, need float64, ref string, p *parampb.Parameter) string {
				return fmt.Sprintf("rail %q is declared to draw up to %gA peak, but %s supplies it rated at only %s %gA",
					b.Rail, b.Peak, ref, p.GetSymbol(), p.GetValue().GetMax())
			})
		},
		StatesConsideredSet: true,
	}
}

// railBudgetMarginRule: the supply clears the peak budget but not the declared margin over it. It
// stays silent below the peak, which is railBudgetCapacityRule's finding.
func railBudgetMarginRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleRailCurrentMargin,
		Severity: "warning",
		Summary:  "a rail's supply meets its declared peak budget but not the declared margin over it",
		Detail:   intentDoc(RuleRailCurrentMargin),
		Impact: "the rail works at the budgeted load and has no headroom for the things a budget does not " +
			"capture: an inrush transient, a load added late, a part running hotter than the budget assumed. " +
			"The design has no room left before the capacity finding becomes real.",
		Remedy:       intentRemedy(RuleRailCurrentMargin),
		Reads:        []string{"param.output_current", "on_net"},
		ParamSymbols: check.OutputCurrentSymbols(),
		Tags:         intentTags(),
		Eval: func(m check.Model) []check.Verdict {
			return evalRailBudgets(m, d.RailBudgets, d.MarginFactor, func(b RailBudget, need float64, ref string, p *parampb.Parameter) string {
				return fmt.Sprintf("rail %q is declared to draw up to %gA peak and the declared margin factor of %g asks for %gA, but %s supplies it rated at only %s %gA",
					b.Rail, b.Peak, d.MarginFactor, need, ref, p.GetSymbol(), p.GetValue().GetMax())
			})
		},
		StatesConsideredSet: true,
	}
}

// evalRailBudgets is the shared mechanism both sizing rules run: for each DECLARED rail budget, find
// the best-rated supply reaching that rail and compare it against factor x the peak. The margin rule
// passes the declared factor, the capacity rule passes 1.
//
// Three cases stay SILENT:
//
//   - A declared rail the design does not carry. That is a missing-rail defect, which the voltage-domain
//     and subsystem forms report; firing here as well would report one defect under two items.
//   - A rail no seeded supply reaches. The rule has nothing to compare, so a finding would be a false
//     fail. That case is the review runner's needs-data gate, which the rules feed by declaring
//     ParamSymbols. The gate is design-wide (nothing on the board states an output current), so a design
//     where SOME regulator is seeded and this rail's is not still reads pass; the doc cards say so.
//   - A supply that clears the threshold. Nothing to report.
func evalRailBudgets(m check.Model, budgets []RailBudget, factor float64, msg func(b RailBudget, need float64, ref string, p *parampb.Parameter) string) []check.Verdict {
	var out []check.Verdict
	for _, b := range budgets {
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(b.Rail)}}
		n := netNamed(m, b.Rail)
		if n == nil {
			// The missing rail is the voltage-domain and subsystem forms' defect, so this rule does not
			// report it as one. It did look, though, and a budget it could not size is a different
			// answer from a budget it sized and cleared.
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the design carries no net named %q, so there is no supply on it to size", b.Rail)
			out = append(out, v)
			continue
		}
		v.Subjects = []check.Entity{check.NetEntity(n)}
		ref, spec, p := bestSupply(m, n)
		if p == nil {
			v.Outcome = check.NotConsidered
			v.Reason = "no seeded part reaching this rail states an output-current rating, so there is nothing to compare the declared draw against"
			out = append(out, v)
			continue
		}
		rated := p.GetValue().GetMax()
		v.Context = []check.ContextSubject{check.Ctx(check.ComponentEntity(ref), "supply")}
		need := b.Peak * factor
		terms := []check.WitnessTerm{
			{Label: "rated", Value: fmt.Sprintf("%gA", rated)},
			{Label: "needed", Value: fmt.Sprintf("%gA", need)},
		}
		cite := []*check.DatasheetCitation{check.DatasheetCitationOf(spec, p)}
		// The margin rule declines the range the capacity rule owns. Without it a supply rated below the
		// peak would fire BOTH rules for one defect. Saying so is what makes the partition legible: the
		// two rules split one range, and a reader of the margin rule's rows should be able to see that
		// this rail is not unexamined but answered next door.
		if factor > 1 && below(rated, b.Peak) {
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the supply is rated below the %gA peak itself, which rail-current-capacity reports rather than this rule", b.Peak)
			out = append(out, v)
			continue
		}
		if !below(rated, need) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s supplies %q rated at %s %gA, at or above the %gA the declaration asks for",
					ref, b.Rail, p.GetSymbol(), rated, need),
				Terms:     terms,
				Datasheet: cite,
			}
			out = append(out, v)
			continue
		}
		v.Outcome = check.Fail
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("%s supplies %q rated at %s %gA, below the %gA the declaration asks for",
				ref, b.Rail, p.GetSymbol(), rated, need),
			Terms:     terms,
			Datasheet: cite,
		}
		v.Finding = &check.Finding{Subject: check.NetEntity(n), Message: msg(b, need, ref, p) + " — " + check.Citation(spec, p), Prov: n.GetProv(), DatasheetProv: cite}
		out = append(out, v)
	}
	return out
}

// bestSupply returns the HIGHEST output-current rating among the seeded parts reaching a rail, with
// the part's ref-des and spec. nil when no part reaching the rail states one.
//
// Highest, not lowest, and the choice is about false fails. A rail can be within reach of more than
// one seeded part (a second regulator one series element away, a multi-channel PMIC stating a rating
// per channel with no way to say which channel this net is), and picking the smallest of those would
// report a shortfall the design does not have. Where the evidence is ambiguous the rule takes the
// reading that does not fire; the cost is a missed finding on a rail genuinely fed by the smaller of
// two supplies.
//
// Reach is check.SupplyPathReachHops, the same radius the connection-aware voltage rules use, so a
// bead or a sense resistor between the regulator and the rail does not hide the supply. It needs no
// power-output pin typing, which is why these rules carry no format capability gate: an EDIF netlist
// (which types no power outputs, WS3-072) resolves the association exactly as a KiCad one does.
func bestSupply(m check.Model, rail *ir.Net) (string, *parampb.PartSpec, *parampb.Parameter) {
	var bestRef string
	var bestSpec *parampb.PartSpec
	var best *parampb.Parameter
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue
		}
		limits := check.OutputCurrentLimits(spec)
		if len(limits) == 0 || !reaches(m, c, rail) {
			continue
		}
		for _, p := range limits {
			if best == nil || p.GetValue().GetMax() > best.GetValue().GetMax() {
				bestRef, bestSpec, best = c.RefDes, spec, p
			}
		}
	}
	return bestRef, bestSpec, best
}

// reaches reports whether c sits on rail or within check.SupplyPathReachHops of it.
func reaches(m check.Model, c *ir.Component, rail *ir.Net) bool {
	for _, rn := range m.Reach(rail, check.SupplyPathReachHops).Nets {
		for _, conn := range rn.GetConnections() {
			if conn.GetComponentRef() == c.RefDes {
				return true
			}
		}
	}
	return false
}

// below reports whether have falls short of want, with a relative tolerance so binary floating-point
// error cannot manufacture a finding.
//
// The threshold is a PRODUCT (peak x factor) and the rating is a decimal an author read off a
// datasheet, and the two round differently. 0.1 x 1.5 is 0.15000000000000002 in float64 while the
// literal 0.15 is 0.1499999999999999944, so a 150mA part on a 100mA rail at a 1.5 factor fails a
// margin it meets exactly. Roughly one in six combinations of a milliamp-resolution budget and a
// common factor lands this way, so it is the ordinary case rather than a corner.
func below(have, want float64) bool {
	return have < want*(1-1e-9)
}
