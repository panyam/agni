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
				if msg, undecidable := propertyUndecidable(m, p); undecidable {
					out = append(out, check.Finding{Kind: check.KindNet, Subject: p.Net, Message: msg, Inconclusive: true})
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

// propertyUndecidable reports the subjects this rule family can look at and genuinely cannot decide,
// and the message saying so (agni issue 74). It runs BEFORE propertyViolation, so an undecidable
// subject never reaches the contradiction test and never lands in the silence that used to read pass.
//
// Only reset-polarity has such a case, and it is PERMANENT rather than a data gap: a netlist states
// polarity nowhere, the only structural evidence is a bias resistor, and a reset driven by a
// supervisor with an internal pull carries none. No seeding, declaration or fact tier will ever
// supply it, which is exactly why this could not be reported as needs-data.
//
// ac-coupled and strap are deliberately absent. A series capacitor is decidable by looking, so absent
// means the declaration is unmet. A strap with no bias is the DEFAULT-state case a datasheet tells
// you to leave unfitted, so silence there is a correct pass rather than an unanswered question, and
// emitting inconclusive for it would flag most correct boards.
func propertyUndecidable(m check.Model, p NetProperty) (string, bool) {
	if p.Property != PropResetPolarity {
		return "", false
	}
	n := netNamed(m, p.Net)
	if n == nil {
		return "", false // absent from the design; the presence forms report missing things
	}
	if up, down := check.NetBias(m, n); up || down {
		return "", false // biased one way or the other, so the contradiction test can decide
	}
	// NetBias reports neither for TWO different designs, and telling a reviewer the wrong one wastes
	// their time at the schematic: a net with no bias resistor at all, and a DIVIDER, which reports
	// neither because it holds the line at an intermediate level rather than at either rail. Both are
	// undecidable here, and the next step differs.
	if dividerOn(m, n) {
		return fmt.Sprintf(
			"net %q is declared an active-%s reset, but a divider holds it at an intermediate level rather "+
				"than at either rail, so which level the receiver reads cannot be determined from the "+
				"netlist. Verify the divider ratio against the part's input thresholds.", p.Net, p.Value), true
	}
	return fmt.Sprintf(
		"net %q is declared an active-%s reset, but the design carries no bias on it, so its resting level "+
			"cannot be determined from the netlist. Verify by hand that the driver holds it de-asserted "+
			"(a supervisor or PMIC with an internal pull is normal and correct).", p.Net, p.Value), true
}

// dividerOn reports whether the net carries BOTH a pull-up and a pull-down, which is what makes
// check.NetBias answer neither. It re-asks the two questions separately rather than exposing a third
// return from NetBias, because every other caller wants the two-value answer and only the message
// needs to tell the two no-answer cases apart.
func dividerOn(m check.Model, n *ir.Net) bool {
	up, down := false, false
	for _, c := range n.GetConnections() {
		ref := c.GetComponentRef()
		if !m.HasClass(ref, check.ClassResistor) {
			continue
		}
		for _, far := range m.Nets() {
			if far.GetName() == n.GetName() || !connectsRef(far, ref) {
				continue
			}
			if m.IsGroundNet(far) {
				down = true
			} else if m.IsPowerRail(far.GetName()) {
				up = true
			}
		}
	}
	return up && down
}

// connectsRef reports whether refDes has a connection on n.
func connectsRef(n *ir.Net, refDes string) bool {
	return check.Exists(n.GetConnections(), func(c *ir.Connection) bool {
		return c.GetComponentRef() == refDes
	})
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
