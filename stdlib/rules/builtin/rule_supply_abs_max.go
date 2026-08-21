package builtin

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
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
	Remedy:     "Feed the part from a rail inside its rated supply range, or change the part for one rated to the rail it sits on. An absolute maximum is a damage threshold rather than a tolerance to design against.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.supply_abs_max", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("supply-exceeds-abs-max"),
	Eval: func(m check.Model) []check.Verdict {
		return aliasSupplyVerdicts(m, check.SupplyAbsMaxLimits, mostRestrictiveMax,
			"absolute-maximum supply", "absolute maximum",
			func(p *parampb.Parameter) check.Bound { return check.Bound{Max: p.Value.Max} },
			func(ev aliasSupplyEvent, binding *parampb.Parameter, _ check.Bound) *check.Finding {
				return &check.Finding{
					Kind:    check.KindComponent,
					Subject: ev.comp.RefDes,
					Message: fmt.Sprintf("power-input pin %s on rail %q: nominal %gV exceeds absolute-maximum %s %gV — %s",
						ev.pin, ev.net, ev.nominal, binding.Symbol, binding.Value.GetMax(), check.Citation(ev.spec, binding)),
					Prov:          ev.comp.Prov,
					DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(ev.spec, binding)},
					// The pin and the rail, in the order the message names them. The PIN matters as
					// much as the net here: the subject is the whole part, and a part can have several
					// supply pins, so highlighting the part alone cannot say which one is over its
					// limit (agni issue 349). No NetID: this rule has the rail by name only.
					Context: aliasSupplyContext(ev),
				}
			})
	},
	StatesConsideredSet: true,
}

// aliasSupplyEvent is one supply terminal of one alias-path part: what the shared body needs to
// phrase a verdict about it. "Alias path" is the join that reaches a rating through the vendor symbol
// table rather than through pin bindings, which is what makes these two rules part-scoped where
// pin-exceeds-abs-max is terminal-scoped.
type aliasSupplyEvent struct {
	comp    *ir.Component
	pin     string
	net     string
	nominal float64
	spec    *parampb.PartSpec
}

// aliasSupplyContext is the pin and the rail. These rules' subject is the whole part, and a part with
// several supply pins is not located by its ref des alone (agni issue 349).
func aliasSupplyContext(ev aliasSupplyEvent) []check.ContextSubject {
	return []check.ContextSubject{
		{Kind: check.KindPin, Subject: ev.comp.RefDes, Pin: ev.pin, Role: "pin"},
		{Kind: check.KindNet, Subject: ev.net, Role: "rail"},
	}
}

// mostRestrictiveMax picks the binding row for the one-sided ceiling: the lowest stated maximum.
// Applying the tightest row across a part's supply pins is conservative, which is what makes the
// alias path safe for a ceiling and unsafe for a two-sided range.
func mostRestrictiveMax(rows []*parampb.Parameter) (*parampb.Parameter, string) {
	binding := rows[0]
	for _, p := range rows[1:] {
		if p.Value.GetMax() < binding.Value.GetMax() {
			binding = p
		}
	}
	return binding, ""
}

// aliasSupplyVerdicts is the body both alias-path datasheet rules share: walk every supply terminal
// of every seeded part the per-pin rules do not own, and decide it against the part-level row.
//
// THE SUBJECT IS THE TERMINAL, the Finding stays part-scoped, and those are different granularities
// on purpose. The question "why is this supply pin fine" is asked of a pin; the sentence a reviewer
// reads names the part, because that is what they change and what the viewer highlights. It is the
// same split pin-exceeds-abs-max already draws, and it is why a part with three VDD pins on one rail
// produces three verdicts and the one finding it always produced. The extra terminals fail on the
// same evidence and carry no Finding, since the contract reports one sentence per (part, rail) rather
// than one per pin that shares the rail.
//
// WHAT USED TO BE SILENT AND NOW IS NOT. A part whose datasheet binds no comparable row of this kind,
// a pin on no net, and a rail whose name states no voltage all took the same `continue` a passing pin
// took. Those are NotConsidered with the step that stopped them, matching the vocabulary the per-pin
// rules already use. A row that states no bound at all is NoLimit, which check.CompareToBound returns
// without the rule having to remember to ask.
func aliasSupplyVerdicts(
	m check.Model,
	limitsOf func(*parampb.PartSpec) []*parampb.Parameter,
	bindingOf func([]*parampb.Parameter) (*parampb.Parameter, string), // row, or a reason it cannot pick one
	kindName string, // names the row kind in a drop reason: "absolute-maximum supply"
	limitName string, // names the bound in a witness: "absolute maximum"
	boundOf func(*parampb.Parameter) check.Bound,
	findingFor func(aliasSupplyEvent, *parampb.Parameter, check.Bound) *check.Finding,
) []check.Verdict {
	var out []check.Verdict
	for _, c := range m.Components() {
		spec := m.PartSpec(c.RefDes)
		if spec == nil {
			continue // no seeded datasheet: there is no rating to compare against, so not a subject
		}
		// A spec with pin bindings belongs to the per-pin rules, which answer it per terminal. They
		// emit their own verdicts over the same pins, so deferring here is not silence.
		if pinBoundSpec(m, c.RefDes) != nil {
			continue
		}
		binding, cannot := pickBinding(limitsOf(spec), bindingOf, kindName)

		seen := map[string]bool{} // rails this part has already filed a finding about
		for _, pin := range m.Pins() {
			if pin.Component.RefDes != c.RefDes || !check.SupplyInputPin(m, c.RefDes, pin.Designator) {
				continue // not a supply terminal: not a subject of these rules
			}
			ev := aliasSupplyEvent{comp: c, pin: pin.Designator, spec: spec}
			v := check.Verdict{Kind: check.KindPin, Subject: c.RefDes, Pin: pin.Designator}
			if cannot != "" {
				v.Outcome, v.Reason = check.NotConsidered, cannot
				out = append(out, v)
				continue
			}
			ev.net = m.PinNetName(c.RefDes, pin.Designator)
			if ev.net == "" {
				v.Outcome, v.Reason = check.NotConsidered, "pin is not connected to a net"
				out = append(out, v)
				continue
			}
			nominal, ok := check.NominalVoltageFromName(ev.net)
			if !ok {
				v.Outcome = check.NotConsidered
				v.Reason = fmt.Sprintf("rail %q does not name a nominal voltage", ev.net)
				out = append(out, v)
				continue
			}
			ev.nominal = nominal
			v.Context = aliasSupplyContext(ev)

			b := boundOf(binding)
			outcome, w := check.CompareToBound(nominal, "V", b, "nominal", limitName)
			if w != nil {
				w.Datasheet = []*check.DatasheetCitation{check.DatasheetCitationOf(spec, binding)}
			}
			v.Outcome, v.Witness = outcome, w
			if outcome == check.Fail && !seen[ev.net] {
				seen[ev.net] = true
				v.Finding = findingFor(ev, binding, b)
			}
			out = append(out, v)
		}
	}
	return out
}

// pickBinding reduces a part's rows of one kind to the row that governs, or returns the reason no row
// can be chosen. Both refusals are stated rather than swallowed: "the datasheet has no row of this
// kind" and "it has several and nothing says which pin each belongs to" are different answers, and
// the second is the documented restriction that keeps rail-nominal-out-of-recommended off multi-supply
// parts rather than inventing an over- or under-voltage on one.
func pickBinding(rows []*parampb.Parameter, bindingOf func([]*parampb.Parameter) (*parampb.Parameter, string), kindName string) (*parampb.Parameter, string) {
	if len(rows) == 0 {
		return nil, "the datasheet states no comparable " + kindName + " row for this part"
	}
	binding, cannot := bindingOf(rows)
	if cannot != "" {
		return nil, cannot
	}
	return binding, ""
}
