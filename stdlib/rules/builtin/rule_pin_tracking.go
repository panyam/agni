package builtin

import (
	"fmt"
	"math"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The pin-tracking rules (agni issue 192): a datasheet's constraint BETWEEN two pins of one part,
// checked against what the design does with those two terminals.
//
// WHY TWO RULES RATHER THAN ONE. A PinRelation carries a modality, because "shall never exceed" and
// "should be at least 1 V higher for best translator operation" are different claims and reporting
// both at one severity misstates one of them. check.Run stamps Finding.Severity from the rule, by
// design, so a rule states one severity for all its findings. The split is therefore by modality,
// exactly as pin-exceeds-abs-max and pin-out-of-recommended split one comparison by LimitKind. It
// also gives a team the selector it actually wants: fail CI on the required rule, report the other.
//
// THE TWO EVIDENCE TIERS, AND WHY CONNECTIVITY GOES FIRST. Every other voltage rule in the catalog
// reads a rail's volts off its NAME (check.NominalVoltageFromName), which is a convention: silent on
// a rail nobody named for its voltage, wrong on a name that outlived a design change. A relation
// bounds the DIFFERENCE between two terminals, and connectivity settles that difference outright in
// one case -- two pins on ONE net are one node, so the difference is exactly zero with no name
// involved. That tier is decisive in both directions: a `max 0` bound (VCCA <= VCCB) is SATISFIED by
// tying them, and a `min 1` bound (an enable required to sit a volt above a reference) is VIOLATED by
// tying them. Only when the terminals sit on different nets does the rule fall back to comparing two
// name-derived nominals.
//
// THE RAIL GATE ON THE NAME TIER. net.nominal_voltage token-scans a whole net name, so a signalling
// level encoded in a SIGNAL net's name parses as a rail nominal (agni issue 194: `U3_12_U7_4_3V3`
// yields 3.3 while classifying as neither rail nor ground). Comparing a supply pin's rail against
// that would produce a confident wrong difference, so the name tier requires both nets to carry the
// rail role (Model.IsRailNet, the narrow role question) before it reads either name.

// pinRelationSpec returns the seeded spec for a component when it carries pins AND relations, else
// nil. Deliberately NOT pinBoundSpec: that gate additionally requires a pin-bound PARAMETER, which is
// what a per-pin limit needs and what a relation does not -- a spec may state a tracking constraint
// between two pins and bind no limit row to either.
func pinRelationSpec(m check.Model, refDes string) *parampb.PartSpec {
	spec := m.PartSpec(refDes)
	if spec == nil || len(spec.GetPins()) == 0 || len(spec.GetRelations()) == 0 {
		return nil
	}
	return spec
}

// netByPin indexes every design terminal onto the net it sits on. A pin claimed by SEVERAL nets is
// mapped to nil rather than to one of them: that is malformed input (pin-net-conflict reports it),
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
// param.ResolvePin over each design pin and inverting the result. Inverting the DESIGN->SPEC join
// rather than writing a second one keeps ResolvePin's precedence and, more importantly, its
// refusals: an ambiguous name or a name/number disagreement drops the terminal here too. A spec pin
// reached by two design terminals is dropped for the same reason.
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

// pinTracking is one resolved comparison: the relation, the two terminals it binds, and the
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
	// bound is the relation's difference reduced to VOLTS, so the comparison and the message never
	// depend on which prefix the vendor printed. Never the relation's own field.
	bound *parampb.RangeValue
	// diff is subject minus reference, in volts. shared is true when both terminals sit on ONE net,
	// which fixes diff at exactly 0 from connectivity alone and needs no net name to have parsed.
	diff   float64
	shared bool
}

// differenceInVolts reduces a relation's bound to volts, and reports false for a bound printed in
// anything that does not reduce to volts. Both prefixes occur in real documents -- a tracking bound
// is stated as "0.5 V" by one vendor and "100mV" by another -- so rejecting everything but a bare
// "V" would silently drop the millivolt half of the evidence.
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
// design places, and yields the difference between them. Every step that cannot be taken safely
// drops the relation: an unresolvable terminal, a pin on no net, a non-rail net on the name tier, a
// name carrying no voltage, a bound stated in something other than volts. Nothing here reports, so a
// drop is a skip and never a pass.
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
				pt.shared = true // one node: the difference is 0 by connectivity, no name read
				yield(pt)
				continue
			}
			// The name tier. Both nets must be rails before either name is read (issue 194).
			if !m.IsRailNet(subjNet) || !m.IsRailNet(refNet) {
				continue
			}
			sv, ok := check.NominalVoltageFromName(subjNet.GetName())
			if !ok {
				continue
			}
			rv, ok := check.NominalVoltageFromName(refNet.GetName())
			if !ok {
				continue
			}
			pt.diff = roundVolts(sv - rv)
			yield(pt)
		}
	}
}

// roundVolts trims the representation error a SUBTRACTION of two decimal rail nominals introduces.
// 3.3 - 1.8 is 1.4999999999999998 in binary floating point, which both prints as noise in a finding
// and misjudges a bound of exactly 1.5. This is the one rule that subtracts two voltages, which is
// why the problem appears here and not in the per-pin limit rules that only compare. Microvolt
// resolution is finer than any bound a datasheet states, so nothing real is rounded away.
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
	sv, _ := check.NominalVoltageFromName(pt.subjNet.GetName())
	rv, _ := check.NominalVoltageFromName(pt.refNet.GetName())
	return fmt.Sprintf("pin %s (%s) sits on rail %q at %gV and pin %s (%s) on rail %q at %gV, a difference of %gV",
		pt.subjDes, pt.subjPin.GetName(), pt.subjNet.GetName(), sv,
		pt.refDes, pt.refPin.GetName(), pt.refNet.GetName(), rv, pt.diff)
}

// trackingFindings is the body both rules share: the comparison, the two inconclusive guards, and the
// finding. Only the severity and the doc differ between them, and those live on the Rule.
func trackingFindings(m check.Model, wantModality func(parampb.Modality) bool) []check.Finding {
	var out []check.Finding
	eachPinTracking(m, wantModality, func(pt pinTracking) {
		breach, bad := boundBreach(pt.bound, pt.diff)
		if !bad {
			return
		}
		// Two things can reach here breached and still not be reportable as a violation, and both
		// stay silent when the numbers are WITHIN the bound: there is nothing for a reviewer to look
		// at, and flagging every affected relation would convert a coverage gap into noise (the
		// Finding.Inconclusive contract asks that the rule NAME what it could not resolve).
		//
		// A regime-scoped bound: the rule cannot tell whether the regime the vendor named is the one
		// the design is in ("transient only, not for DC").
		//
		// An unstated modality: param.Validate requires a relation's kind, bound and provenance but
		// NOT its modality, so a draft can carry a breach whose severity is unknown. Reporting it as
		// an error would invent a requirement and dropping it would pass in silence, so the required
		// rule takes it and says what is missing.
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
		out = append(out, check.Finding{
			Kind:          check.KindComponent,
			Subject:       pt.component.RefDes,
			Inconclusive:  inconclusive,
			Message:       msg,
			Prov:          pt.component.Prov,
			DatasheetProv: []*check.DatasheetCitation{check.DatasheetCitationOfProv(pt.spec, pt.rel.GetProv())},
		})
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
	// A RELATION-SHAPED SUBJECT, which the verdict key has no grammar for (agni issue 391).
	//
	// Its subject is a PAIR of pins on one part, which is what a tracking relation binds. A part
	// stating two tracking relations is two findings about one ref-des, and a spec pin can be the
	// subject of more than one of them, so neither the ref-des nor the (ref-des, pin) key separates them.
	//
	// A verdict is named `<rule>:<kind>:<ref>` and every kind's ref names ONE entity, so keying these
	// verdicts by the subject alone would issue one id for several different answers. `checkspb.Subject`
	// has the same shape on the wire (kind, ref, pin, net_id, bus_id), and TestVerdictFieldCensus exists
	// to make adding to it a decision rather than a drift, so a ref that names two entities is a
	// wire-vocabulary change rather than something to slip in under a rule conversion. Reducing to one
	// verdict per subject instead would mean dropping findings this rule reports today.
	//
	// So it reports violations only, and RunVerdicts leaves it out rather than presenting its failure
	// list as coverage. It is one of five rules in the catalog with this shape: copper-clearance (a pair
	// of nets), regulator-output-exceeds-abs-max (a part and the part that feeds it),
	// fet-vdss-below-switched-rail (a part and one of the rails it touches), and the two pin-tracking
	// rules (a pair of pins on one part).
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		// UNSPECIFIED lands here rather than on the advisory rule so an unstated modality cannot
		// pass in silence; trackingFindings reports it inconclusive rather than as an error.
		return trackingFindings(m, func(md parampb.Modality) bool {
			return md == parampb.Modality_MODALITY_REQUIRED || md == parampb.Modality_MODALITY_UNSPECIFIED
		})
	}),
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
	// A RELATION-SHAPED SUBJECT, which the verdict key has no grammar for (agni issue 391).
	//
	// Its subject is a PAIR of pins on one part, which is what a tracking relation binds. A part
	// stating two tracking relations is two findings about one ref-des, and a spec pin can be the
	// subject of more than one of them, so neither the ref-des nor the (ref-des, pin) key separates them.
	//
	// A verdict is named `<rule>:<kind>:<ref>` and every kind's ref names ONE entity, so keying these
	// verdicts by the subject alone would issue one id for several different answers. `checkspb.Subject`
	// has the same shape on the wire (kind, ref, pin, net_id, bus_id), and TestVerdictFieldCensus exists
	// to make adding to it a decision rather than a drift, so a ref that names two entities is a
	// wire-vocabulary change rather than something to slip in under a rule conversion. Reducing to one
	// verdict per subject instead would mean dropping findings this rule reports today.
	//
	// So it reports violations only, and RunVerdicts leaves it out rather than presenting its failure
	// list as coverage. It is one of five rules in the catalog with this shape: copper-clearance (a pair
	// of nets), regulator-output-exceeds-abs-max (a part and the part that feeds it),
	// fet-vdss-below-switched-rail (a part and one of the rails it touches), and the two pin-tracking
	// rules (a pair of pins on one part).
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		return trackingFindings(m, func(md parampb.Modality) bool {
			return md == parampb.Modality_MODALITY_RECOMMENDED
		})
	}),
}
