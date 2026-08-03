package check

import (
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// crystalLoadCaps flags a passive crystal whose oscillator terminal has no load capacitor. See Detail.
var crystalLoadCaps = &Rule{
	Name:       "crystal-load-caps",
	Severity:   "warning",
	Summary:    "A passive crystal has an oscillator terminal with no load capacitor to ground.",
	Impact:     "A quartz crystal oscillates at its rated frequency only with the specified load capacitance on each terminal. Omit a load cap and the oscillator either will not start, starts intermittently over temperature, or runs off-frequency, which corrupts every timed peripheral downstream (UART baud, USB, CAN bit timing). The failure is analog and load-dependent, so it routinely passes bring-up and fails in the field.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		KeyCategory:     CategoryConnectivity,
		KeyTier:         "R",
		KeyDistribution: DistOpen,
	},
	Detail: ruleDoc("crystal-load-caps"),
	Eval: func(m Model) []Finding {
		// A crystal's terminals and whether it carries a power pin are BOTH cross-net facts about
		// one component, so this rule quantifies over components, not nets: gather each crystal's
		// non-ground, non-rail terminal nets and note whether any of its nets is a power rail.
		type xtal struct {
			prov    *ir.Provenance
			terms   map[string]*ir.Net // non-ground, non-rail terminal nets, deduped by name
			powered bool
		}
		xtals := map[string]*xtal{}
		var order []string
		for _, c := range m.Components() {
			// Quantify over the CLOCK FAMILY (WS10-015), excluding the subtypes that do not take external
			// load caps: an active oscillator has no external caps, a ceramic resonator has them integrated.
			// A bare clock candidate the classifier could not subtype (the common un-seeded case) stays in,
			// and the powered / exactly-two-terminal topology gates below remain the backstop that catches
			// an oscillator whose Vcc net is not rail-recognizable.
			if m.HasClass(c.RefDes, ClassClock) &&
				!m.HasClass(c.RefDes, ClassOscillator) && !m.HasClass(c.RefDes, ClassCeramicResonator) {
				xtals[c.RefDes] = &xtal{prov: c.Prov, terms: map[string]*ir.Net{}}
				order = append(order, c.RefDes)
			}
		}
		if len(xtals) == 0 {
			return nil
		}
		for _, n := range m.Nets() {
			// A non-ground power rail on a crystal pin marks it an ACTIVE oscillator (it has a Vdd
			// pin); active oscillators do not use external load caps, so exclude them entirely.
			rail := m.IsPowerRail(n.Name) && !isGroundName(n.Name)
			ground := isGroundName(n.Name)
			for _, conn := range n.Connections {
				x := xtals[conn.ComponentRef]
				if x == nil {
					continue
				}
				switch {
				case rail:
					x.powered = true
				case ground:
					// the grounded case/frame of a 3- or 4-pin crystal, not a signal terminal
				default:
					x.terms[n.Name] = n
				}
			}
		}
		var out []Finding
		for _, ref := range order {
			x := xtals[ref]
			// A passive two-terminal resonator has EXACTLY two non-ground signal terminals. An
			// active oscillator carries a Vcc (and often an enable/standby) pin, so it has more —
			// and its supply net is not always name- or attribute-recognizable as a rail (real
			// EDIF corpus: an oscillator whose Vcc net was neither flagged nor rail-named). Gating
			// on exactly two terminals excludes active oscillators structurally, independent of
			// whether the supply net reads as a rail; the rail check above is the belt to this
			// suspenders for a 2-pin device sitting on a recognized rail.
			if x.powered || len(x.terms) != 2 {
				continue
			}
			names := make([]string, 0, len(x.terms))
			for name := range x.terms {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				n := x.terms[name]
				// An unresolved external net may carry the cap on an unread sheet; skip it rather
				// than fire on a read gap (the decoupling/bulk-cap external-skip precedent).
				if n.Attributes[netgraph.AttrExternal] == "true" {
					continue
				}
				hasCap := Exists(n.Connections, func(c *ir.Connection) bool {
					return m.HasClass(c.ComponentRef, ClassCapacitor)
				})
				if !hasCap {
					out = append(out, Finding{
						Kind:    KindComponent,
						Subject: ref,
						Message: "crystal terminal net " + n.Name + " has no load capacitor",
						Prov:    x.prov,
					})
				}
			}
		}
		return out
	},
}
