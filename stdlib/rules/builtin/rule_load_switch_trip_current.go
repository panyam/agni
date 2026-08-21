package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// loadSwitchTripAboveFetRating flags a controller-based load switch whose current limit is set above
// the continuous drain rating of the external MOSFET it switches through (WS3-085). Silence here is
// "I could not tell", never "this is fine": the switch is only resolved when the controller, the FET
// and the shunt are each unambiguous (see check.ExternalFetLoadSwitches). Every gap that produces
// silence, and what the rule does not claim about derating, are in
// docs/load-switch-trip-above-fet-rating.md.
var loadSwitchTripAboveFetRating = &check.Rule{
	Name:       "load-switch-trip-above-fet-rating",
	Severity:   "error",
	Summary:    "A controller-based load switch trips above the continuous drain rating of its external MOSFET.",
	Impact:     "The current limit never protects the pass element: the external FET reaches its own rating while the controller is still below its trip point, so the part the switch exists to protect is the one that fails, and a shorted high-side switch applies the full rail to the load. Both numbers are vendor values, cited with the page they came from.",
	Remedy:     "Lower the switch's current-limit setting below the FET's continuous drain rating, or fit a FET rated above the trip point. As drawn, the limit protects nothing.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads: []string{
		"param.ocp_threshold", "param.drain_current", "param.on_resistance",
		"component.value", "component.class", "pin.role", "on_net",
	},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail:              ruleDoc("load-switch-trip-above-fet-rating"),
	Eval:                loadSwitchTripVerdicts,
	StatesConsideredSet: true,
}

// loadSwitchTripVerdicts decides every RESOLVED external-FET load switch, one verdict per pass FET,
// and the resolution is what makes the subject set honest here. A switch is only resolved when the
// controller, the FET and the shunt are each unambiguous (check.ExternalFetLoadSwitches), so a
// transistor the walk could not tie to a controller and a sense resistor is not a load switch this
// rule declined to check — as far as the rule can tell it is not a load switch at all, and claiming
// it as a subject would overstate what the netlist supports.
//
// THE UNRATED PASS FET BECOMES NotConsidered, which is the gap this rule's own doc leads with:
// silence here is "I could not tell", never "this is fine". An unseeded FET and a seeded one stating
// no continuous rating are the same gap and both used to leave through one `continue`, so a switch
// whose protection nobody could verify reported exactly what a correctly-sized one did.
//
// check.CompareToBound makes the comparison and the witness together, so a pass here carries the two
// numbers it rests on rather than asserting that the limit protects the FET.
func loadSwitchTripVerdicts(m check.Model) []check.Verdict {
	var out []check.Verdict
	for _, sw := range check.ExternalFetLoadSwitches(m) {
		v := check.Verdict{
			Kind:    check.KindComponent,
			Subject: sw.Fet,
			// The controller whose threshold sets the trip current, and the sense resistor that sets
			// it with them. The subject is the FET, because it is the part that overheats, but the
			// fix is usually one of these two (agni issue 349).
			Context: []check.ContextSubject{
				{Kind: check.KindComponent, Subject: sw.Controller, Role: "controller"},
				{Kind: check.KindComponent, Subject: sw.Sense, Role: "sense"},
			},
		}
		// An unseeded pass element and a seeded one stating no continuous rating are the same gap,
		// and DrainCurrentLimits answers both with an empty slice (the proto getters are
		// nil-tolerant), so one guard covers them.
		fetSpec := m.PartSpec(sw.Fet)
		rated := check.DrainCurrentLimits(fetSpec)
		if len(rated) == 0 {
			v.Outcome = check.NotConsidered
			v.Reason = fmt.Sprintf("the pass FET %s states no continuous drain rating, so the %gA trip current has nothing to be compared against",
				sw.Fet, sw.TripAmps)
			out = append(out, v)
			continue
		}
		// The LOWEST rating binds, the same reasoning fet-vdss-below-switched-rail uses for
		// breakdown. Taking the highest would let a pulsed-condition row excuse a steady
		// over-current.
		id := rated[0]
		for _, p := range rated[1:] {
			if p.Value.GetMax() < id.Value.GetMax() {
				id = p
			}
		}
		outcome, w := check.CompareToBound(sw.TripAmps, "A",
			check.Bound{Max: id.Value.Max}, "trip current", "continuous drain rating")
		if w != nil {
			w.Datasheet = []*check.DatasheetCitation{check.DatasheetCitationOf(fetSpec, id)}
			w.Terms = append(w.Terms, check.WitnessTerm{Label: "sense resistor", Value: sw.Sense})
		}
		v.Outcome, v.Witness = outcome, w
		if outcome != check.Fail {
			out = append(out, v)
			continue
		}

		ctrlSpec := m.PartSpec(sw.Controller)
		msg := fmt.Sprintf(
			"%s is the external pass FET of the load switch %s controls, and that switch does not limit current until %gA (%s %gV across sense resistor %s at %gΩ). %s is rated %s %gA continuous, so it reaches its own limit before the controller acts. Rating: %s. Threshold: %s.",
			sw.Fet, sw.Controller, sw.TripAmps,
			sw.Ocp.Symbol, sw.Ocp.Value.GetMax(), sw.Sense, sw.SenseOhms,
			sw.Fet, id.Symbol, id.Value.GetMax(),
			check.Citation(fetSpec, id), check.Citation(ctrlSpec, sw.Ocp))
		// The effective on-resistance of a controller-based switch is the external FET's RDS(on),
		// which is the number a reviewer needs next. Quoted with an inline citation but NOT added
		// to DatasheetProv: the verdict does not rest on it, and the review's data-trust gate
		// rates a finding by its WEAKEST citation, so an unused low-confidence row would drag a
		// genuine failure down to provisional.
		if sw.OnResistance != nil {
			msg += fmt.Sprintf(" Its effective on-resistance is %s's %s at %gΩ (%s).",
				sw.Fet, sw.OnResistance.Symbol, sw.OnResistance.Value.GetMax(),
				check.Citation(fetSpec, sw.OnResistance))
		}
		v.Finding = &check.Finding{
			Kind:    check.KindComponent,
			Subject: sw.Fet,
			Message: msg,
			Context: v.Context,
			Prov:    componentProv(m, sw.Fet),
			// The endangered part first (the FET carries the rating being exceeded), then the
			// controller's threshold the trip current came from. Both are values the conclusion
			// rests on (WS3-028).
			DatasheetProv: []*check.DatasheetCitation{
				check.DatasheetCitationOf(fetSpec, id),
				check.DatasheetCitationOf(ctrlSpec, sw.Ocp),
			},
		}
		out = append(out, v)
	}
	return out
}

// componentProv locates a component in its source file, or nil when the design carries no provenance
// for it. Findings on a component subject stay locatable in the viewer that way.
func componentProv(m check.Model, refDes string) *ir.Provenance {
	for _, c := range m.Components() {
		if c.GetRefDes() == refDes {
			return c.GetProv()
		}
	}
	return nil
}
