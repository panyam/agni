package intent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// propertyRule builds the rule for ONE property kind, covering every declared property of that kind.
// One rule per kind, named intent/property-reset-polarity, intent/property-ac-coupled, the same split
// protectionRule uses.
func propertyRule(kind string, ps []NetProperty) *check.Rule {
	return &check.Rule{
		Name:                "property-" + kind,
		Severity:            "warning",
		Summary:             fmt.Sprintf("a net's %s contradicts what the design intent declares", kind),
		Detail:              intentDoc("property-" + kind),
		Impact:              propertyImpact(kind),
		Remedy:              intentRemedy("property-" + kind),
		Reads:               []string{"component.net", "component.class", "net.ground", "net.rail"},
		Tags:                intentTags(),
		Eval:                func(ctx context.Context, m check.Model) []check.Verdict { return propertyVerdicts(m, ps, kind) },
		StatesConsideredSet: true,
	}
}

// propertyVerdicts decides every declared property OF THIS KIND, one verdict each.
//
// A property of another kind is not this rule's subject and yields nothing. The families compile to
// one rule per kind, so `property-ac-coupled` reporting on a reset-polarity declaration would claim a
// check it never made (the fet-vdss scope bug, PR 405).
//
// The undecidable case is check.Inconclusive, which projects to an inconclusive finding (agni issue
// 74). A declared property the design honours reports a PASS, so it reads differently from a property
// nobody declared.
func propertyVerdicts(m check.Model, ps []NetProperty, kind string) []check.Verdict {
	var out []check.Verdict
	for _, p := range ps {
		if p.Property != kind {
			continue // another kind's declaration, not a subject of this rule
		}
		v := check.Verdict{Subjects: []check.Entity{check.NetNameEntity(p.Net)}}
		msg, undecidable := propertyUndecidable(m, p)
		bad := false
		if !undecidable {
			msg, bad = propertyViolation(m, p)
		}
		if !undecidable && !bad {
			msg, undecidable = strapBandUnjudged(m, p)
		}
		switch {
		case undecidable:
			v.Outcome = check.Inconclusive
			v.Witness = &check.Witness{Statement: msg}
			v.Finding = &check.Finding{Subject: check.NetNameEntity(p.Net), Message: msg, Inconclusive: true}
		case bad:
			v.Outcome = check.Fail
			v.Witness = &check.Witness{Statement: msg}
			v.Finding = &check.Finding{Subject: check.NetNameEntity(p.Net), Message: msg}
		default:
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("net %q carries the declared %s property and the design does not contradict it", p.Net, kind),
				Terms:     []check.WitnessTerm{{Label: "declared", Value: kind}},
			}
		}
		out = append(out, v)
	}
	return out
}

// propertyViolation reports whether the design contradicts one declared property, and the message to
// say so. The kinds differ in what absence means:
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
		refs, up, down := check.NetBiasResistors(m, n)
		// The VALUE half (WS3-119). Checked before the direction half because a strap can be pulled
		// the right way by a resistor of the wrong value.
		if msg, bad := strapValueViolation(m, p, refs); bad {
			return msg, true
		}
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

// strapValueViolation checks a strap's pull resistor against the band the DECLARATION states, and
// reports only a value it read and found outside the band (WS3-119). A band it cannot compare is
// strapBandUnjudged's to report.
//
// The finding quotes the SOURCE TEXT ("10k"), not the parsed 10000, so a reviewer reads the value
// they will see on the schematic.
func strapValueViolation(m check.Model, p NetProperty, refs []string) (string, bool) {
	// No explicit "no band declared" early-out. An undeclared bound is zero and both comparisons
	// below are gated on `> 0`, so an undeclared band already reports nothing. Mutation testing
	// showed a guard here was unreachable.
	if len(refs) != 1 {
		return "", false
	}
	ref := refs[0]
	ohms, ok := check.ComponentValueIn(m, ref, check.UnitOhm)
	if !ok {
		return "", false
	}
	text, _ := check.ComponentValueText(m, ref)
	if text == "" {
		text = fmt.Sprintf("%g", ohms)
	}
	switch {
	case p.MinOhms > 0 && ohms < p.MinOhms:
		return fmt.Sprintf("net %q straps through %s at %s, below the declared minimum of %s: "+
			"too strong a pull fights whatever drives the line",
			p.Net, ref, text, ohmsText(p.MinOhms)), true
	case p.MaxOhms > 0 && ohms > p.MaxOhms:
		return fmt.Sprintf("net %q straps through %s at %s, above the declared maximum of %s: "+
			"too weak a pull may not hold the level against pin leakage",
			p.Net, ref, text, ohmsText(p.MaxOhms)), true
	}
	return "", false
}

// strapBandUnjudged reports a declared band that was not compared, and why (agni issue 1281). It runs
// after the violation checks because a wrong-way strap is a defect whatever its value reads as.
//
// Unreadable is neither a defect nor a pass, since nothing shows the band held and nothing shows it
// broke. Three shapes land here: a resistor with no value, a value that does not read as a resistance,
// and several resistors biasing the net, because a divider's two resistors set a level together and
// neither is "the" strap pull. A net with no strap resistor is not one of them. Its band has nothing
// to bound, and leaving the resistor off is the datasheet's default-state case.
func strapBandUnjudged(m check.Model, p NetProperty) (string, bool) {
	if p.Property != PropStrap || (p.MinOhms <= 0 && p.MaxOhms <= 0) {
		return "", false
	}
	n := netNamed(m, p.Net)
	if n == nil {
		return "", false
	}
	refs, _, _ := check.NetBiasResistors(m, n)
	band := bandText(p)
	switch len(refs) {
	case 0:
		return "", false
	case 1:
		ref := refs[0]
		if _, ok := check.ComponentValueIn(m, ref, check.UnitOhm); ok {
			return "", false
		}
		if text, _ := check.ComponentValueText(m, ref); text != "" {
			return fmt.Sprintf("net %q declares a strap band of %s, but %s reads %q, which is not a resistance, "+
				"so the band was not checked", p.Net, band, ref, text), true
		}
		return fmt.Sprintf("net %q declares a strap band of %s, but %s carries no value, so the band was not checked",
			p.Net, band, ref), true
	}
	return fmt.Sprintf("net %q declares a strap band of %s, but %s bias it together, so no one resistor can be "+
		"held to the band and it was not checked", p.Net, band, strings.Join(refs, ", ")), true
}

// bandText renders a declared band, one-sided or not.
func bandText(p NetProperty) string {
	switch {
	case p.MinOhms > 0 && p.MaxOhms > 0:
		return ohmsText(p.MinOhms) + " to " + ohmsText(p.MaxOhms)
	case p.MinOhms > 0:
		return "at least " + ohmsText(p.MinOhms)
	}
	return "at most " + ohmsText(p.MaxOhms)
}

// ohmsText renders a declared bound the way an engineer writes it (4700 -> "4.7k").
func ohmsText(ohms float64) string {
	switch {
	case ohms >= 1e6:
		return trimNum(ohms/1e6) + "M"
	case ohms >= 1e3:
		return trimNum(ohms/1e3) + "k"
	}
	return trimNum(ohms)
}

func trimNum(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// propertyUndecidable reports the subjects this rule family can look at and genuinely cannot decide,
// and the message saying so (agni issue 74). It runs BEFORE propertyViolation, so an undecidable
// subject never reaches the contradiction test.
//
// Only reset-polarity has such a case, and it is PERMANENT rather than a data gap. A netlist states
// polarity nowhere, the only structural evidence is a bias resistor, and a reset driven by a
// supervisor with an internal pull carries none. No seeding, declaration or fact tier will ever
// supply it, which is why this is not reported as needs-data.
//
// ac-coupled and a strap's direction have no undecidable case. A series capacitor is decidable by
// looking, so absent means the declaration is unmet. A strap with no bias is the DEFAULT-state case a
// datasheet tells you to leave unfitted, so silence there is a correct pass rather than an unanswered
// question, and emitting inconclusive for it would flag most correct boards. A strap's declared band
// can be undecidable, which strapBandUnjudged reports once the direction has had its say.
func propertyUndecidable(m check.Model, p NetProperty) (string, bool) {
	if p.Property != PropResetPolarity {
		return "", false
	}
	n := netNamed(m, p.Net)
	if n == nil {
		return "", false // absent from the design; see netNamed
	}
	if up, down := check.NetBias(m, n); up || down {
		return "", false // biased one way or the other, so the contradiction test can decide
	}
	// NetBias reports neither for TWO different designs, a net with no bias resistor at all and a
	// DIVIDER holding the line at an intermediate level rather than at either rail. Both are
	// undecidable here, but the next step at the schematic differs, so the message names which one.
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
// check.NetBias answer neither. It re-asks the two questions separately rather than adding a third
// return to NetBias, whose other callers all want the two-value answer.
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
// design does not have. An absent net is silent here, because the presence forms (modules,
// subsystems) are what report a missing thing.
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
