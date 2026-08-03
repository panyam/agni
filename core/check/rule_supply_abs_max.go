package check

import (
	"fmt"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
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
			limits := supplyAbsMaxLimits(spec)
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
				if pin.Component.RefDes != c.RefDes || !supplyInputPin(m, c.RefDes, pin.Designator) {
					continue
				}
				net := m.PinNetName(c.RefDes, pin.Designator)
				if net == "" || seen[net] {
					continue
				}
				nominal, ok := nominalVoltageFromName(net)
				if !ok || nominal <= binding.Value.GetMax() {
					continue
				}
				seen[net] = true
				out = append(out, Finding{
					Kind:    KindComponent,
					Subject: c.RefDes,
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV exceeds absolute-maximum %s %gV — %s",
						pin.Designator, net, nominal, binding.Symbol, binding.Value.GetMax(), citation(spec, binding)),
					Prov:          c.Prov,
					DatasheetProv: datasheetCitationOf(spec, binding),
				})
			}
		}
		return out
	},
}

// citation renders the datasheet side of a finding's dual provenance as a message string: which
// document revision, page, and table the limit came from, and how it was extracted. The design side
// travels in Finding.Prov; the same facts also travel structured in Finding.DatasheetProv (built via
// datasheetCitationOf, which this shares), so a renderer can column them instead of parsing this text.
func citation(spec *parampb.PartSpec, p *parampb.Parameter) string {
	c := datasheetCitationOf(spec, p)
	doc := c.Doc
	if doc == "" {
		doc = "unknown source"
	}
	return fmt.Sprintf("datasheet %q page %d, %q (%s, confidence %g)",
		doc, c.Page, c.Section, c.Method, c.Confidence)
}
