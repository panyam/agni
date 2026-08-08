package intent

import (
	"fmt"

	"github.com/panyam/agni/core/check"
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
// say so. The kinds differ in what absence means, which is the whole subtlety of this rule family:
//
//   - ac-coupled: a series capacitor is decidable, so ABSENT is a violation.
//   - reset-polarity: bias is the only evidence a netlist carries, and a supervisor with an internal
//     pull leaves none, so absent evidence is NOT a violation. Only a bias TOWARD the asserted level is.
//   - strap: the same evidence read the other way round. The declared level is the one the pin should
//     LATCH, so a bias toward it is correct and a bias AWAY from it is the violation, in either
//     direction. Absent bias is silent for the reset-polarity reason (internal pulls).
func propertyViolation(m check.Model, p NetProperty) (string, bool) {
	switch p.Property {
	case PropACCoupled:
		if netNamed(m, p.Net) == nil || check.ACCoupled(m, netNamed(m, p.Net)) {
			return "", false
		}
		return fmt.Sprintf("net %q is declared AC-coupled, but no series capacitor carries it", p.Net), true

	case PropResetPolarity:
		n := netNamed(m, p.Net)
		if n == nil {
			return "", false // a declared net absent from the design is not a contradiction
		}
		up, down := check.NetBias(m, n)
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

	case PropStrap:
		n := netNamed(m, p.Net)
		if n == nil {
			return "", false
		}
		up, down := check.NetBias(m, n)
		// Only an OPPOSITE bias is a contradiction. Neither direction (no bias, or a divider holding
		// neither rail) leaves the latched level unevidenced, which is not the same as wrong.
		switch {
		case p.Value == "high" && down:
			return fmt.Sprintf("net %q is declared to strap HIGH, but it is biased LOW (a pull-down selects the opposite configuration)", p.Net), true
		case p.Value == "low" && up:
			return fmt.Sprintf("net %q is declared to strap LOW, but it is biased HIGH (a pull-up selects the opposite configuration)", p.Net), true
		}
		return "", false
	}
	return "", false
}

// netNamed returns the design net with this exact name, or nil when the declaration names a net the
// design does not have. The property rules iterate the DECLARATION, so an absent net is silence: the
// presence forms (modules, subsystems) are what report a missing thing.
func netNamed(m check.Model, name string) *ir.Net {
	for _, n := range m.Nets() {
		if n.GetName() == name {
			return n
		}
	}
	return nil
}

// propertyImpact is the per-kind impact line shown with a finding.
func propertyImpact(kind string) string {
	switch kind {
	case PropACCoupled:
		return "a link the design intended to be AC-coupled is DC-connected, so a DC offset or a common-mode difference between the two ends reaches the receiver directly"
	case PropResetPolarity:
		return "the design biases a reset line to its ASSERTED level, so the part it resets is held in reset (or released at the wrong moment) rather than running"
	case PropStrap:
		return "a config strap latches the OPPOSITE of the intended level at reset, so the part comes up in the wrong boot mode, bus width or device address, and a colliding address takes its bus down with it"
	}
	return "a declared property of a net is contradicted by the design"
}
