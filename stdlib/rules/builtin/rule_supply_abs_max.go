package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
)

// supplyExceedsAbsMax flags a power-input pin fed by a rail whose nominal voltage
// exceeds the part's absolute-maximum supply rating from its seeded datasheet spec.
// The first datasheet-backed rule (WS10-003): purpose-built Go (the join is not spec
// vocabulary yet, the pairwise-geometry precedent), silent without a seeded set.
var supplyExceedsAbsMax = &check.Rule{
	Name:       "supply-exceeds-abs-max",
	Severity:   "error",
	Summary:    "A power-input pin sits on a rail whose nominal voltage exceeds the part's absolute-maximum supply rating.",
	Impact:     "Exceeding an absolute-maximum rating is outside the vendor's stress envelope: the part may be damaged immediately or degrade in the field. Unlike a heuristic, this limit is the vendor's own number, with the page it came from.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.supply_abs_max", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("supply-exceeds-abs-max"),
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		for _, c := range m.Components() {
			spec := m.PartSpec(c.RefDes)
			if spec == nil {
				continue
			}
			// A spec with pin bindings is pin-exceeds-abs-max's to answer, per terminal. Deferring
			// here is not only about double-reporting: the most-restrictive-row shortcut below is a
			// FALSE POSITIVE on a part whose supplies differ, since it checks a 6.5 V terminal
			// against a 4.6 V one. The alias path stays for every part without pin data (C9).
			if pinBoundSpec(m, c.RefDes) != nil {
				continue
			}
			limits := check.SupplyAbsMaxLimits(spec)
			if len(limits) == 0 {
				continue
			}
			// The most restrictive comparable abs-max supply row is the binding one.
			binding := limits[0]
			for _, p := range limits[1:] {
				if p.Value.GetMax() < binding.Value.GetMax() {
					binding = p
				}
			}
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
				if !ok || nominal <= binding.Value.GetMax() {
					continue
				}
				seen[net] = true
				out = append(out, check.Finding{
					Kind:    check.KindComponent,
					Subject: c.RefDes,
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV exceeds absolute-maximum %s %gV — %s",
						pin.Designator, net, nominal, binding.Symbol, binding.Value.GetMax(), check.Citation(spec, binding)),
					Prov:          c.Prov,
					DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(spec, binding)},
					// The pin and the rail, in the order the message names them. The PIN matters as
					// much as the net here: the subject is the whole part, and a part can have several
					// supply pins, so highlighting the part alone cannot say which one is over its
					// limit (agni issue 349). No NetID: this rule has the rail by name only.
					Context: []check.ContextSubject{
						{Kind: check.KindPin, Subject: c.RefDes, Pin: pin.Designator, Role: "pin"},
						{Kind: check.KindNet, Subject: net, Role: "rail"},
					},
				})
			}
		}
		return out
	},
}
