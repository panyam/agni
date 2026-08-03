package profiles

import (
	"strings"

	"github.com/panyam/agni/core/check"
)

// InUse reports whether interface p's signal convention is genuinely in use on the model — the SAME
// gate the compiled completeness rules apply (profile.go `in_use`): at least TWO DISTINCT signals of
// the profile appear as nets, matched by the full convention (suffix AND, when set, prefix). This is
// the review's absence gate (WS3-090): it must agree with the rules, or an interface the rules will
// not fire on gets scored as a clean pass. The prefix is load-bearing — a prefix-discriminated
// profile (PCIe signals prefixed `PCIE_`) must NOT read in-use just because foreign nets share a bare
// suffix (`LIN_TX`, `CAN_RX`); requiring the prefix is exactly what the rules do, so the gate and the
// rules never disagree. A lone matching signal is not evidence (a real corpus has many `_CS` nets),
// hence the two-distinct-signal floor.
func InUse(m check.Model, p Profile) bool {
	distinct := 0
	for _, s := range p.Signals {
		for _, n := range m.Nets() {
			if netMatchesSignal(n.GetName(), s) {
				distinct++
				break
			}
		}
		if distinct >= 2 {
			return true
		}
	}
	return false
}

// Named reports WEAK evidence that interface p is on the board: at least two of its signals appear as
// nets matched by SUFFIX ALONE — ignoring prefix and host. It is deliberately looser than InUse, and
// exists only to separate a host-bound interface that IS on the board but whose host/convention cannot
// resolve (annotated on no component and not strictly in use -> not-automated, the intended check is
// blocked) from one that is simply absent (-> not-applicable). WS3-090. Do NOT use it as the run gate;
// that is InUse/HostDeclared, which agree with the rules.
func Named(m check.Model, p Profile) bool {
	found := 0
	for _, s := range p.Signals {
		if anyNetHasSuffix(m, s.Suffix) {
			if found++; found >= 2 {
				return true
			}
		}
	}
	return false
}

func anyNetHasSuffix(m check.Model, suffix string) bool {
	for _, n := range m.Nets() {
		if strings.HasSuffix(n.GetName(), suffix) {
			return true
		}
	}
	return false
}

// HostDeclared reports whether a component on the model DECLARES this profile's host interface
// (its HostAttrKey=HostAttrVal attribute, WS3-042). False for a profile with no host binding. The
// review gate distinguishes a host-bound profile whose host is declared (the host path can evaluate)
// from one whose host is annotated nowhere (the host path cannot evaluate, so the item reads
// not-automated rather than a hollow pass — WS3-090).
func HostDeclared(m check.Model, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	for _, c := range m.Components() {
		if c.GetAttributes()[p.HostAttrKey] == p.HostAttrVal {
			return true
		}
	}
	return false
}

// netMatchesSignal is the Go twin of the datalog netMatch: a net satisfies a signal when its name
// carries the signal's suffix AND, when the signal sets one, its prefix. Keeping this in lockstep
// with netMatch is what makes InUse agree with the rules' in_use gate.
func netMatchesSignal(name string, s Signal) bool {
	if !strings.HasSuffix(name, s.Suffix) {
		return false
	}
	if s.Prefix != "" && !strings.HasPrefix(name, s.Prefix) {
		return false
	}
	return true
}
