package intent

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Power-up sequencing as design intent (WS3-092): the third intent mechanism, after presence
// (module/subsystem/protection) and property (net_properties). Presence asks whether a thing is on the
// design and property asks what a net IS; neither can express an ORDER, and order is what a multi-rail
// board gets wrong in the way that damages parts.
//
// A netlist carries no order. It carries connectivity, and the only ordering an order leaves behind in
// connectivity is a GATING CHAIN: the earlier stage's power-good signal drives the later stage's
// enable, so the later stage physically cannot come up first.
//
// The other two ways a board enforces order leave no netlist trace at all. A PMIC steps its rails from
// its own NVM configuration, and firmware drives enables in turn after reading power-good. Both are
// real and common; neither is checkable here. The declaration is what separates them: an author can
// only declare the gating nets a chain actually uses, so a board that sequences internally or in
// firmware declares no sequence, compiles no rule, and its review items read needs-design-intent
// rather than passing on evidence nobody has. Parse enforces it by rejecting a sequence with no
// gating handle to check (see the load validation).

// sequenceRule builds the rule for ONE declared sequence. One rule per sequence, named
// intent/sequence-<slug>, following subsystemRule (WS3-058).
func sequenceRule(s Sequence) *check.Rule {
	return &check.Rule{
		Name:     "sequence-" + slug(s.Name),
		Severity: "error",
		Summary:  fmt.Sprintf("the %q power-up order the design intent declares is not enforced by the design", s.Name),
		Detail:   intentDoc(docKeySequence),
		Impact: "rails come up in an order the architecture does not allow. A part powered on its I/O rail " +
			"before its core rail back-drives current through its input clamp diodes, a peripheral released " +
			"from reset before its supply is good never enumerates, and a device latched up on a bad " +
			"sequence draws until something gives. None of it is visible in the schematic, and it surfaces " +
			"as an intermittent bring-up failure or a part that dies after a power cycle.",
		Remedy: intentRemedy(docKeySequence),
		Reads:  []string{"on_net", "component.class"},
		Tags:   intentTags(),
		// The two gating handles, in the order the assertion reads: the earlier stage's power-good, then
		// the later stage's enable. A gating link belongs to neither net alone, and the pair is what the
		// rule judges (agni issue 391's tuple).
		SubjectShape:        []string{check.KindNet, check.KindNet},
		Eval:                func(m check.Model) []check.Verdict { return evalSequence(m, s) },
		StatesConsideredSet: true,
	}
}

// evalSequence walks the declared order and judges each adjacent GATING PAIR: the earlier stage's
// power-good net against the later stage's enable net. A pair is judged only where the declaration
// supplies both handles, because those two names are the assertion. Everything else in the stage
// (the rail) is context.
//
// Three outcomes per pair, and there is deliberately no fourth:
//
//   - the design gates the pair the REVERSE way round -> finding (the sharpest diagnosis, so it is
//     tested before the two below and reported even when the declared handles are themselves absent)
//   - a declared handle net is not on the design      -> finding (the chain is not there to enforce anything)
//   - the handles are on the design but not linked    -> finding (nothing holds the later stage off)
//
// So a silent rule meant every declared link was FOUND, not that nothing was looked at, and that
// distinction is now stated rather than left to be inferred: a found link is a PASS naming the two
// handles it walked. An absent handle net fails here rather than being skipped the way an absent RAIL
// is: a rail is a presence question the voltage-domain and subsystem forms own, while the gating nets
// ARE this rule's subject.
//
// THE SUBJECT IS THE PAIR, and which pair depends on the branch. A link judged the declared way round
// is (earlier power-good, later enable); a link the design gates BACKWARDS is judged on the mirror
// handles, so that verdict names those instead. Both are 2-tuples of nets, which is what the declared
// SubjectShape says, and in each case the finding's own subject is one of the two.
func evalSequence(m check.Model, s Sequence) []check.Verdict {
	var out []check.Verdict
	fan := netFan(m)
	pair := func(x, y string) []check.Entity {
		return []check.Entity{check.NetNameEntity(x), check.NetNameEntity(y)}
	}
	for i := 0; i+1 < len(s.Order); i++ {
		a, b := s.Order[i], s.Order[i+1]
		if a.Good == "" || b.Enable == "" {
			continue // no gating handle declared for this link; Parse guarantees the sequence has one
		}
		goodNet, enNet := netNamed(m, a.Good), netNamed(m, b.Enable)
		if goodNet != nil && enNet != nil && linked(fan, goodNet, enNet) {
			out = append(out, check.Verdict{
				Subjects: pair(a.Good, b.Enable),
				Outcome:  check.Pass,
				Witness: &check.Witness{
					Statement: fmt.Sprintf("%q (the power-good of %s) reaches %q (the enable of %s), so %s is held off until %s is good",
						a.Good, a.Rail, b.Enable, b.Rail, b.Rail, a.Rail),
					Terms: []check.WitnessTerm{{Label: "power-good", Value: a.Good}, {Label: "enable", Value: b.Enable}},
				},
			})
			continue
		}
		// Looked for BEFORE the absent-handle case: an order declared the wrong way round usually
		// names the wrong handles too, and telling an author "your nets do not exist" would send them
		// looking for the wrong thing. It needs the mirror handles: the LATER stage's power-good and
		// the EARLIER stage's enable.
		if b.Good != "" && a.Enable != "" {
			if rGood, rEn := netNamed(m, b.Good), netNamed(m, a.Enable); linked(fan, rGood, rEn) {
				f := check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: a.Enable}, Prov: rEn.GetProv(), Message: fmt.Sprintf("sequence %q declares %s before %s, but the design gates it the other way round: %q (the power-good of %s) drives %q (the enable of %s)",
					s.Name, a.Rail, b.Rail, b.Good, b.Rail, a.Enable, a.Rail), // The power-good net doing the driving. Only this one: the branch has RESOLVED it
					// (rGood is non-nil or linked would not have matched), whereas the rail names come
					// straight from the intent declaration and are not known to exist on the design.
					// A chip that highlights nothing is worse than no chip (agni issue 349).
					Context: []check.ContextSubject{
						{Entity: check.Entity{Kind: check.KindNet, Ref: b.Good, NetID: rGood.GetId()}, Role: "power-good"},
					}}
				out = append(out, check.Verdict{
					// The MIRROR handles, because those are the pair this branch judged.
					Subjects: pair(b.Good, a.Enable),
					Outcome:  check.Fail,
					Witness:  &check.Witness{Statement: fmt.Sprintf("%q reaches %q, which is the declared order inverted", b.Good, a.Enable)},
					Context:  f.Context,
					Finding:  &f,
				})
				continue
			}
		}
		if goodNet == nil || enNet == nil {
			f := absentHandleFinding(s, a, b, goodNet, enNet)
			out = append(out, check.Verdict{
				Subjects: pair(a.Good, b.Enable),
				Outcome:  check.Fail,
				Witness:  &check.Witness{Statement: absentHandleWitness(a, b, goodNet, enNet)},
				Finding:  &f,
			})
			continue
		}
		f := check.Finding{
			Subject: check.Entity{Kind: check.KindNet, Ref: b.Enable},
			Prov:    enNet.GetProv(),
			Message: fmt.Sprintf("sequence %q declares %s before %s, but nothing connects %q (the power-good of %s) to %q (the enable of %s), so %s is free to come up first",
				s.Name, a.Rail, b.Rail, a.Good, a.Rail, b.Enable, b.Rail, b.Rail),
			// The power-good net that should have been connected. goodNet is non-nil on this path, so
			// unlike the rail names it is known to exist on the design.
			Context: []check.ContextSubject{
				{Entity: check.Entity{Kind: check.KindNet, Ref: a.Good, NetID: goodNet.GetId()}, Role: "power-good"},
			},
		}
		out = append(out, check.Verdict{
			Subjects: pair(a.Good, b.Enable),
			Outcome:  check.Fail,
			Witness: &check.Witness{
				Statement: fmt.Sprintf("both handles are on the design and nothing connects %q to %q", a.Good, b.Enable),
				Terms:     []check.WitnessTerm{{Label: "power-good", Value: a.Good}, {Label: "enable", Value: b.Enable}},
			},
			Context: f.Context,
			Finding: &f,
		})
	}
	return out
}

// absentHandleWitness says which of the two declared handles the design does not carry, which is the
// fact the finding's sentence rests on.
func absentHandleWitness(a, b SequenceStage, goodNet, enNet *ir.Net) string {
	switch {
	case goodNet == nil && enNet == nil:
		return fmt.Sprintf("the design carries neither %q nor %q", a.Good, b.Enable)
	case goodNet == nil:
		return fmt.Sprintf("the design carries no net named %q", a.Good)
	}
	return fmt.Sprintf("the design carries no net named %q", b.Enable)
}

// absentHandleFinding reports a declared gating net the design does not carry. Both handles are named
// in one finding when both are missing, so one broken link is one review line rather than two.
func absentHandleFinding(s Sequence, a, b SequenceStage, goodNet, enNet *ir.Net) check.Finding {
	var missing []string
	subject := b.Enable
	if goodNet == nil {
		missing = append(missing, fmt.Sprintf("%q (the power-good of %s)", a.Good, a.Rail))
		subject = a.Good
	}
	if enNet == nil {
		missing = append(missing, fmt.Sprintf("%q (the enable of %s)", b.Enable, b.Rail))
	}
	verb := "is"
	if len(missing) > 1 {
		verb = "are"
	}
	return check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: subject}, Message: fmt.Sprintf("sequence %q declares %s before %s through a power-good/enable chain, but %s %s not on the design, so no structure holds %s off until %s is good",
		s.Name, a.Rail, b.Rail, strings.Join(missing, " and "), verb, b.Rail, a.Rail)}
}

// gatingFanLimit bounds how many nets a component may touch and still count as a gating part.
//
// A gating chain often runs through a part rather than a wire: a single-gate buffer, a comparator, a
// load switch, a discrete FET. Those are small parts, and crediting them keeps a sequenced board off
// a false fail.
//
// A controller is not that. When a power-good lands on an MCU and the MCU drives the enable, the order
// lives in FIRMWARE, and firmware is not in the netlist. Crediting that path would let every board
// whose supervisory signals converge on one processor read as correctly sequenced. The netlist-visible
// fan-out separates the two without needing to know what any part is: a gate has a handful of nets, a
// controller has dozens.
const gatingFanLimit = 16

// linked reports whether the design carries a path from a power-good net to an enable net that could
// hold the later stage off: the two names are one net, or a single SMALL part touches both (see
// gatingFanLimit). One part is the whole radius, and it covers both wirings that occur: a resistor
// dropping an open-drain power-good to the enable pin's threshold, and an active gate or load switch
// between them. A series element IS a part sitting on both nets.
//
// A bounded series walk (check.Model.Reach) is deliberately not used: at any radius that stays honest
// it credits a strict SUBSET of what the one-part test credits.
//
// Where the evidence is ambiguous this errs toward crediting the link, because every FAIL has to be a
// genuine defect. The cost is a missed finding on a board whose chain runs through two parts in
// series.
func linked(fan map[string]int, from, to *ir.Net) bool {
	if from == nil || to == nil {
		return false
	}
	if from.GetName() == to.GetName() {
		return true
	}
	return sharesGatingPart(fan, from, to)
}

// sharesGatingPart reports whether one component small enough to be a gating part sits on both nets.
// Virtual connectivity symbols (a KiCad #PWR/#FLG) are not parts and cannot gate anything, so they
// never form a link.
func sharesGatingPart(fan map[string]int, from, to *ir.Net) bool {
	on := map[string]bool{}
	for _, c := range from.GetConnections() {
		if !check.IsVirtualRef(c.GetComponentRef()) {
			on[c.GetComponentRef()] = true
		}
	}
	for _, c := range to.GetConnections() {
		ref := c.GetComponentRef()
		if on[ref] && fan[ref] <= gatingFanLimit {
			return true
		}
	}
	return false
}

// netFan counts the distinct nets each component appears on, once per evaluation. It is the
// netlist-visible size of a part, which is what gatingFanLimit reads: a part-type pin count would say
// how big the package is, and this says how much of the design the part actually touches.
func netFan(m check.Model) map[string]int {
	fan := map[string]int{}
	for _, n := range m.Nets() {
		seen := map[string]bool{}
		for _, c := range n.GetConnections() {
			ref := c.GetComponentRef()
			if seen[ref] {
				continue // a part on a net twice is one net for this count
			}
			seen[ref] = true
			fan[ref]++
		}
	}
	return fan
}
