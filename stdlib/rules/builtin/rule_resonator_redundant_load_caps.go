package builtin

import (
	"sort"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	Primitives: []string{"select", "exists", "traverse", "pattern"},
	Reads:      []string{"component.class", "net.attributes", "net.names", "on_net"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("resonator-redundant-load-caps"),
	Eval: func(m check.Model) []check.Finding {
		// Collect ceramic resonators (the built-in-cap clock subtype). A bare un-subtyped clock
		// candidate is NOT one (it may be a crystal that genuinely needs caps), so this rule stays
		// silent until the datasheet class seeds ceramic_resonator.
		type reso struct {
			prov  *ir.Provenance
			terms map[string]*ir.Net // non-ground signal-terminal nets, deduped by name
		}
		resos := map[string]*reso{}
		var order []string
		for _, c := range m.Components() {
			if m.HasClass(c.RefDes, check.ClassCeramicResonator) {
				resos[c.RefDes] = &reso{prov: c.Prov, terms: map[string]*ir.Net{}}
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
		// Gather each resonator's non-ground, resolved signal terminals. A ground-named net is the
		// resonator's center/case pin, not a signal terminal; an unresolved external net may carry
		// its wiring on an unread sheet, so skip it (the crystal-load-caps external-skip precedent).
		for _, n := range m.Nets() {
			if m.IsGroundNet(n) || n.Attributes[netgraph.AttrExternal] == "true" {
				continue
			}
			for _, conn := range n.Connections {
				if r := resos[conn.ComponentRef]; r != nil {
					r.terms[n.Name] = n
				}
			}
		}
		var out []check.Finding
		for _, ref := range order {
			r := resos[ref]
			names := make([]string, 0, len(r.terms))
			for name := range r.terms {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				n := r.terms[name]
				// A redundant external load cap = a capacitor on this terminal net whose other leg
				// reaches ground. A coupling/series cap between two signals (not touching ground) is
				// not a load cap, so it is not flagged.
				var capRef string
				for _, conn := range n.Connections {
					if conn.ComponentRef != ref && m.HasClass(conn.ComponentRef, check.ClassCapacitor) && groundRef[conn.ComponentRef] {
						capRef = conn.ComponentRef
						break
					}
				}
				if capRef != "" {
					out = append(out, check.Finding{
						Kind:    check.KindComponent,
						Subject: ref,
						Message: "ceramic resonator terminal net " + name + " has an external load capacitor " + capRef + " (this part integrates its load caps)",
						Prov:    r.prov,
					})
				}
			}
		}
		return out
	},
}
