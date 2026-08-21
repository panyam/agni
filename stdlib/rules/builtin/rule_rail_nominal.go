package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
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
	Eval: func(m check.Model) []check.Verdict {
		return aliasSupplyVerdicts(m, check.RecommendedOperatingLimits, singleRecommendedRow,
			"recommended operating supply", "recommended range",
			func(p *parampb.Parameter) check.Bound { return check.Bound{Min: p.Value.Min, Max: p.Value.Max} },
			func(ev aliasSupplyEvent, binding *parampb.Parameter, b check.Bound) *check.Finding {
				// Which side was crossed, for the message only. The OUTCOME came from the comparison
				// in aliasSupplyVerdicts, so the two cannot disagree about whether this is a breach.
				rel := fmt.Sprintf("exceeds recommended maximum %gV", binding.Value.GetMax())
				if b.Min != nil && ev.nominal < *b.Min {
					rel = fmt.Sprintf("is below recommended minimum %gV", binding.Value.GetMin())
				}
				return &check.Finding{
					Subject: check.Entity{Kind: check.KindComponent, Ref: ev.comp.RefDes},
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV %s for %s — %s",
						ev.pin, ev.net, ev.nominal, rel, binding.Symbol, check.Citation(ev.spec, binding)),
					Prov: ev.comp.Prov,
					// The pin and the rail, as in supply-exceeds-abs-max: a part with several supply
					// pins is not located by its ref des alone.
					Context: aliasSupplyContext(ev),
				}
			})
	},
	StatesConsideredSet: true,
}

// singleRecommendedRow picks the binding row for the two-sided range, and refuses where a part states
// more than one. That refusal is the rule's documented restriction and now has somewhere to go: a
// netlist does not label which power-in pin is which supply, so applying one part's range to the wrong
// terminal invents an over- or under-voltage. supply-exceeds-abs-max carries no such restriction
// because its one-sided ceiling is conservative to apply across pins. pin-out-of-recommended is the
// per-pin mapping that answers these parts properly, and it owns any part whose spec binds pins.
func singleRecommendedRow(rows []*parampb.Parameter) (*parampb.Parameter, string) {
	if len(rows) > 1 {
		return nil, fmt.Sprintf("the datasheet states %d recommended supply ranges and a netlist does not say "+
			"which supply pin each belongs to, so applying one of them here could invent an over- or under-voltage", len(rows))
	}
	return rows[0], ""
}
