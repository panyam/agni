package builtin

import (
	"context"
	"fmt"
	"math"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The pin-tracking rules (agni issue 192) check a datasheet's constraint BETWEEN two pins of one part
// against what the design does with those two terminals.
//
// There are two rules, split by modality. A PinRelation carries a modality, because "shall never
// exceed" and "should be at least 1 V higher for best translator operation" are different claims,
// and reporting both at one severity misstates one of them. check.Run stamps Finding.Severity from
// the rule, so a rule states one severity for all its findings. pin-exceeds-abs-max and
// pin-out-of-recommended split one comparison by LimitKind in the same way. The split also lets a
// team fail CI on the required rule and only report the other.
//
// Connectivity is the first evidence tier and the net name is the second. Every other voltage rule
// in the catalog reads a rail's volts off its NAME (Model.NominalVoltage). That convention is
// silent on a rail nobody named for its voltage, and wrong on a name that outlived a design change.
// A relation bounds the DIFFERENCE between two terminals, and in one case connectivity settles that
// difference outright. Two pins on ONE net are one node, so the difference is exactly zero and no
// name is read. That tier decides in both directions. Tying the pins SATISFIES a `max 0` bound
// (VCCA <= VCCB), and it VIOLATES a `min 1` bound, such as an enable required to sit a volt above a
// reference. The rule compares two name-derived nominals only when the terminals sit on different
// nets.
//
// The name tier is gated on rails. net.nominal_voltage token-scans a whole net name, so a
// signalling level in a SIGNAL net's name parses as a rail nominal. In agni issue 194,
// `U3_12_U7_4_3V3` yields 3.3 while classifying as neither rail nor ground. Comparing a supply
// pin's rail against that value would give a confident wrong difference. So the name tier requires
// both nets to carry the rail role before it reads either name. Model.IsRailNet asks that narrow
// role question.

// pinRelationSpec returns the seeded spec for a component when it carries pins AND relations, else
// nil. Deliberately NOT pinBoundSpec, which also requires a pin-bound PARAMETER. A per-pin limit needs
// one and a relation does not, since a spec may state a tracking constraint between two pins and bind
// no limit row to either.
func pinRelationSpec(m check.Model, refDes string) *parampb.PartSpec {
	spec := m.PartSpec(refDes)
	if spec == nil || len(spec.GetPins()) == 0 || len(spec.GetRelations()) == 0 {
		return nil
	}
	return spec
}

// netByPin indexes every design terminal onto the net it sits on. A pin claimed by SEVERAL nets is
// mapped to nil rather than to one of them. That is malformed input (pin-net-conflict reports it),
// and picking a claimant would decide the same-net question by luck.
func netByPin(m check.Model) map[string]*ir.Net {
	idx := make(map[string]*ir.Net)
	for _, n := range m.Nets() {
		for _, c := range n.GetConnections() {
			key := c.GetComponentRef() + "\x00" + c.GetPinRef()
			if prev, seen := idx[key]; seen && prev != n {
				idx[key] = nil
				continue
			}
			idx[key] = n
		}
	}
	return idx
}

// designPinsBySpecPin maps this component's spec pins back onto its design terminals, by running
// param.ResolvePin over each design pin and inverting the result. It inverts the DESIGN->SPEC join
// instead of writing a second one, and so it keeps ResolvePin's precedence and its refusals. An
// ambiguous name or a name/number disagreement drops the terminal here too. A spec pin reached by
// two design terminals is dropped for the same reason.
func designPinsBySpecPin(m check.Model, refDes string, spec *parampb.PartSpec) map[string]string {
	pkg := ""
	if p := param.PackageForMPN(spec, m.ComponentMPN(refDes)); p != nil {
		pkg = p.GetId()
	}
	out := make(map[string]string)
	dup := make(map[string]bool)
	for _, pin := range m.Pins() {
		if pin.Component.RefDes != refDes {
			continue
		}
		specPin, err := param.ResolvePin(spec, m.PinName(refDes, pin.Designator), pin.Designator, pkg)
		if err != nil {
			continue
		}
		id := specPin.GetId()
		if _, seen := out[id]; seen {
			dup[id] = true
			continue
		}
		out[id] = pin.Designator
	}
	for id := range dup {
		delete(out, id)
	}
	return out
}

// pinTracking is one resolved comparison, holding the relation, the two terminals it binds, and the
// difference the design puts between them.
type pinTracking struct {
	component *ir.Component
	spec      *parampb.PartSpec
	rel       *parampb.PinRelation
	subjPin   *parampb.Pin
	refPin    *parampb.Pin
	subjDes   string
	refDes    string
	subjNet   *ir.Net
	refNet    *ir.Net
	// subjVolts and refVolts are what each rail's name states, read once by eachPinTracking.
	subjVolts, refVolts float64
	// bound is the relation's difference reduced to VOLTS, so the comparison and the message never
	// depend on which prefix the vendor printed. Never the relation's own field.
	bound *parampb.RangeValue
	// diff is subject minus reference, in volts. shared is true when both terminals sit on ONE net,
	// which fixes diff at exactly 0 from connectivity alone and needs no net name to have parsed.
	diff   float64
	shared bool
}

// differenceInVolts reduces a relation's bound to volts, and reports false for a bound printed in
// anything that does not reduce to volts. Both prefixes occur in real documents (one vendor states a
// tracking bound as "0.5 V", another as "100mV"), so rejecting everything but a bare "V" would
// silently drop the millivolt half of the evidence.
func differenceInVolts(rel *parampb.PinRelation) (*parampb.RangeValue, bool) {
	base, exp, ok := param.BaseUnit(rel.GetUnit())
	if !ok || base != "V" {
		return nil, false
	}
	scale := math.Pow(10, float64(exp))
	d := rel.GetDifference()
	out := &parampb.RangeValue{}
	if d.Min != nil {
		v := d.GetMin() * scale
		out.Min = &v
	}
	if d.Max != nil {
		v := d.GetMax() * scale
		out.Max = &v
	}
	return out, true
}

// eachPinTracking walks every tracking relation of the requested modality whose two terminals the
// design places, and yields the difference between them. It drops the relation at every step it
// cannot take safely. Those steps are an unresolvable terminal, a pin on no net, a non-rail net on
// the name tier, a name carrying no voltage, and a bound stated in something other than volts.
// Nothing here reports, so a drop is a skip and never a pass.
func eachPinTracking(m check.Model, wantModality func(parampb.Modality) bool, yield func(pinTracking)) {
	nets := netByPin(m)
	for _, c := range m.Components() {
		spec := pinRelationSpec(m, c.RefDes)
		if spec == nil {
			continue
		}
		terminals := designPinsBySpecPin(m, c.RefDes, spec)
		for _, rel := range spec.GetRelations() {
			if rel.GetKind() != parampb.PinRelationKind_PIN_RELATION_KIND_TRACKING ||
				!wantModality(rel.GetModality()) {
				continue
			}
			bound, ok := differenceInVolts(rel)
			if !ok {
				continue
			}
			subjPin, refPin := param.PinByID(spec, rel.GetSubjectPinRef()), param.PinByID(spec, rel.GetReferencePinRef())
			if subjPin == nil || refPin == nil {
				continue
			}
			subjDes, ok1 := terminals[subjPin.GetId()]
			refDes, ok2 := terminals[refPin.GetId()]
			if !ok1 || !ok2 {
				continue
			}
			subjNet, refNet := nets[c.RefDes+"\x00"+subjDes], nets[c.RefDes+"\x00"+refDes]
			if subjNet == nil || refNet == nil {
				continue
			}
			pt := pinTracking{
				component: c, spec: spec, rel: rel, bound: bound,
				subjPin: subjPin, refPin: refPin, subjDes: subjDes, refDes: refDes,
				subjNet: subjNet, refNet: refNet,
			}
			if subjNet == refNet {
				pt.shared = true // one node, so the difference is 0 by connectivity and no name is read
				yield(pt)
				continue
			}
			// The name tier. Both nets must be rails before either name is read (issue 194).
			if !m.IsRailNet(subjNet) || !m.IsRailNet(refNet) {
				continue
			}
			sv, ok := m.NominalVoltage(subjNet.GetName())
			if !ok {
				continue
			}
			rv, ok := m.NominalVoltage(refNet.GetName())
			if !ok {
				continue
			}
			pt.subjVolts, pt.refVolts = sv, rv
			pt.diff = roundVolts(sv - rv)
			yield(pt)
		}
	}
}

// roundVolts trims the representation error introduced by SUBTRACTING two decimal rail nominals.
// 3.3 - 1.8 is 1.4999999999999998 in binary floating point. That prints as noise in a finding, and
// it misjudges a bound of exactly 1.5. No other rule subtracts two voltages, since the per-pin
// limit rules only compare. Microvolt resolution is finer than any bound a datasheet states, so
// nothing real is rounded away.
func roundVolts(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// boundBreach reports how a difference breaks a relation's bound, and whether it does at all. An
// absent bound on a side is unbounded there, exactly as it is on a Parameter's value, so a one-sided
// requirement needs no sentinel.
func boundBreach(rv *parampb.RangeValue, diff float64) (string, bool) {
	if rv.Max != nil && diff > rv.GetMax() {
		return fmt.Sprintf("exceeds the permitted maximum of %gV", rv.GetMax()), true
	}
	if rv.Min != nil && diff < rv.GetMin() {
		return fmt.Sprintf("is below the required minimum of %gV", rv.GetMin()), true
	}
	return "", false
}

// requirementText renders a relation's bound as the sentence a reviewer reads, naming the two pins in
// subtraction order so the sign is unambiguous.
func requirementText(pt pinTracking) string {
	d := pt.bound
	subj, ref := pt.subjPin.GetName(), pt.refPin.GetName()
	switch {
	case d.Min != nil && d.Max != nil:
		return fmt.Sprintf("%s - %s must sit between %gV and %gV", subj, ref, d.GetMin(), d.GetMax())
	case d.Max != nil:
		return fmt.Sprintf("%s - %s must not exceed %gV", subj, ref, d.GetMax())
	default:
		return fmt.Sprintf("%s - %s must be at least %gV", subj, ref, d.GetMin())
	}
}

// evidenceText names WHERE the difference came from, because the two tiers do not carry the same
// weight and a reviewer should not have to guess which one spoke.
func evidenceText(pt pinTracking) string {
	if pt.shared {
		return fmt.Sprintf("both terminals are tied to net %q, so the difference is 0V", pt.subjNet.GetName())
	}
	sv, rv := pt.subjVolts, pt.refVolts
	return fmt.Sprintf("pin %s (%s) sits on rail %q at %gV and pin %s (%s) on rail %q at %gV, a difference of %gV",
		pt.subjDes, pt.subjPin.GetName(), pt.subjNet.GetName(), sv,
		pt.refDes, pt.refPin.GetName(), pt.refNet.GetName(), rv, pt.diff)
}

// trackingVerdicts is the body both rules share. It decides and reports every tracking relation of
// the requested modality whose two terminals the design places.
//
// THE SUBJECT IS THE PAIR OF PINS, because a tracking relation binds a pair. A part stating two
// tracking relations gives two answers about one ref-des, and a spec pin can be the subject of more
// than one of them. So neither the ref-des nor a single (ref-des, pin) key separates them. The
// order is the relation's own, subject pin then reference pin. The bound is on subject MINUS
// reference, so swapping the two inverts the sign of the claim.
//
// A pass carries a witness, such as "VCCA - VCCB is 0V, inside the required -0.3V to 0.3V, per
// SLLSEA9 p.6". A reviewer signing off a supply sequence wants that sentence.
//
// THE TWO CAVEATS are reported Inconclusive, never as a pass or a decline. A regime-scoped bound
// and an unstated modality are breaches the rule REACHED and could not rate. They still reach a
// reviewer through Finding.Inconclusive, and VerdictsToFindings projects that as well as Fail.
// Where the numbers are INSIDE the bound, those same two cases are an ordinary pass.
func trackingVerdicts(m check.Model, wantModality func(parampb.Modality) bool) []check.Verdict {
	var out []check.Verdict
	eachPinTracking(m, wantModality, func(pt pinTracking) {
		v := check.Verdict{Subjects: []check.Entity{
			check.PinEntity(pt.component.RefDes, pt.subjDes),
			check.PinEntity(pt.component.RefDes, pt.refDes),
		}}
		breach, bad := boundBreach(pt.bound, pt.diff)
		if !bad {
			v.Outcome = check.Pass
			v.Witness = &check.Witness{
				Statement: fmt.Sprintf("%s: %s, which is inside the bound", requirementText(pt), evidenceText(pt)),
				Terms: []check.WitnessTerm{
					{Label: "difference", Value: fmt.Sprintf("%gV", pt.diff)},
					{Label: "required", Value: requirementText(pt)},
				},
				Datasheet: []*check.DatasheetCitation{check.DatasheetCitationOfProv(pt.spec, pt.rel.GetProv())},
			}
			out = append(out, v)
			return
		}
		// The two caveats from the doc comment. Each names what the rule could not resolve, per the
		// Finding.Inconclusive contract.
		//
		// A regime-scoped bound means the rule cannot tell whether the regime the vendor named is the
		// one the design is in ("transient only, not for DC").
		//
		// An unstated modality gets here because param.Validate requires a relation's kind, bound and
		// provenance but NOT its modality, so a draft can carry a breach whose severity is unknown.
		// Reporting it as an error would invent a requirement and dropping it would pass in silence,
		// so the required rule takes it and says what is missing.
		var caveat string
		switch {
		case len(pt.rel.GetConditions()) > 0:
			caveat = fmt.Sprintf("but the datasheet scopes the bound to a regime this check cannot evaluate (%s); verify by hand",
				conditionText(pt.rel))
		case pt.rel.GetModality() == parampb.Modality_MODALITY_UNSPECIFIED:
			caveat = "but the spec does not record whether the datasheet requires or merely recommends this bound, so its severity is unknown; verify by hand"
		}
		msg := fmt.Sprintf("%s: %s, which %s — %s", requirementText(pt), evidenceText(pt), breach,
			check.RelationCitation(pt.spec, pt.rel))
		if caveat != "" {
			msg = fmt.Sprintf("%s: %s, which %s, %s — %s", requirementText(pt), evidenceText(pt),
				breach, caveat, check.RelationCitation(pt.spec, pt.rel))
		}
		inconclusive := caveat != ""
		v.Outcome = check.Fail
		if inconclusive {
			v.Outcome = check.Inconclusive
		}
		v.Witness = &check.Witness{
			Statement: msg,
			Terms: []check.WitnessTerm{
				{Label: "difference", Value: fmt.Sprintf("%gV", pt.diff)},
				{Label: "required", Value: requirementText(pt)},
			},
			Datasheet: []*check.DatasheetCitation{check.DatasheetCitationOfProv(pt.spec, pt.rel.GetProv())},
		}
		v.Finding = &check.Finding{
			Subject:       check.ComponentEntity(pt.component.RefDes),
			Inconclusive:  inconclusive,
			Message:       msg,
			Prov:          pt.component.Prov,
			DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOfProv(pt.spec, pt.rel.GetProv())},
		}
		out = append(out, v)
	})
	return out
}

// conditionText renders the regime a relation is scoped to, preferring the vendor's printed wording.
func conditionText(rel *parampb.PinRelation) string {
	for _, c := range rel.GetConditions() {
		if raw := c.GetRaw(); raw != "" {
			return raw
		}
		if sym := c.GetSymbol(); sym != "" {
			return sym
		}
	}
	return "unstated"
}

// pinTrackingViolated flags a design that breaks a tracking constraint the datasheet REQUIRES.
var pinTrackingViolated = &check.Rule{
	Name:       "pin-tracking-violated",
	Severity:   "error",
	Summary:    "Two pins of one part sit outside the tracking bound their datasheet requires between them.",
	Impact:     "The vendor states this bound as a requirement, and breaking it is outside the stress envelope the part is guaranteed in: a supply ordering violation can forward-bias an internal path and damage the die on every power cycle. Where both terminals share a net the verdict rests on connectivity alone rather than on a rail's name, so it holds on a design whose nets are not named for their voltages.",
	Remedy:     "Restore the ordering the datasheet requires between the two terminals, using sequencing, a clamp diode between them, or a shared rail. This is a stress violation, so it wants fixing before the board is powered again.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_relation", "net.role", "net.nominal_voltage", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("pin-tracking-violated"),
	// The two pins the relation binds, subject then reference. The order matters (see
	// trackingVerdicts).
	SubjectShape: []string{check.KindPin, check.KindPin},
	Eval: func(ctx context.Context, m check.Model) []check.Verdict {
		// This rule takes UNSPECIFIED, not the advisory rule, so an unstated modality cannot pass
		// in silence. trackingVerdicts reports it inconclusive rather than as an error.
		return trackingVerdicts(m, func(md parampb.Modality) bool {
			return md == parampb.Modality_MODALITY_REQUIRED || md == parampb.Modality_MODALITY_UNSPECIFIED
		})
	},
	StatesConsideredSet: true,
}

// pinTrackingAdvisory flags the same breach where the datasheet RECOMMENDS rather than requires.
var pinTrackingAdvisory = &check.Rule{
	Name:       "pin-tracking-advisory",
	Severity:   "warning",
	Summary:    "Two pins of one part sit outside a tracking bound their datasheet recommends between them.",
	Impact:     "The vendor states this bound with a recommending verb (\"should be at least 1 V higher for best operation\"), so breaking it is a loss of margin or of stated performance rather than a stress violation. It is reported separately from the required bound because a team that gates CI on datasheet violations wants the two answered differently, and folding them together would misstate one of them.",
	Remedy:     "Restore the recommended ordering between the two terminals where the design allows it, or record that the loss of margin is accepted. Unlike the required bound, this costs performance rather than the part.",
	Primitives: []string{"select", "traverse", "pin-role", "param-join"},
	Reads:      []string{"param.pin", "param.pin_relation", "net.role", "net.nominal_voltage", "net.name", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryDatasheet,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
		"evidence":            "datasheet",
	},
	Detail: ruleDoc("pin-tracking-advisory"),
	// The two pins the relation binds, subject then reference. The order matters (see
	// trackingVerdicts).
	SubjectShape: []string{check.KindPin, check.KindPin},
	Eval: func(ctx context.Context, m check.Model) []check.Verdict {
		return trackingVerdicts(m, func(md parampb.Modality) bool {
			return md == parampb.Modality_MODALITY_RECOMMENDED
		})
	},
	StatesConsideredSet: true,
}
