package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Load-switch sizing, the lower bound (WS3-085).
//
// A load switch's current limit has to sit in a WINDOW. Above it is the pass element's own rating: a
// limit set higher than the FET can survive means the FET fails before the protection acts, which is
// what builtin's load-switch-trip-above-fet-rating reports. Below it is the load: a limit set under the
// current the rail actually draws means the switch opens on normal operation, so the rail never comes
// up under load.
//
// The upper bound is decidable from two datasheets. THE LOWER BOUND IS NOT, and that is why this half
// is an intent rule and its twin is a builtin. Nothing in a design states what a rail draws, and summing
// every load's rated draw would need near-complete part seeding plus an assumption about which loads
// draw at once. The demand has to be DECLARED, so it lives in the same rail_budgets the
// regulator-sizing rules read.

// loadSwitchTripBelowBudgetRule: a controller-based load switch limits current below the peak the
// declaration says the rail it feeds draws.
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
		Reads: []string{
			"param.ocp_threshold", "param.on_resistance",
			"component.value", "component.class", "pin.role", "on_net",
		},
		ParamSymbols: check.OcpThresholdSymbols(),
		Tags:         intentTags(),
		Eval: func(m check.Model) []check.Finding {
			return evalLoadSwitchTrip(m, d.RailBudgets)
		},
	}
}

// evalLoadSwitchTrip reports each DECLARED rail budget whose load switch limits current below the
// declared peak.
//
// Four cases stay SILENT:
//
//   - A declared rail the design does not carry. That is a missing-rail defect the voltage-domain and
//     subsystem forms report, and firing here as well would put one defect under two review items.
//   - A rail no controller-based load switch reaches. The design may have no switch on that rail at
//     all, or an INTEGRATED switch (one part, no external FET, so nothing for the resolver to find), or
//     a switch the resolver refused as ambiguous. None of those is a sizing defect.
//   - A switch whose controller states no overcurrent threshold, or whose shunt the design does not
//     state in ohms. There is no trip current to compare, so a verdict would be invented. That case is
//     the review runner's needs-data gate, which this rule feeds by declaring ParamSymbols.
//   - A trip point at or above the budget. Nothing to report.
//
// A pass says only that the limit is above the declared draw. It does not say the limit is below what
// the FET survives (that is the builtin rule's question) and it does not say the FET runs cool at the
// declared current (nothing states a thermal limit to judge against, see sizingClause).
func evalLoadSwitchTrip(m check.Model, budgets []RailBudget) []check.Finding {
	switches := check.ExternalFetLoadSwitches(m)
	var out []check.Finding
	for _, b := range budgets {
		rail := netNamed(m, b.Rail)
		if rail == nil {
			// The guard is redundant today (Reach walks nothing from a nil start, so an absent rail falls
			// out at the sw == nil skip below) and stays anyway: a missing rail is the voltage-domain and
			// subsystem forms' defect to report, and if Reach ever answered a nil start differently the
			// fallthrough would build a finding with a nil Prov that the viewer cannot locate.
			continue
		}
		sw := highestTripOnRail(m, switches, rail)
		if sw == nil {
			continue
		}
		if !below(sw.TripAmps, b.Peak) {
			continue
		}
		ctrlSpec := m.PartSpec(sw.Controller)
		msg := fmt.Sprintf(
			"rail %q is declared to draw up to %gA peak, but the load switch feeding it limits at %gA (%s %gV across sense resistor %s at %gΩ), so %s opens under the declared load rather than under a fault",
			b.Rail, b.Peak, sw.TripAmps,
			sw.Ocp.GetSymbol(), sw.Ocp.GetValue().GetMax(), sw.Sense, sw.SenseOhms, sw.Controller)
		msg += " — " + check.Citation(ctrlSpec, sw.Ocp) + sizingClause(m, sw, b.Peak)
		out = append(out, check.Finding{
			Kind:    check.KindNet,
			Subject: b.Rail,
			Message: msg,
			Prov:    rail.GetProv(),
			// The controller's threshold is the only datasheet value the VERDICT rests on: the trip
			// current is that threshold divided by a resistance the DESIGN states. The pass FET's
			// on-resistance is reported in the message but not cited, because a finding is rated by its
			// WEAKEST citation and a value the conclusion never used could drag a genuine failure down
			// to provisional.
			DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(ctrlSpec, sw.Ocp)},
		})
	}
	return out
}

// sizingClause is the RDS(on) half of item-26-style sizing: what the pass element dissipates at the
// current the declaration says the rail draws. It is the number a reviewer needs next, because the fix
// for a trip point set too low is to lower the shunt, and that only helps if the FET can carry the
// budgeted current in the first place.
//
// It is REPORTED, never judged, which is why it is a clause and not a second rule. Judging it needs a
// thermal limit: a package thermal resistance, an ambient, a junction rise the house is willing to
// accept. No datasheet row the parameter layer reads states one and no declaration field carries one,
// so a rule that failed on dissipation would be failing against a threshold nobody declared.
//
// Empty when the FET is unseeded or states no comparable RDS(on) row. A zero or an omitted figure would
// read as a FET that dissipates nothing, so there is no fallback text.
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
// within reach of more than one switch, and picking the smallest limit would report a nuisance trip on
// a switch that gates a different branch. The cost is a missed finding on a rail genuinely gated by the
// smaller of two switches.
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

// switchCarriesRail reports whether the switch's current flows through the rail: a POWER terminal of
// the pass element sits on it or within one series element of it (check.SupplyPathReachHops, the radius
// the supply-side rule associates at, so the two sizing rules cannot drift to different answers about
// what "on this rail" means).
//
// Both sides of the switch count: a series element carries the same current on its input and its
// output, so a limit that opens under the declared draw does so whichever side of the FET an author
// declared the budget on.
//
// The GATE is excluded, which is why this is a pin-role test rather than an "is the FET connected here"
// test. A gate net touches the pass element but carries none of its current, so crediting it would fire
// on the gate of any switch a board happens to have. The role comes from the naming lexicon, never from
// a pin name matched here (C20).
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
