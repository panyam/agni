package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Load-switch sizing, the lower bound (WS3-085).
//
// A load switch's current limit has to sit in a WINDOW. Above the FET's rating, the FET fails before
// the protection acts (builtin's load-switch-trip-above-fet-rating). Below what the rail draws, the
// switch opens on normal operation and the rail never comes up under load.
//
// The upper bound is decidable from two datasheets. THE LOWER BOUND IS NOT, which is why this half is an
// intent rule and its twin is a builtin. Nothing in a design states what a rail draws, and summing rated
// draws would need near-complete part seeding plus a guess at which loads draw at once. So the demand is
// DECLARED, in the same rail_budgets the regulator-sizing rules read.

// loadSwitchTripBelowBudgetRule reports a controller-based load switch whose limit is below the
// declared peak draw of the rail it feeds.
func loadSwitchTripBelowBudgetRule(d Declaration) *check.Rule {
	return &check.Rule{
		Name:     RuleLoadSwitchTripBelowBudget,
		Severity: "error",
		Summary:  "a load switch limits current below the peak the design intent declares for the rail it feeds",
		Detail:   intentDoc(RuleLoadSwitchTripBelowBudget),
		Impact: "the switch opens under the load the architecture was drawn for, so the rail collapses or " +
			"cycles on ordinary operation rather than on a fault. Nothing in the schematic looks wrong: the " +
			"trip point is the arithmetic of a threshold the controller states and a shunt the designer chose, " +
			"and it surfaces at bring-up as an intermittent rail nobody can pin on a part.",
		Remedy: intentRemedy(RuleLoadSwitchTripBelowBudget),
		Reads: []string{
			"param.ocp_threshold", "param.on_resistance",
			"component.value", "component.class", "pin.role", "on_net",
		},
		ParamSymbols:        check.OcpThresholdSymbols(),
		Tags:                intentTags(),
		Eval:                func(m check.Model) []check.Verdict { return evalLoadSwitchTrip(m, d.RailBudgets) },
		StatesConsideredSet: true,
	}
}

// evalLoadSwitchTrip reports each DECLARED rail budget whose load switch limits current below the
// declared peak. Per budget (#417):
//
//   - A declared rail the design does not carry is NotConsidered, the same shape as budgets.go.
//   - A rail no controller-based load switch reaches is NOT A SUBJECT and yields nothing. The design
//     may have no switch there, an INTEGRATED switch (one part, no external FET for the resolver to
//     find), or one the resolver refused as ambiguous. None is a sizing defect.
//   - A controller stating no overcurrent threshold, or a shunt with no value in ohms, keeps the switch
//     out of ExternalFetLoadSwitches, so it lands in the case above. The review runner's needs-data
//     gate covers it through ParamSymbols.
//   - A trip point at or above the budget is a PASS carrying both numbers and the citation.
//
// A pass says only that the limit is above the declared draw. Whether it is below what the FET survives
// is the builtin rule's question, and whether the FET runs cool is not judged at all (see sizingClause).
func evalLoadSwitchTrip(m check.Model, budgets []RailBudget) []check.Verdict {
	switches := check.ExternalFetLoadSwitches(m)
	var out []check.Verdict
	for _, b := range budgets {
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(b.Rail)}}
		rail := netNamed(m, b.Rail)
		if rail == nil {
			// The voltage-domain and subsystem forms report the missing rail. This only says it could
			// not judge.
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the design carries no net named %q, so there is no switch on it to size", b.Rail)
			out = append(out, v)
			continue
		}
		v.Subjects = []check.Entity{check.NetEntity(rail)}
		sw := highestTripOnRail(m, switches, rail)
		if sw == nil {
			// NOT a subject. A switch resolves only when controller, FET and shunt are each
			// unambiguous, so this also covers one the walk could not read, which
			// check.ExternalFetLoadSwitches reports by omission.
			continue
		}
		ctrlSpec := m.PartSpec(sw.Controller)
		v.Context = []check.ContextSubject{
			check.Ctx(check.ComponentEntity(sw.Controller), "controller"),
			check.Ctx(check.ComponentEntity(sw.Sense), "sense"),
		}
		if !below(sw.TripAmps, b.Peak) {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("the load switch feeding %q limits at %gA, at or above the %gA peak the intent declares",
					b.Rail, sw.TripAmps, b.Peak),
				Terms: []check.WitnessTerm{
					{Label: "trip", Value: fmt.Sprintf("%gA", sw.TripAmps)},
					{Label: "declared peak", Value: fmt.Sprintf("%gA", b.Peak)},
				},
				Datasheet: []*check.DatasheetCitation{check.DatasheetCitationOf(ctrlSpec, sw.Ocp)},
			}
			out = append(out, v)
			continue
		}
		msg := fmt.Sprintf(
			"rail %q is declared to draw up to %gA peak, but the load switch feeding it limits at %gA (%s %gV across sense resistor %s at %gΩ), so %s opens under the declared load rather than under a fault",
			b.Rail, b.Peak, sw.TripAmps,
			sw.Ocp.GetSymbol(), sw.Ocp.GetValue().GetMax(), sw.Sense, sw.SenseOhms, sw.Controller)
		msg += " — " + check.Citation(ctrlSpec, sw.Ocp) + sizingClause(m, sw, b.Peak)
		f := check.Finding{Subject: check.NetEntity(rail), Message: msg, Prov: rail.GetProv(), // Only the controller's threshold is cited, since the trip is that threshold over a
			// resistance the DESIGN states. The FET's on-resistance is in the message but not cited,
			// because a finding is rated by its WEAKEST citation and an unused value could drag a real
			// failure down to provisional.
			DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(ctrlSpec, sw.Ocp)}}
		v.Outcome = check.Fail
		v.Witness = &check.Witness{
			Statement: fmt.Sprintf("the load switch feeding %q limits at %gA, below the %gA peak the intent declares",
				b.Rail, sw.TripAmps, b.Peak),
			Terms: []check.WitnessTerm{
				{Label: "trip", Value: fmt.Sprintf("%gA", sw.TripAmps)},
				{Label: "declared peak", Value: fmt.Sprintf("%gA", b.Peak)},
			},
			Datasheet: []*check.DatasheetCitation{check.DatasheetCitationOf(ctrlSpec, sw.Ocp)},
		}
		v.Finding = &f
		out = append(out, v)
	}
	return out
}

// sizingClause is the RDS(on) half of load-switch sizing, the pass element's dissipation at the
// declared rail current. A reviewer needs it next, because lowering the shunt fixes a low trip point
// only if the FET can carry the budgeted current.
//
// It is REPORTED, never judged, so it is a clause and not a second rule. Judging needs a thermal limit
// (package thermal resistance, ambient, acceptable junction rise), and neither the parameter layer nor
// any declaration field carries one.
//
// Empty when OnResistance is nil (see check.ExternalFetLoadSwitch.OnResistance), with no fallback text.
func sizingClause(m check.Model, sw *check.ExternalFetLoadSwitch, peak float64) string {
	if sw.OnResistance == nil {
		return ""
	}
	ohms := sw.OnResistance.GetValue().GetMax()
	watts, ok := check.ResistivePowerWatts(peak, ohms)
	if !ok {
		return ""
	}
	return fmt.Sprintf(" Sizing the pass element: %s carries the declared %gA through its %s of %gΩ, dissipating %gW (%s).",
		sw.Fet, peak, sw.OnResistance.GetSymbol(), ohms, watts, check.Citation(m.PartSpec(sw.Fet), sw.OnResistance))
}

// highestTripOnRail returns the resolved load switch on a rail whose limit is HIGHEST, or nil when no
// resolved switch carries it.
//
// HIGHEST, not lowest, the same false-fail trade bestSupply makes on the supply side. A rail can be
// within reach of more than one switch, and the smallest limit may belong to a switch gating a different
// branch. The cost is a missed finding on a rail genuinely gated by the smaller of two switches.
func highestTripOnRail(m check.Model, switches []check.ExternalFetLoadSwitch, rail *ir.Net) *check.ExternalFetLoadSwitch {
	var best *check.ExternalFetLoadSwitch
	for i := range switches {
		sw := &switches[i]
		if !switchCarriesRail(m, sw, rail) {
			continue
		}
		if best == nil || sw.TripAmps > best.TripAmps {
			best = sw
		}
	}
	return best
}

// switchCarriesRail reports whether a non-GATE terminal of the pass element sits on the rail or within
// check.SupplyPathReachHops (one series element) of it. That is the radius the supply-side rule uses,
// so the two sizing rules agree on what "on this rail" means.
//
// Both sides of the switch count, since a series element carries the same current in and out, so the
// budget may be declared on either side of the FET.
//
// The GATE is excluded because a gate net touches the pass element but carries none of its current.
// The role comes from the naming lexicon, never from a pin name matched here (C20).
func switchCarriesRail(m check.Model, sw *check.ExternalFetLoadSwitch, rail *ir.Net) bool {
	for _, rn := range m.Reach(rail, check.SupplyPathReachHops).Nets {
		for _, c := range rn.GetConnections() {
			if c.GetComponentRef() == sw.Fet && m.PinRole(sw.Fet, c.GetPinRef()) != check.RoleGate {
				return true
			}
		}
	}
	return false
}
