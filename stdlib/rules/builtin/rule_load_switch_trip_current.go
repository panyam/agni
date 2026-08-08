package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// loadSwitchTripAboveFetRating flags a controller-based load switch whose current limit is set above
// the continuous drain rating of the external MOSFET it switches through (WS3-085).
//
// An INTEGRATED load switch is one part, and its current limit and its pass element ship together in
// one datasheet, so the vendor has already made them agree. A CONTROLLER drives a MOSFET the designer
// chose, through a sense resistor the designer also chose, and nothing checks that those three agree.
// When the trip point lands above the FET's rating, the protection is decorative: the FET reaches its
// own limit while the controller is still waiting, so the part meant to be protected fails first, and
// a high-side switch that fails short hands the full rail to the load.
//
// WHAT IT WILL NOT CLAIM. Silence here is "I could not tell", never "this is fine". The switch is only
// resolved when the controller, the FET and the shunt are each unambiguous (see
// check.ExternalFetLoadSwitches), the controller's threshold and the FET's rating are both seeded, and
// the shunt's value is stated in ohms in the design. Any gap and the rule reports nothing at all.
//
// It also says nothing about DERATING. A trip point below the FET's printed rating passes here, and a
// printed rating is a 25C number a real design sizes well under. This catches the unambiguous half,
// where the limit exceeds the vendor's own figure before any derating argument starts.
var loadSwitchTripAboveFetRating = &check.Rule{
	Name:       "load-switch-trip-above-fet-rating",
	Severity:   "error",
	Summary:    "A controller-based load switch trips above the continuous drain rating of its external MOSFET.",
	Impact:     "The current limit never protects the pass element: the external FET reaches its own rating while the controller is still below its trip point, so the part the switch exists to protect is the one that fails, and a shorted high-side switch applies the full rail to the load. Both numbers are vendor values, cited with the page they came from.",
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
	Detail: ruleDoc("load-switch-trip-above-fet-rating"),
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		for _, sw := range check.ExternalFetLoadSwitches(m) {
			fetSpec := m.PartSpec(sw.Fet)
			if fetSpec == nil {
				continue // the pass element is unseeded: no rating to judge against, so no verdict
			}
			rated := check.DrainCurrentLimits(fetSpec)
			if len(rated) == 0 {
				continue
			}
			// The LOWEST rating binds, the same reasoning fet-vdss-below-switched-rail uses for
			// breakdown: a part is endangered at its weakest number. Taking the highest would let a
			// pulsed-condition row excuse a steady over-current.
			id := rated[0]
			for _, p := range rated[1:] {
				if p.Value.GetMax() < id.Value.GetMax() {
					id = p
				}
			}
			if sw.TripAmps <= id.Value.GetMax() {
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
			// which is the number a reviewer needs next (what does the FET dissipate at this current).
			// It is reported with its inline citation but is deliberately NOT added to DatasheetProv:
			// the verdict does not rest on it, and DatasheetProv is what the review's data-trust gate
			// weighs. A finding is rated by its WEAKEST citation, so listing a value the conclusion
			// never used could drag a genuine failure down to provisional on the strength of an
			// unrelated low-confidence row.
			if sw.OnResistance != nil {
				msg += fmt.Sprintf(" Its effective on-resistance is %s's %s at %gΩ (%s).",
					sw.Fet, sw.OnResistance.Symbol, sw.OnResistance.Value.GetMax(),
					check.Citation(fetSpec, sw.OnResistance))
			}
			out = append(out, check.Finding{
				Kind:    check.KindComponent,
				Subject: sw.Fet,
				Message: msg,
				Prov:    componentProv(m, sw.Fet),
				// The endangered part first: the FET carries the rating being exceeded, and the
				// controller carries the threshold the trip current was computed from. Both are values
				// the conclusion rests on, so both are cited (WS3-028).
				DatasheetProv: []*check.DatasheetCitation{
					check.DatasheetCitationOf(fetSpec, id),
					check.DatasheetCitationOf(ctrlSpec, sw.Ocp),
				},
			})
		}
		return out
	},
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
