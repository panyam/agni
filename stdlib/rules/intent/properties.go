package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// propertyRule builds the rule for ONE property kind, covering every declared property of that kind.
// One rule per kind (intent/property-reset-polarity, intent/property-ac-coupled) so distinct review
// items bind independently — the same split Protections uses and for the same reason.
//
// It iterates the DECLARATION and probes the design, never the reverse: a net the design has but the
// intent does not mention is not this rule's business.
func propertyRule(kind string, ps []NetProperty) *check.Rule {
	return &check.Rule{
		Name:     "property-" + kind,
		Severity: "warning",
		Summary:  fmt.Sprintf("a net's %s contradicts what the design intent declares", kind),
		Detail:   intentDoc("property-" + kind),
		Impact:   propertyImpact(kind),
		Reads:    []string{"component-on-net", "component.class", "net.ground", "rail"},
		Tags:     intentTags(),
		Eval: func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, p := range ps {
				if p.Property != kind {
					continue
				}
				msg, bad := propertyViolation(m, p)
				if !bad {
					continue
				}
				out = append(out, check.Finding{Kind: check.KindNet, Subject: p.Net, Message: msg})
			}
			return out
		},
	}
}

// propertyViolation reports whether the design contradicts one declared property, and the message to
// say so. The two kinds differ in what absence means, which is the whole subtlety of this rule family:
//
//   - ac-coupled: a series capacitor is decidable, so ABSENT is a violation.
//   - reset-polarity: bias is the only evidence a netlist carries, and a supervisor with an internal
//     pull leaves none, so absent evidence is NOT a violation. Only an OPPOSITE bias is.
func propertyViolation(m check.Model, p NetProperty) (string, bool) {
	switch p.Property {
	case PropACCoupled:
		if seriesCapOn(m, p.Net) {
			return "", false
		}
		return fmt.Sprintf("net %q is declared AC-coupled, but no series capacitor carries it", p.Net), true

	case PropResetPolarity:
		up, down := biasOn(m, p.Net)
		switch p.Value {
		case "low":
			if down {
				return fmt.Sprintf("net %q is declared active-low reset, but it is biased LOW (a pull-down holds it asserted)", p.Net), true
			}
		case "high":
			if up {
				return fmt.Sprintf("net %q is declared active-high reset, but it is biased HIGH (a pull-up holds it asserted)", p.Net), true
			}
		}
		return "", false
	}
	return "", false
}

// seriesCapOn reports whether a capacitor carries the net IN SERIES rather than to ground. A
// decoupling cap has its far side on ground; a coupling cap has it on another signal, and that is the
// difference between the two uses of the same part. A cap whose far side is a rail is likewise not a
// coupling cap.
func seriesCapOn(m check.Model, net string) bool {
	for ref := range componentsOnNet(m, net) {
		if !m.HasClass(ref, check.ComponentClass("capacitor")) {
			continue
		}
		for _, n := range m.Nets() {
			if n.GetName() == net || !touches(n, ref) {
				continue
			}
			if !classify.ActiveRoleVocab().IsGround(n.GetName()) && !m.IsPowerRail(n.GetName()) {
				return true
			}
		}
	}
	return false
}

// biasOn reports which way a resistor biases the net: to a rail (up), to ground (down), or neither.
// Both can hold at once on a divider, in which case the net carries no unambiguous bias and neither
// polarity is contradicted.
func biasOn(m check.Model, net string) (up, down bool) {
	for ref := range componentsOnNet(m, net) {
		if !m.HasClass(ref, check.ComponentClass("resistor")) {
			continue
		}
		for _, n := range m.Nets() {
			if n.GetName() == net || !touches(n, ref) {
				continue
			}
			switch {
			case classify.ActiveRoleVocab().IsGround(n.GetName()):
				down = true
			case m.IsPowerRail(n.GetName()):
				up = true
			}
		}
	}
	if up && down {
		return false, false // a divider biases neither way on its own
	}
	return up, down
}

// touches reports whether refDes has a connection on n.
func touches(n *ir.Net, refDes string) bool {
	for _, c := range n.GetConnections() {
		if c.GetComponentRef() == refDes {
			return true
		}
	}
	return false
}

// propertyImpact is the per-kind impact line shown with a finding.
func propertyImpact(kind string) string {
	switch kind {
	case PropACCoupled:
		return "a link the design intended to be AC-coupled is DC-connected, so a DC offset or a common-mode difference between the two ends reaches the receiver directly"
	case PropResetPolarity:
		return "the design biases a reset line to its ASSERTED level, so the part it resets is held in reset (or released at the wrong moment) rather than running"
	}
	return "a declared property of a net is contradicted by the design"
}
