package profiles

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Nets returns the net names that belong to interface p on this design, the scope a review filters a
// broad rule's findings to so a per-interface ask (e.g. "CAN ESD") sees only that interface's nets.
// When the interface declares a host and the host is on the design, the scope is exactly the host's
// nets, which disambiguates suffixes shared across buses such as LIN's _TX/_RX. Otherwise it is the
// nets matched by the profile's signal matchers, the same ones the rules compile to, so a scoped item
// cannot pull in a net no finding can name.
//
// An empty result means present but unmatched, a clean pass. Absence is InUse's job (see present.go).
func Nets(m check.Model, p Profile) map[string]bool {
	nets, _ := scope(m, p)
	return nets
}

// scope walks the design once and returns both the net names of interface p and the RefDes on those
// nets. Nets and Components project it, so the host-beats-convention precedence lives in one place.
func scope(m check.Model, p Profile) (nets, comps map[string]bool) {
	nets, comps = map[string]bool{}, map[string]bool{}
	if p.HasHost() {
		hosts := map[string]bool{}
		for _, c := range m.Components() {
			if p.IsHost(m, c) {
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
