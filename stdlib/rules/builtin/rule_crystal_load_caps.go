package builtin

import (
	"fmt"
	"sort"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// crystalLoadCaps flags a passive crystal whose oscillator terminal has no load capacitor. See Detail.
var crystalLoadCaps = &check.Rule{
	Name:       "crystal-load-caps",
	Severity:   "warning",
	Summary:    "A passive crystal has an oscillator terminal with no load capacitor to ground.",
	Impact:     "A quartz crystal oscillates at its rated frequency only with the specified load capacitance on each terminal. Omit a load cap and the oscillator either will not start, starts intermittently over temperature, or runs off-frequency, which corrupts every timed peripheral downstream (UART baud, USB, CAN bit timing). The failure is analog and load-dependent, so it routinely passes bring-up and fails in the field.",
	Remedy:     "Fit a load capacitor from each oscillator terminal to ground, sized from the crystal's specified load capacitance and the stray capacitance of the layout rather than copied from another design.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("crystal-load-caps"),
	Eval:                crystalLoadCapsVerdicts,
	StatesConsideredSet: true,
}

// crystalTerminal is one oscillator terminal of one clock part: the net it sits on and the part's own
// pin on that net. Both are needed, and for different jobs. The NET is what carries the capacitor and
// what the finding's sentence names; the PIN is what makes the verdict addressable, since a crystal
// has two terminals and a verdict keyed by the part alone would be one identity for both.
type crystalTerminal struct {
	net *ir.Net
	pin string
}

// clockPart is one member of the clock family and the terminals the walk found on it.
type clockPart struct {
	comp    *ir.Component
	terms   map[string]crystalTerminal // non-ground, non-rail terminals, deduped by net name
	powered bool
}

// crystalLoadCapsVerdicts decides every oscillator TERMINAL of every passive crystal, one verdict
// each. The terminal is the subject because the terminal is the question: a crystal with a cap on one
// leg and none on the other is half right, and a part-level verdict could not say which half. The
// Finding stays part-scoped, matching what the viewer highlights and what every existing test asserts,
// the same split the per-pin datasheet rules already draw.
//
// A crystal's terminals and whether it carries a power pin are BOTH cross-net facts about one
// component, so the enumeration quantifies over components and then over nets, rather than over nets
// alone.
//
// THE TWO-TERMINAL GATE BECOMES NotConsidered, and it is the one exemption that deserved a voice. A
// clock part carrying a recognized rail IS an active oscillator, which takes no external load caps, so
// it is genuinely not a subject of this rule and gets no verdict. But the terminal COUNT is a
// heuristic standing in for the same question where the rail is not recognizable, and a real EDIF
// corpus supplied exactly that case: an oscillator whose Vcc net was neither flagged nor rail-named.
// A part failing that count is one the rule declined to classify, not one it cleared, and the two used
// to be the same silence.
func crystalLoadCapsVerdicts(m check.Model) []check.Verdict {
	parts := map[string]*clockPart{}
	var order []string
	for _, c := range m.Components() {
		// Quantify over the CLOCK FAMILY (WS10-015), excluding the subtypes that do not take external
		// load caps: an active oscillator has no external caps, a ceramic resonator has them integrated.
		// A bare clock candidate the classifier could not subtype (the common un-seeded case) stays in,
		// and the powered / exactly-two-terminal topology gates below remain the backstop that catches
		// an oscillator whose Vcc net is not rail-recognizable.
		if m.HasClass(c.RefDes, check.ClassClock) &&
			!m.HasClass(c.RefDes, check.ClassOscillator) && !m.HasClass(c.RefDes, check.ClassCeramicResonator) {
			parts[c.RefDes] = &clockPart{comp: c, terms: map[string]crystalTerminal{}}
			order = append(order, c.RefDes)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	for _, n := range m.Nets() {
		// A non-ground power rail on a crystal pin marks it an ACTIVE oscillator (it has a Vdd
		// pin); active oscillators do not use external load caps, so exclude them entirely.
		rail := m.IsPowerRail(n.Name) && !m.IsGroundNet(n)
		ground := m.IsGroundNet(n)
		for _, conn := range n.Connections {
			p := parts[conn.ComponentRef]
			if p == nil {
				continue
			}
			switch {
			case rail:
				p.powered = true
			case ground:
				// the grounded case/frame of a 3- or 4-pin crystal, not a signal terminal
			default:
				if _, seen := p.terms[n.Name]; !seen {
					p.terms[n.Name] = crystalTerminal{net: n, pin: conn.PinRef}
				}
			}
		}
	}

	var out []check.Verdict
	for _, ref := range order {
		p := parts[ref]
		if p.powered {
			continue // an active oscillator: it carries its own drive and takes no external load caps
		}
		// A passive two-terminal resonator has EXACTLY two non-ground signal terminals. An active
		// oscillator carries a Vcc (and often an enable/standby) pin, so it has more. Gating on the
		// count excludes active oscillators structurally, independent of whether the supply net reads
		// as a rail, which is the belt to the rail check's suspenders.
		if len(p.terms) != 2 {
			out = append(out, check.Verdict{
				Kind:    check.KindComponent,
				Subject: ref,
				Outcome: check.NotConsidered,
				Reason: fmt.Sprintf("the part has %d non-ground terminal(s) rather than the two a passive crystal has, "+
					"so the rule cannot tell it from an active oscillator that needs no load caps", len(p.terms)),
			})
			continue
		}
		names := make([]string, 0, len(p.terms))
		for name := range p.terms {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t := p.terms[name]
			v := check.Verdict{Kind: check.KindPin, Subject: ref, Pin: t.pin}
			v.Context = []check.ContextSubject{{Kind: check.KindNet, Subject: name, NetID: t.net.Id, Role: "terminal"}}
			loadCap := firstOnNet(m, t.net, check.ClassCapacitor)
			switch {
			case t.net.Attributes[netgraph.AttrExternal] == "true":
				// An unresolved external net may carry the cap on an unread sheet, so the rule has not
				// cleared this terminal; it could not see it.
				v.Outcome = check.NotConsidered
				v.Reason = "terminal net " + name + " continues onto a sheet this read did not open, so its load capacitor may be drawn outside it"
			case loadCap != "":
				v.Outcome = check.Pass
				v.Witness = &check.Witness{
					Statement: "capacitor " + loadCap + " sits on terminal net " + name,
					Terms:     []check.WitnessTerm{{Label: "load capacitor", Value: loadCap}},
				}
				v.Context = append(v.Context, check.ContextSubject{Kind: check.KindComponent, Subject: loadCap, Role: "load capacitor"})
			default:
				v.Outcome = check.Fail
				v.Witness = &check.Witness{
					Statement: "no capacitor sits on terminal net " + name,
				}
				v.Finding = &check.Finding{
					Kind:    check.KindComponent,
					Subject: ref,
					Message: "crystal terminal net " + name + " has no load capacitor",
					Prov:    p.comp.Prov,
					// The terminal the sentence is about. The subject is the crystal, because that
					// is the part a reader changes, but a crystal has two terminals and both sit
					// inside the highlighted symbol, so without this the drawing cannot say which
					// one is at fault (agni issue 349). Kept in step with the datalog twin.
					Context: []check.ContextSubject{{
						Kind: check.KindNet, Subject: name, NetID: t.net.Id, Role: "terminal",
					}},
				}
			}
			out = append(out, v)
		}
	}
	return out
}
