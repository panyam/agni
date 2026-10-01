package profiles

import (
	"strings"

	"github.com/panyam/agni/core/check"
)

// InUse reports whether interface p's signal convention is in use on the model. It is the Go twin of
// the generated `in_use` relation (profile.go), true when at least TWO DISTINCT signals of the profile
// appear as nets, matched by each signal's full declared matcher (netMatchesSignal, the Go twin of
// the generated netMatch). This is the review's absence gate (WS3-090), and it must agree with the rules, or an
// interface the rules will not fire on gets scored as a clean pass. It is only HALF that agreement,
// because the convention completeness rule conjoins in_use with the anchor net, which is Anchored
// (WS3-099); the run gate is both. The whole matcher applies, so a prefix- or glob-discriminated
// profile (PCIe signals prefixed `PCIE_`) must NOT read in-use just because foreign nets share a bare
// suffix (`LIN_TX`, `CAN_RX`). A lone matching signal is not evidence (a real corpus has many `_CS`
// nets), hence the two-distinct-signal floor.
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

// Anchored reports whether profile p's CONVENTION completeness check can hang on this model, meaning
// its anchor signal matches some net. InUse alone does not answer that (WS3-099). InUse is the twin of
// the datalog `in_use` relation (two distinct signals), while signalMissingRule conjoins in_use with
// the ANCHOR net existing. So an interface can clear InUse through two NON-anchor signals while the
// anchor is absent, the completeness rule reports nothing, and zero findings score a clean pass on a
// bus nothing checked. The review gate needs both halves. They stay separate predicates rather than
// one widened InUse because the secondary rules (signal-dangling, missing-pullup) gate on InUse alone,
// and those DO evaluate without the anchor.
//
// A profile that declares NO anchor generates no convention completeness rule at all (signalMissingRule
// returns nil), so there is nothing for a missing anchor to block and it counts as vacuously anchored.
func Anchored(m check.Model, p Profile) bool {
	a := p.anchorSignal()
	if a == nil {
		return true
	}
	for _, n := range m.Nets() {
		if netMatchesSignal(n.GetName(), *a) {
			return true
		}
	}
	return false
}

// Named reports WEAK evidence that interface p is on the board. At least two of its signals appear as
// nets matched by SUFFIX ALONE, dropping the prefix that narrows an affix matcher and ignoring host.
// It is looser than InUse, and exists only to separate a host-bound interface that IS on the board but
// whose host/convention cannot resolve (annotated on no component and not strictly in use ->
// not-automated, the intended check is blocked) from one that is simply absent (-> not-applicable).
// WS3-090. Do NOT use it as the run gate; that is InUse/HostDeclared, which agree with the rules.
//
// A glob or regex signal has no suffix to drop, so there is no looser reading of it than the matcher
// itself and it falls back to netMatchesSignal. Without that fallback its empty suffix would match
// every net and report every design as "named".
func Named(m check.Model, p Profile) bool {
	found := 0
	for _, s := range p.Signals {
		if anyNetLooselyMatches(m, s) {
			if found++; found >= 2 {
				return true
			}
		}
	}
	return false
}

func anyNetLooselyMatches(m check.Model, s Signal) bool {
	for _, n := range m.Nets() {
		if s.Suffix != "" {
			if strings.HasSuffix(n.GetName(), s.Suffix) {
				return true
			}
			continue
		}
		if netMatchesSignal(n.GetName(), s) {
			return true
		}
	}
	return false
}

// HostDeclared reports whether a component on the model DECLARES this profile's host interface
// (its HostAttrKey=HostAttrVal attribute, WS3-042). False for a profile with no host binding. The
// review gate distinguishes a host-bound profile whose host is declared, so the host path can
// evaluate, from one whose host is annotated nowhere. The host path cannot evaluate that one, so the
// item reads not-automated rather than a hollow pass (WS3-090).
func HostDeclared(m check.Model, p Profile) bool {
	if !p.HasHost() {
		return false
	}
	for _, c := range m.Components() {
		if p.IsHost(m, c) {
			return true
		}
	}
	return false
}
