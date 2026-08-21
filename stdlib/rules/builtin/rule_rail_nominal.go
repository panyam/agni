package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
)

// railNominalOutOfRecommended flags a power-input pin fed by a rail whose nominal
// voltage falls outside the part's recommended operating supply range from its seeded
// datasheet spec. The recommended-operating sibling of supply-exceeds-abs-max
// (WS3-036): same datasheet join, but the vendor's functional envelope (min..max)
// rather than the destroy-it ceiling. A rail outside the recommended range is not the
// guaranteed damage an abs-max breach is; the part may still run, but it is operating
// outside the conditions its datasheet specs are guaranteed under, so margin, accuracy,
// and lifetime are no longer assured.
//
// It acts only on a part that declares a SINGLE recommended supply row: a netlist does
// not label which power-in pin is which supply, and the range is two-sided, so a
// multi-supply part can't be checked without risking a false over/under finding
// (per-pin supply mapping is a follow-up). supply-exceeds-abs-max carries no such
// restriction because its one-sided ceiling is conservative to apply across pins.
// Silent by construction without a seeded params set (Model.PartSpec is nil).
var railNominalOutOfRecommended = &check.Rule{
	Name:       "rail-nominal-out-of-recommended",
	Severity:   "warning",
	Summary:    "A power-input pin sits on a rail whose nominal voltage is outside the part's recommended operating supply range.",
	Impact:     "Outside the recommended operating range the datasheet's guaranteed specifications no longer hold: the part may still function, but its behavior is uncharacterized and margin, accuracy, and lifetime are no longer assured. Unlike a heuristic, the range is the vendor's own number, with the page it came from.",
	Remedy:     "Bring the rail inside the part's recommended operating range, or accept the excursion in writing with the reason. Outside that window the datasheet's numbers no longer apply.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.recommended_operating", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("rail-nominal-out-of-recommended"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		var out []check.Finding
		for _, c := range m.Components() {
			spec := m.PartSpec(c.RefDes)
			if spec == nil {
				continue
			}
			// A spec with pin bindings is pin-out-of-recommended's to answer. That rule is the
			// per-pin supply mapping this one's header names as the follow-up, so where it applies
			// the single-row restriction below is no longer the best available answer.
			if pinBoundSpec(m, c.RefDes) != nil {
				continue
			}
			// Exactly one recommended supply row: 0 => nothing to check; >1 => the
			// pin-to-supply mapping is ambiguous, so skip rather than risk a false finding.
			limits := check.RecommendedOperatingLimits(spec)
			if len(limits) != 1 {
				continue
			}
			binding := limits[0]
			hasLo, hasHi := binding.Value.Min != nil, binding.Value.Max != nil
			lo, hi := binding.Value.GetMin(), binding.Value.GetMax()
			seen := map[string]bool{}
			for _, pin := range m.Pins() {
				if pin.Component.RefDes != c.RefDes || !check.SupplyInputPin(m, c.RefDes, pin.Designator) {
					continue
				}
				net := m.PinNetName(c.RefDes, pin.Designator)
				if net == "" || seen[net] {
					continue
				}
				nominal, ok := check.NominalVoltageFromName(net)
				if !ok {
					continue
				}
				var rel string
				switch {
				case hasHi && nominal > hi:
					rel = fmt.Sprintf("exceeds recommended maximum %gV", hi)
				case hasLo && nominal < lo:
					rel = fmt.Sprintf("is below recommended minimum %gV", lo)
				default:
					continue
				}
				seen[net] = true
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: c.RefDes,
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV %s for %s — %s",
						pin.Designator, net, nominal, rel, binding.Symbol, check.Citation(spec, binding)),
					Prov: c.Prov,
					// The pin and the rail, as in supply-exceeds-abs-max: a part with several supply
					// pins is not located by its ref des alone.
					Context: []check.ContextSubject{
						{Kind: check.KindPin, Subject: c.RefDes, Pin: pin.Designator, Role: "pin"},
						{Kind: check.KindNet, Subject: net, Role: "rail"},
					},
				})
			}
		}
		return out
	}),
}
