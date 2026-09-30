package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The per-pin datasheet rules (agni issue 190), the pin-binding counterparts of
// supply-exceeds-abs-max and rail-nominal-out-of-recommended.
//
// WHAT THEY FIX. The alias-path rules reach a terminal through a vendor symbol table meeting a
// pin-type inference, which cannot tell two pins of one part apart. supply-exceeds-abs-max resolves
// that by applying the MOST RESTRICTIVE row across every power-in pin, which is conservative and,
// on a part whose terminals genuinely differ, wrong. A translator rated 4.6 V on one supply and
// 6.5 V on the other reports a 5 V rail on the 6.5 V terminal as a violation. And
// rail-nominal-out-of-recommended declines multi-supply parts outright, because a two-sided range
// applied to the wrong terminal produces a false over- or under-voltage.
//
// THE ALIAS PATH STAYS. These act only on a part whose spec carries pin bindings, and the alias
// rules defer only on such a part, so exactly one rule speaks per part and a design read against a
// corpus seeded before pin binding behaves exactly as it did (CONSTRAINTS C9).
//
// WHY GO AND NOT DATALOG, although the pin relations exist. Mapping a design pin onto a spec pin
// can REFUSE, on an ambiguous name with no package identified or a name and number that disagree.
// A datalog join cannot express refusal, so it would silently drop the pin or cross-join.
// param.ResolvePin owns that decision, so the rules call it and declare the relations they consume
// for the availability gate.

// pinBoundSpec returns the seeded spec for a component when it carries pin bindings, else nil. It
// is the switch between the two paths. A nil result means the alias rules own this part and non-nil
// means these do. Both sides read it, so the two cannot overlap or leave a gap.
func pinBoundSpec(m check.Model, refDes string) *parampb.PartSpec {
	spec := m.PartSpec(refDes)
	if spec == nil || len(spec.GetPins()) == 0 {
		return nil
	}
	for _, p := range spec.GetParameters() {
		if len(p.GetPinRefs()) > 0 {
			return spec
		}
	}
	// Pins declared but nothing bound to them leaves no per-pin limit to check, so the alias path
	// answers instead.
	return nil
}

// pinEvent is one supply terminal the rule was applied to, and it is the unit of the CONSIDERED
// SET. Every terminal in scope produces exactly one, whether or not it could be judged.
//
// It is PIN-SHAPED rather than row-shaped. A row-shaped event could only speak about terminals the
// datasheet has a usable row for, so a resolved pin the datasheet ignored would be absent for the
// same reason a clean pin is. It would also give a terminal carrying two rows of one kind two
// verdicts, a duplicate identity once verdicts are addressable.
type pinEvent struct {
	component  *ir.Component
	designator string
	pinName    string

	// drop is why this terminal cannot be judged, in the author's words, and empty when it can.
	// When set, every field below is unset and the rule emits NotConsidered.
	drop string

	net   string
	volts float64
	spec  *parampb.PartSpec
	rows  []*parampb.Parameter // comparable rows of the requested kind bound to this terminal
}

// eachSupplyPin walks every supply-input terminal of every pin-bound part and yields one event per
// terminal.
//
// The two `continue`s before the event is built are NOT drops and yield nothing. A part with no pin
// bindings belongs to the alias rules, and a pin that is not a supply input is not a subject of
// these rules at all. A drop is a terminal the rule DOES claim and cannot answer for.
func eachSupplyPin(m check.Model, kind parampb.LimitKind, kindName string, yield func(pinEvent)) {
	for _, c := range m.Components() {
		spec := pinBoundSpec(m, c.RefDes)
		if spec == nil {
			continue // not pin-bound, so the alias rules own this part
		}
		// The design's MPN may carry the package suffix; when it does not, PackageForMPN returns nil
		// and ResolvePin falls back to requiring cross-package agreement rather than assuming a body.
		pkg := ""
		if p := param.PackageForMPN(spec, m.ComponentMPN(c.RefDes)); p != nil {
			pkg = p.GetId()
		}
		for _, pin := range m.Pins() {
			if pin.Component.RefDes != c.RefDes || !check.SupplyInputPin(m, c.RefDes, pin.Designator) {
				continue // not a supply terminal, so not a subject of this rule
			}
			ev := pinEvent{component: c, designator: pin.Designator}
			specPin, err := param.ResolvePin(spec, m.PinName(c.RefDes, pin.Designator), pin.Designator, pkg)
			if err != nil {
				ev.drop = "pin could not be resolved to a datasheet terminal"
				yield(ev)
				continue
			}
			ev.pinName = specPin.GetName()
			net := m.PinNetName(c.RefDes, pin.Designator)
			if net == "" {
				ev.drop = "pin is not connected to a net"
				yield(ev)
				continue
			}
			volts, ok := check.NominalVoltageFromName(net)
			if !ok {
				ev.drop = fmt.Sprintf("rail %q does not name a nominal voltage", net)
				yield(ev)
				continue
			}
			ev.net, ev.volts, ev.spec = net, volts, spec
			for _, row := range param.PinParameters(spec, specPin.GetId()) {
				q, ok := param.InBaseUnit(row)
				if !ok || q.LimitKind != kind || q.Unit != "V" ||
					param.UnderSpecified(q) || !param.MachineComparable(q) {
					continue
				}
				ev.rows = append(ev.rows, q)
			}
			if len(ev.rows) == 0 {
				ev.drop = "the datasheet binds no comparable " + kindName + " row to this terminal"
			}
			yield(ev)
		}
	}
}

// bindingRow reduces a terminal's rows to the one that governs, the row the design has least margin
// against. It is only called with a non-empty list, since an empty one is a drop.
//
// Most-restrictive-wins matches what supply-exceeds-abs-max does across a part's supply pins.
// Reporting the first row enumerated would mislead, since a terminal that clears a 6.5 V row while
// sitting against a 5.0 V one is not fine and nothing in the output would say which row was read.
//
// check.Bound.Margin makes the choice one comparison for every bound shape. A violated row has a
// negative margin so it beats any passing row, and a row stating no bound has none so it loses to
// any real limit, which leaves NoLimit for the terminal whose rows ALL state nothing.
func bindingRow(rows []*parampb.Parameter, volts float64, boundOf func(*parampb.Parameter) check.Bound) (*parampb.Parameter, check.Bound) {
	best, bestBound := rows[0], boundOf(rows[0])
	bestMargin := bestBound.Margin(volts)
	for _, r := range rows[1:] {
		b := boundOf(r)
		if mg := b.Margin(volts); mg < bestMargin {
			best, bestBound, bestMargin = r, b, mg
		}
	}
	return best, bestBound
}

// PROOF ON PASS (#387). The two rules below decide through check.CompareToBound and produce one
// check.Verdict per supply TERMINAL, with Eval projecting the failures back out as findings.
//
// A pass carries evidence a reviewer can check, such as "3.3 V is within the absolute maximum of
// 3.6 V" plus the citation.
//
// A row stating no maximum is check.NoLimit, not a pass. The rule-level gates (Reads,
// RequiresCapability, ParamSymbols) cannot see that false pass per datasheet ROW.
//
// A terminal the rule cannot judge is a NotConsidered verdict carrying the step that stopped it
// (pinEvent.drop), so the verdict list is the considered set.

// pinVerdict builds the Verdict common to both rules. The verdict is PIN-scoped because the question
// it answers is "why is this terminal fine", while the Finding it may carry stays COMPONENT-scoped,
// which is what the viewer highlights and what every existing test asserts.
//
// row is the binding row the outcome was decided against, and nil for a verdict that reached no row.
func pinVerdict(rule string, ev pinEvent, outcome check.Outcome, w *check.Witness, row *parampb.Parameter) check.Verdict {
	if w != nil && row != nil {
		w.Datasheet = []*check.DatasheetCitation{check.DatasheetCitationOf(ev.spec, row)}
	}
	// The rail only. This verdict's SUBJECT is the terminal, so listing the pin as context too would
	// name it twice. The Finding does list it, because a finding's subject is the whole component
	// (see pinContext).
	var ctx []check.ContextSubject
	if ev.net != "" {
		ctx = []check.ContextSubject{{Entity: check.Entity{Kind: check.KindNet, Ref: ev.net}, Role: "rail"}}
	}
	return check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindPin, Ref: ev.component.RefDes, Pin: ev.designator}}, Rule: rule, Outcome: outcome, Witness: w, Context: ctx}
}

// pinLimitVerdicts is the body both per-pin rules share. It enumerates the terminals, reduces each
// one's rows to the row that binds, judges it, and lets the caller phrase the failing case. The caller
// supplies the bound's shape and the sentence.
func pinLimitVerdicts(
	m check.Model,
	rule string,
	kind parampb.LimitKind,
	kindName string, // names the kind in a drop reason: "absolute-maximum"
	limitName string, // names the bound in a witness: "absolute maximum"
	boundOf func(*parampb.Parameter) check.Bound,
	findingFor func(pinEvent, *parampb.Parameter, check.Bound) *check.Finding,
) []check.Verdict {
	var out []check.Verdict
	eachSupplyPin(m, kind, kindName, func(ev pinEvent) {
		if ev.drop != "" {
			v := pinVerdict(rule, ev, check.NotConsidered, nil, nil)
			v.Reason = ev.drop
			out = append(out, v)
			return
		}
		row, b := bindingRow(ev.rows, ev.volts, boundOf)
		outcome, w := check.CompareToBound(ev.volts, "V", b, "nominal", limitName)
		v := pinVerdict(rule, ev, outcome, w, row)
		if outcome == check.Fail {
			v.Finding = findingFor(ev, row, b)
		}
		out = append(out, v)
	})
	return out
}

// pinContext is the pin and the rail. These rules are per-PIN and their subject is the whole part,
// so without the pin a reader cannot tell which terminal of a many-pin part is over its own limit
// (agni issue 349).
func pinContext(ev pinEvent) []check.ContextSubject {
	return []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindPin, Ref: ev.component.RefDes, Pin: ev.designator}, Role: "pin"},
		{Entity: check.Entity{Kind: check.KindNet, Ref: ev.net}, Role: "rail"},
	}
}

// pinAbsMaxVerdicts decides every pin-bound supply terminal against its own absolute-maximum row.
func pinAbsMaxVerdicts(m check.Model) []check.Verdict {
	return pinLimitVerdicts(m, "pin-exceeds-abs-max",
		parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX, "absolute-maximum", "absolute maximum",
		func(r *parampb.Parameter) check.Bound { return check.Bound{Max: r.Value.Max} },
		func(ev pinEvent, row *parampb.Parameter, _ check.Bound) *check.Finding {
			return &check.Finding{Subject: check.Entity{Kind: check.KindComponent, Ref: ev.component.RefDes}, Message: fmt.Sprintf("pin %s (%s) on rail %q: nominal %gV exceeds that pin's absolute-maximum %s %gV — %s",
				ev.designator, ev.pinName, ev.net, ev.volts, row.Symbol, row.Value.GetMax(),
				check.Citation(ev.spec, row)), Prov: ev.component.Prov, DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(ev.spec, row)}, Context: pinContext(ev)}
		})
}

// pinRecommendedVerdicts decides every pin-bound supply terminal against its own recommended range.
func pinRecommendedVerdicts(m check.Model) []check.Verdict {
	return pinLimitVerdicts(m, "pin-out-of-recommended",
		parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING, "recommended-operating", "recommended range",
		func(r *parampb.Parameter) check.Bound { return check.Bound{Min: r.Value.Min, Max: r.Value.Max} },
		func(ev pinEvent, row *parampb.Parameter, b check.Bound) *check.Finding {
			// Which side was crossed, for the message only. The OUTCOME came from the comparison in
			// pinLimitVerdicts, so the two cannot disagree about whether this is a violation.
			rel := fmt.Sprintf("exceeds recommended maximum %gV", row.Value.GetMax())
			if b.Min != nil && ev.volts < *b.Min {
				rel = fmt.Sprintf("is below recommended minimum %gV", row.Value.GetMin())
			}
			return &check.Finding{Subject: check.Entity{Kind: check.KindComponent, Ref: ev.component.RefDes}, Message: fmt.Sprintf("pin %s (%s) on rail %q: nominal %gV %s for that pin (%s) — %s",
				ev.designator, ev.pinName, ev.net, ev.volts, rel, row.Symbol,
				check.Citation(ev.spec, row)), Prov: ev.component.Prov, DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(ev.spec, row)}, Context: pinContext(ev)}
		})
}

// pinExceedsAbsMax flags a supply pin sitting on a rail above THAT TERMINAL's absolute-maximum
// rating. The per-pin counterpart of supply-exceeds-abs-max, and the rule that removes its false
// positive on a part whose supplies differ.
var pinExceedsAbsMax = &check.Rule{
	Name:       "pin-exceeds-abs-max",
	Severity:   "error",
	Summary:    "A supply pin sits on a rail whose nominal voltage exceeds that pin's own absolute-maximum rating.",
	Impact:     "Exceeding an absolute-maximum rating is outside the vendor's stress envelope: the part may be damaged immediately or degrade in the field. Unlike the part-level check, this compares against the limit the datasheet states for THIS terminal, so a part whose supplies are rated differently is answered per supply rather than against its most restrictive one.",
	Remedy:     "Move this terminal to a rail inside its own rated maximum. A part's supplies are often rated differently from one another, so work from this pin's number rather than the part's.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_range", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail:              ruleDoc("pin-exceeds-abs-max"),
	Eval:                pinAbsMaxVerdicts,
	StatesConsideredSet: true,
}

// pinOutOfRecommended flags a supply pin sitting on a rail outside THAT TERMINAL's recommended
// operating range. The per-pin counterpart of rail-nominal-out-of-recommended, and the rule that
// lets a multi-supply part be range-checked at all. That rule declines one, because applying a
// two-sided range to the wrong terminal invents an over- or under-voltage.
var pinOutOfRecommended = &check.Rule{
	Name:       "pin-out-of-recommended",
	Severity:   "warning",
	Summary:    "A supply pin sits on a rail whose nominal voltage is outside that pin's own recommended operating range.",
	Impact:     "Outside the recommended operating range the datasheet's guaranteed specifications no longer hold: the part may still function, but margin, accuracy, and lifetime are no longer assured. Because the range is read per terminal, a part with several supplies at different windows is checked against the right one instead of being skipped as ambiguous.",
	Remedy:     "Bring this terminal's supply inside the range the datasheet gives for it. Where a part has several supplies, each carries its own window.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_range", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail:              ruleDoc("pin-out-of-recommended"),
	Eval:                pinRecommendedVerdicts,
	StatesConsideredSet: true,
}
