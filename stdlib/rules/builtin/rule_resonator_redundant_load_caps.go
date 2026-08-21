package builtin

import (
	"sort"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/internal/netgraph"
)

// resonatorRedundantLoadCaps flags a ceramic resonator (which integrates its load capacitors) that
// ALSO carries an external load capacitor to ground on an oscillator terminal. See Detail.
//
// This is the inverse of crystal-load-caps: a passive crystal NEEDS external load caps, so their
// absence is the defect; a ceramic resonator of the built-in-cap family (Murata CERALOCK and kin)
// already contains them, so an external load cap to ground is the "double load" mistake. The
// ceramic_resonator class is datasheet-seeded (WS10-015), so this rule is silent until a resonator
// is classified — never firing on a crystal or an un-subtyped clock candidate.
var resonatorRedundantLoadCaps = &check.Rule{
	Name:       "resonator-redundant-load-caps",
	Severity:   "warning",
	Summary:    "A ceramic resonator with integrated load capacitors also has an external load cap to ground on a terminal.",
	Impact:     "A ceramic resonator of the built-in-cap family already presents its specified load capacitance internally. Adding external load caps to ground doubles the load: the total capacitance is now well above spec, so the oscillator starts slowly or off-frequency, or (over temperature and part spread) fails to start. The board usually oscillates on the bench and drifts or drops out in the field, and every timed peripheral clocked from it (UART baud, CAN bit timing) goes with it. It is a silent BOM/layout carry-over from a crystal design, invisible until the load is measured.",
	Remedy:     "Remove the external load capacitors. A resonator of the built-in-cap family carries its own, so the schematic should show the resonator alone.",
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("resonator-redundant-load-caps"),
	Eval:                resonatorRedundantLoadCapsVerdicts,
	StatesConsideredSet: true,
}

// resonatorRedundantLoadCapsVerdicts decides every signal terminal of every ceramic resonator, one
// verdict each, the same terminal-scoped subject crystal-load-caps uses and for the same reason: a
// resonator with a stray cap on one leg is not the same design as one with caps on both, and a
// part-level verdict cannot say which leg.
//
// SCOPE IS DATASHEET-GATED, and that is why an un-subtyped clock part gets no verdict here. The
// ceramic_resonator class is seeded (WS10-015), so until a datasheet says a part integrates its load
// caps, an external cap on its terminal may be exactly what the part needs. Declining by silence is
// right here because the part is not a subject: crystal-load-caps owns the un-subtyped case and asks
// the opposite question of it.
//
// THE EXTERNAL TERMINAL BECOMES NotConsidered rather than vanishing during the walk. The old
// enumeration dropped an external net before the terminal set was built, so such a terminal was not
// merely uncleared, it was invisible: nothing downstream could tell it from a resonator that has no
// such terminal at all.
func resonatorRedundantLoadCapsVerdicts(m check.Model) []check.Verdict {
	// Collect ceramic resonators (the built-in-cap clock subtype). A bare un-subtyped clock
	// candidate is NOT one (it may be a crystal that genuinely needs caps), so this rule stays
	// silent until the datasheet class seeds ceramic_resonator.
	resos := map[string]*clockPart{}
	var order []string
	for _, c := range m.Components() {
		if m.HasClass(c.RefDes, check.ClassCeramicResonator) {
			resos[c.RefDes] = &clockPart{comp: c, terms: map[string]crystalTerminal{}}
			order = append(order, c.RefDes)
		}
	}
	if len(resos) == 0 {
		return nil
	}
	// A component touches ground if any of its pins sits on a ground net: this marks the far leg
	// of a load capacitor (terminal -> cap -> ground), the shape we are looking for.
	groundRef := map[string]bool{}
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			for _, conn := range n.Connections {
				groundRef[conn.ComponentRef] = true
			}
		}
	}
	// Gather each resonator's non-ground signal terminals. A ground-named net is the resonator's
	// center/case pin, not a signal terminal, so it is out of scope and yields nothing.
	for _, n := range m.Nets() {
		if m.IsGroundNet(n) {
			continue
		}
		for _, conn := range n.Connections {
			if r := resos[conn.ComponentRef]; r != nil {
				if _, seen := r.terms[n.Name]; !seen {
					r.terms[n.Name] = crystalTerminal{net: n, pin: conn.PinRef}
				}
			}
		}
	}

	var out []check.Verdict
	for _, ref := range order {
		r := resos[ref]
		names := make([]string, 0, len(r.terms))
		for name := range r.terms {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t := r.terms[name]
			v := check.Verdict{Kind: check.KindPin, Subject: ref, Pin: t.pin}
			v.Context = []check.ContextSubject{{Kind: check.KindNet, Subject: name, NetID: t.net.Id, Role: "terminal"}}
			if t.net.Attributes[netgraph.AttrExternal] == "true" {
				v.Outcome = check.NotConsidered
				v.Reason = "terminal net " + name + " continues onto a sheet this read did not open, so a load capacitor may be drawn outside it"
				out = append(out, v)
				continue
			}
			// A redundant external load cap = a capacitor on this terminal net whose other leg
			// reaches ground. A coupling/series cap between two signals (not touching ground) is
			// not a load cap, so it is not flagged.
			var capRef string
			for _, conn := range t.net.Connections {
				if conn.ComponentRef != ref && m.HasClass(conn.ComponentRef, check.ClassCapacitor) && groundRef[conn.ComponentRef] {
					capRef = conn.ComponentRef
					break
				}
			}
			if capRef == "" {
				v.Outcome = check.Pass
				v.Witness = &check.Witness{
					Statement: "no capacitor returns from terminal net " + name + " to ground, so the resonator's integrated load stands alone",
				}
				out = append(out, v)
				continue
			}
			v.Outcome = check.Fail
			v.Witness = &check.Witness{
				Statement: "capacitor " + capRef + " sits on terminal net " + name + " with its other leg on ground, adding to the load this part already integrates",
				Terms:     []check.WitnessTerm{{Label: "external load capacitor", Value: capRef}},
			}
			v.Context = append(v.Context, check.ContextSubject{Kind: check.KindComponent, Subject: capRef, Role: "capacitor"})
			v.Finding = &check.Finding{
				Kind:    check.KindComponent,
				Subject: ref,
				Message: "ceramic resonator terminal net " + name + " has an external load capacitor " + capRef + " (this part integrates its load caps)",
				Prov:    r.comp.Prov,
				// The terminal and the cap, in the order the sentence names them. The subject
				// stays the resonator, because that is the part whose datasheet makes the caps
				// redundant, but the cap is what a reader deletes (agni issue 349).
				Context: []check.ContextSubject{
					{Kind: check.KindNet, Subject: name, NetID: t.net.Id, Role: "terminal"},
					{Kind: check.KindComponent, Subject: capRef, Role: "capacitor"},
				},
			}
			out = append(out, v)
		}
	}
	return out
}
