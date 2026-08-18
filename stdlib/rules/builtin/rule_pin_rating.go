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
// on a part whose terminals genuinely differ, wrong: a translator rated 4.6 V on one supply and
// 6.5 V on the other reports a 5 V rail on the 6.5 V terminal as a violation. And
// rail-nominal-out-of-recommended declines multi-supply parts outright, because a two-sided range
// applied to the wrong terminal produces a false over- or under-voltage. Its doc comment named
// per-pin supply mapping as the follow-up; this is it.
//
// THE ALIAS PATH STAYS. These act only on a part whose spec carries pin bindings, and the alias
// rules defer only on such a part, so exactly one rule speaks per part and a design read against a
// corpus seeded before pin binding behaves exactly as it did (CONSTRAINTS C9).
//
// WHY GO AND NOT DATALOG, now that the pin relations exist. Mapping a design pin onto a spec pin
// can REFUSE — an ambiguous name with no package identified, or a name and number that disagree —
// and a datalog join has no way to express refusal; it would silently drop or, worse, cross-join.
// param.ResolvePin owns that decision, so the rules call it and declare the relations they consume
// for the availability gate.

// pinBoundSpec returns the seeded spec for a component when it carries pin bindings, else nil. It
// is the switch between the two paths: nil means the alias rules own this part, non-nil means these
// do. Both sides read it, so the two cannot drift into overlapping or into a gap.
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
	// Pins declared but nothing bound to them: there is no per-pin limit to check, so the alias
	// path is still the better answer rather than silence.
	return nil
}

// pinLimit is one resolved comparison: a design pin, the spec pin it resolved to, and the limit row
// bound to that pin.
type pinLimit struct {
	component  *ir.Component
	designator string
	pinName    string
	net        string
	volts      float64
	spec       *parampb.PartSpec
	row        *parampb.Parameter
}

// eachPinLimit walks every power-input pin of every pin-bound part, resolves it onto a spec pin,
// and yields the rows of the requested limit kind bound to that terminal. Every step that cannot be
// made safely drops the pin: no resolution, no voltage evidence, no bound row of that kind. Nothing
// here reports, so a drop is a skip and never a pass.
func eachPinLimit(m check.Model, kind parampb.LimitKind, yield func(pinLimit)) {
	for _, c := range m.Components() {
		spec := pinBoundSpec(m, c.RefDes)
		if spec == nil {
			continue
		}
		// The design's MPN may carry the package suffix; when it does not, PackageForMPN returns nil
		// and ResolvePin falls back to requiring cross-package agreement rather than assuming a body.
		pkg := ""
		if p := param.PackageForMPN(spec, m.ComponentMPN(c.RefDes)); p != nil {
			pkg = p.GetId()
		}
		for _, pin := range m.Pins() {
			if pin.Component.RefDes != c.RefDes || !check.SupplyInputPin(m, c.RefDes, pin.Designator) {
				continue
			}
			specPin, err := param.ResolvePin(spec, m.PinName(c.RefDes, pin.Designator), pin.Designator, pkg)
			if err != nil {
				continue
			}
			net := m.PinNetName(c.RefDes, pin.Designator)
			if net == "" {
				continue
			}
			volts, ok := check.NominalVoltageFromName(net)
			if !ok {
				continue
			}
			for _, row := range param.PinParameters(spec, specPin.GetId()) {
				q, ok := param.InBaseUnit(row)
				if !ok || q.LimitKind != kind || q.Unit != "V" ||
					param.UnderSpecified(q) || !param.MachineComparable(q) {
					continue
				}
				yield(pinLimit{
					component: c, designator: pin.Designator, pinName: specPin.GetName(),
					net: net, volts: volts, spec: spec, row: q,
				})
			}
		}
	}
}

// pinExceedsAbsMax flags a supply pin sitting on a rail above THAT TERMINAL's absolute-maximum
// rating. The per-pin counterpart of supply-exceeds-abs-max, and the rule that removes its false
// positive on a part whose supplies differ.
var pinExceedsAbsMax = &check.Rule{
	Name:       "pin-exceeds-abs-max",
	Severity:   "error",
	Summary:    "A supply pin sits on a rail whose nominal voltage exceeds that pin's own absolute-maximum rating.",
	Impact:     "Exceeding an absolute-maximum rating is outside the vendor's stress envelope: the part may be damaged immediately or degrade in the field. Unlike the part-level check, this compares against the limit the datasheet states for THIS terminal, so a part whose supplies are rated differently is answered per supply rather than against its most restrictive one.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_range", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("pin-exceeds-abs-max"),
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		eachPinLimit(m, parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX, func(pl pinLimit) {
			if pl.row.Value.Max == nil || pl.volts <= pl.row.Value.GetMax() {
				return
			}
			out = append(out, check.Finding{
				Kind:    check.KindComponent,
				Subject: pl.component.RefDes,
				Message: fmt.Sprintf("pin %s (%s) on rail %q: nominal %gV exceeds that pin's absolute-maximum %s %gV — %s",
					pl.designator, pl.pinName, pl.net, pl.volts, pl.row.Symbol, pl.row.Value.GetMax(),
					check.Citation(pl.spec, pl.row)),
				Prov:          pl.component.Prov,
				DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(pl.spec, pl.row)},
				// The pin and the rail. This rule is per-PIN and its subject is the whole part, so
				// without the pin a reader cannot tell which terminal of a many-pin part is over its
				// own limit (agni issue 349).
				Context: []check.ContextSubject{
					{Kind: check.KindPin, Subject: pl.component.RefDes, Pin: pl.designator, Role: "pin"},
					{Kind: check.KindNet, Subject: pl.net, Role: "rail"},
				},
			})
		})
		return out
	},
}

// pinOutOfRecommended flags a supply pin sitting on a rail outside THAT TERMINAL's recommended
// operating range. The per-pin counterpart of rail-nominal-out-of-recommended, and the rule that
// lets a multi-supply part be range-checked at all: that rule declines one, because applying a
// two-sided range to the wrong terminal invents an over- or under-voltage.
var pinOutOfRecommended = &check.Rule{
	Name:       "pin-out-of-recommended",
	Severity:   "warning",
	Summary:    "A supply pin sits on a rail whose nominal voltage is outside that pin's own recommended operating range.",
	Impact:     "Outside the recommended operating range the datasheet's guaranteed specifications no longer hold: the part may still function, but margin, accuracy, and lifetime are no longer assured. Because the range is read per terminal, a part with several supplies at different windows is checked against the right one instead of being skipped as ambiguous.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_range", "pin.electrical_type", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("pin-out-of-recommended"),
	Eval: func(m check.Model) []check.Finding {
		var out []check.Finding
		eachPinLimit(m, parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING, func(pl pinLimit) {
			hasLo, hasHi := pl.row.Value.Min != nil, pl.row.Value.Max != nil
			lo, hi := pl.row.Value.GetMin(), pl.row.Value.GetMax()
			var rel string
			switch {
			case hasHi && pl.volts > hi:
				rel = fmt.Sprintf("exceeds recommended maximum %gV", hi)
			case hasLo && pl.volts < lo:
				rel = fmt.Sprintf("is below recommended minimum %gV", lo)
			default:
				return
			}
			out = append(out, check.Finding{
				Kind:    check.KindComponent,
				Subject: pl.component.RefDes,
				Message: fmt.Sprintf("pin %s (%s) on rail %q: nominal %gV %s for that pin (%s) — %s",
					pl.designator, pl.pinName, pl.net, pl.volts, rel, pl.row.Symbol,
					check.Citation(pl.spec, pl.row)),
				Prov:          pl.component.Prov,
				DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOf(pl.spec, pl.row)},
				// As above: the recommended-range twin of the same finding.
				Context: []check.ContextSubject{
					{Kind: check.KindPin, Subject: pl.component.RefDes, Pin: pl.designator, Role: "pin"},
					{Kind: check.KindNet, Subject: pl.net, Role: "rail"},
				},
			})
		})
		return out
	},
}
