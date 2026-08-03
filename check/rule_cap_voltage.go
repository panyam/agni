package check

import (
	"fmt"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// capVoltageDerate is the margin multiplier the rated voltage must clear:
// Vrated >= rail_V x derate. 1.25 is the common 20%-derating convention for
// ceramics; making it a per-run parameter rides the WS3-006 parameterization rider,
// and a computed (rather than declared) worst-case rail voltage is WS4.
const capVoltageDerate = 1.25

// capVoltage is the cap-voltage rule (WS10-005, the M3 demo): a capacitor joined to
// its seeded datasheet spec must have a rated voltage clearing the worst rail it
// touches times the derate factor. The first spec-authored datasheet rule: the body
// is a Spec value with no Go twin, and the join/compare lives behind the
// cap_voltage_detail SpecFunc, whose declared Reads flow into the rule's derived
// metadata (the WS3-004 fact capture). Registered inside this initializer because
// Spec.Rule validates Call targets at bind time.
var capVoltage = func() *Rule {
	RegisterSpecFunc("cap_voltage_detail", &SpecFunc{
		Reads:      []string{"param.cap_rated_voltage", "net.max_voltage", "component.mpn", "on_net"},
		Primitives: []string{"param-join", "traverse"},
		Fn: func(m Model, ents map[string]any, _ []any) any {
			return capVoltageDetail(m, ents["component"].(*ir.Component))
		},
	})
	return (&Spec{
		Over: "components",
		Let:  map[string]Term{"detail": Call{Fn: "cap_voltage_detail"}},
		Where: And{Xs: []Expr{
			Cmp{L: Fact{Name: "component.class"}, Op: "==", R: Lit{V: "capacitor"}},
			Cmp{L: Var{Name: "detail"}, Op: "!=", R: Lit{V: ""}},
		}},
		Message: "{detail}",
	}).Rule(Rule{
		Name:     "cap-voltage",
		Severity: "error",
		Summary:  "A capacitor's datasheet rated voltage does not clear the worst rail it touches times the derate factor.",
		Impact:   "A cap run at or above its rated voltage ages fast and fails early (ceramics also lose most of their capacitance near rating). Unlike a heuristic, the limit here is the vendor's own rated voltage, cited to its page.",
		Tags: map[string]string{
			KeyCategory:     CategoryDatasheet,
			KeyTier:         "R",
			KeyDistribution: DistOpen,
			"evidence":      "datasheet",
		},
		Detail: ruleDoc("cap-voltage"),
	})
}()

// capVoltageDetail is the cap_voltage_detail SpecFunc body: the join, the worst-rail
// scan, and the derated compare. It returns the full violation message (with the
// datasheet citation) or "" for pass and for every skip case, so the Spec's Where
// clause and Message share one memoized Let binding.
func capVoltageDetail(m Model, c *ir.Component) string {
	spec := m.PartSpec(c.RefDes)
	if spec == nil {
		return ""
	}
	limits := capRatedVoltageLimits(spec)
	if len(limits) == 0 {
		return ""
	}
	binding := limits[0]
	for _, p := range limits[1:] {
		if p.Value.GetMax() < binding.Value.GetMax() {
			binding = p
		}
	}
	worstNet, worstV, found := "", 0.0, false
	for _, n := range m.Nets() {
		onNet := false
		for _, conn := range n.Connections {
			if conn.ComponentRef == c.RefDes {
				onNet = true
				break
			}
		}
		if !onNet {
			continue
		}
		if v, ok := railMaxVoltage(n, n.Name); ok && (!found || v > worstV) {
			worstNet, worstV, found = n.Name, v, true
		}
	}
	if !found {
		return ""
	}
	required := worstV * capVoltageDerate
	if binding.Value.GetMax() >= required {
		return ""
	}
	return fmt.Sprintf("capacitor %s (MPN %s): rated voltage %s %gV is below %gV required (rail %q %gV x derate %g) — %s",
		c.RefDes, m.ComponentMPN(c.RefDes), binding.Symbol, binding.Value.GetMax(),
		required, worstNet, worstV, capVoltageDerate, citation(spec, binding))
}
