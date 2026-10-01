package check

import "strings"

// FactTier names a family of declared facts that a GATE keys on. Three gates read a rule's Reads to
// decide whether the rule can run at all, and each keys on a prefix rather than on individual fact
// names, so no gate distinguishes param.esd_rating from param.supply_abs_max. The tier is therefore
// the granularity at which a declaration matters, and the one the Reads audit checks (WS3-122).
type FactTier string

const (
	// TierParam is the datasheet parameter tier, injected per run via --params. Available reports a
	// rule reading it not-applicable when no set is seeded.
	TierParam FactTier = "param"
	// TierBoard is the board-geometry tier, per artifact. Available reports a rule reading it
	// not-applicable when the design carries no board.
	TierBoard FactTier = "board"
	// TierConnectivity is the pin/net-membership family. Run reports a rule reading it inconclusive
	// while any symbol is unresolved, because a placement that lost its pins is indistinguishable
	// from one that was never wired (WS1-052).
	TierConnectivity FactTier = "connectivity"
)

// paramTierRelations are relation names that carry datasheet-joined data without a "param" prefix
// (the prefix test misses them), but which are just as absent without a seeded set. They gate the
// same way. The names are literals rather than the relations package's consts because those are
// stdlib content now (issue 10) and relations imports check, so check cannot import it. A
// relation NAME is a stable contract string, so a literal is safe.
var paramTierRelations = map[string]bool{
	"component.device_class": true, // RelComponentDeviceClass (WS10-013)
	"component.esd_rated":    true, // RelEsdRated (WS3-076)
}

// connectivityFactPrefixes name the fact families that come from a component's PINS. A symbol that
// fails to resolve contributes no pins, so every fact in these families is missing for its
// placements.
var connectivityFactPrefixes = []string{"pin.", "on_net"}

// TierOf reports which gated tier a declared fact belongs to, or "" for a fact no gate keys on
// (net.names, component.class and the rest, which are always available once a design is read).
//
// This is the ONE definition of those prefix tests. Available, the unresolved-symbol gate and the
// Reads audit all call it, so the audit cannot drift from the tiers the gates actually apply.
func TierOf(fact string) FactTier {
	if strings.HasPrefix(fact, "param") || paramTierRelations[fact] {
		return TierParam
	}
	if strings.HasPrefix(fact, "board.") {
		return TierBoard
	}
	for _, p := range connectivityFactPrefixes {
		if strings.HasPrefix(fact, p) {
			return TierConnectivity
		}
	}
	return ""
}

// DeclaredTiers returns the gated tiers a rule's Reads declare, deduplicated.
//
// OptionalReads are INCLUDED, where Available skips them. Available asks whether an absent tier
// makes the rule inapplicable, and an optional read (esd-protection's ESD rating, used only to
// EXEMPT a finding) does not. This asks whether the author declared the tier at all, and an
// optional read is a declaration. Excluding them made the audit flag esd-protection's param tier as
// undeclared (WS3-122).
func DeclaredTiers(r *Rule) map[FactTier]bool {
	out := map[FactTier]bool{}
	for _, fact := range r.Reads {
		if t := TierOf(fact); t != "" {
			out[t] = true
		}
	}
	return out
}
