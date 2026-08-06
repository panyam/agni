package profiles

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Nets returns the set of net names that belong to interface p on this design — the scope a review can
// filter a broad rule's findings to, so a per-interface ask (e.g. "CAN ESD") reflects only that
// interface's nets, not the whole design. It mirrors the profile's own host-beats-convention
// precedence: when the interface declares a host and that host is on the design, the scope is exactly
// the host component's nets (precise, and it disambiguates suffixes shared across buses, e.g. LIN's
// _TX/_RX); otherwise it falls back to nets matched by the profile's signal matchers — the SAME
// matchers the rules compile to, so a scoped item cannot pull in a foreign net no finding can name.
//
// Presence is a SEPARATE concern (Present): an absent interface is marked not-applicable before any
// filtering, so an empty result here means "present but none of its nets matched", which reads as a
// clean pass for the scoped ask.
func Nets(m check.Model, p Profile) map[string]bool {
	nets, _ := scope(m, p)
	return nets
}

// scope walks the design ONCE and returns both the net names belonging to interface p and the
// component RefDes on those nets. Nets and Components are thin projections of it, so the
// host-beats-convention logic lives in one place and the design's nets are scanned a single time
// (Components previously called Nets and then re-scanned every net — one redundant linear pass).
func scope(m check.Model, p Profile) (nets, comps map[string]bool) {
	nets, comps = map[string]bool{}, map[string]bool{}
	if p.HasHost() {
		hosts := map[string]bool{}
		for _, c := range m.Components() {
			if c.GetAttributes()[p.HostAttrKey] == p.HostAttrVal {
				hosts[c.GetRefDes()] = true
			}
		}
		if len(hosts) > 0 {
			for _, n := range m.Nets() {
				onHost := false
				for _, conn := range n.GetConnections() {
					if hosts[conn.GetComponentRef()] {
						onHost = true
						break
					}
				}
				if onHost {
					collect(n, nets, comps)
				}
			}
			return nets, comps
		}
	}
	for _, n := range m.Nets() {
		if matchesAnySignal(p, n.GetName()) {
			collect(n, nets, comps)
		}
	}
	return nets, comps
}

// collect records net n's name and every component on it into the two scope sets.
func collect(n *ir.Net, nets, comps map[string]bool) {
	nets[n.GetName()] = true
	for _, conn := range n.GetConnections() {
		comps[conn.GetComponentRef()] = true
	}
}
