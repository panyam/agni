package check

import (
	"fmt"
)

// supplyExceedsAbsMax flags a power-input pin fed by a rail whose nominal voltage
// exceeds the part's absolute-maximum supply rating from its seeded datasheet spec.
// The first datasheet-backed rule (WS10-003): purpose-built Go (the join is not spec
// vocabulary yet, the pairwise-geometry precedent), silent without a seeded set.
var supplyExceedsAbsMax = &Rule{
	Name:       "supply-exceeds-abs-max",
	Severity:   "error",
	Summary:    "A power-input pin sits on a rail whose nominal voltage exceeds the part's absolute-maximum supply rating.",
	Impact:     "Exceeding an absolute-maximum rating is outside the vendor's stress envelope: the part may be damaged immediately or degrade in the field. Unlike a heuristic, this limit is the vendor's own number, with the page it came from.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.supply_abs_max", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		KeyCategory:     CategoryDatasheet,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
		"evidence":      "datasheet",
	},
	Detail: ruleDoc("supply-exceeds-abs-max"),
	Eval: func(m Model) []Finding {
		var out []Finding
		for _, c := range m.Components() {
			spec := m.PartSpec(c.RefDes)
			if spec == nil {
				continue
			}
			limits := SupplyAbsMaxLimits(spec)
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
				if pin.Component.RefDes != c.RefDes || !SupplyInputPin(m, c.RefDes, pin.Designator) {
					continue
				}
				net := m.PinNetName(c.RefDes, pin.Designator)
				if net == "" || seen[net] {
					continue
				}
				nominal, ok := NominalVoltageFromName(net)
				if !ok || nominal <= binding.Value.GetMax() {
					continue
				}
				seen[net] = true
				out = append(out, Finding{
					Kind:    KindComponent,
					Subject: c.RefDes,
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV exceeds absolute-maximum %s %gV — %s",
						pin.Designator, net, nominal, binding.Symbol, binding.Value.GetMax(), Citation(spec, binding)),
					Prov:          c.Prov,
					DatasheetProv: DatasheetCitationOf(spec, binding),
				})
			}
		}
		return out
	},
}
