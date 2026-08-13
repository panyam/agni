package check

import (
	"slices"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The naming lexicon (RoleVocab and its build/active-vocab machinery) moved to package classify in
// WS3-072 so the ingestion pass can stamp net.role without importing check; check re-exports the names
// (see aliases.go). The is*Name helpers below stay here as the name-match FALLBACK the core uses when a
// net carries no stamped role (a hand-authored test IR that skipped the loader), and as the projection
// the spec-language FFIs (rail_name / ground_name / feedback_name) call on a bare name literal.

// IsFeedbackName reports whether a net name is a regulator feedback / sense node (a high-impedance
// divider tap that must not be probed): a rail-NAMED net like "VCC1.2_ETH_FB" reads as feedback, not a
// probe-able rail. Consults the active lexicon (WS3-069), now owned by classify.
func IsFeedbackName(name string) bool { return classify.ActiveRoleVocab().IsFeedback(name) }

// NetHasRole reports whether a net carries a naming role (rail / ground / feedback), trusting the
// stamped role fact (ir.Net.roles, filled at ingestion by classify.StampNetRoles, WS3-072) when the
// net has ANY stamped role — the set is then authoritative, so a role is present iff it is in the set
// — and falling back to nameMatch (the same lexicon the stamp uses) only when the set is empty, i.e.
// the net came from a path that skipped the loader (a hand-authored test IR). This is the naming
// counterpart of the Model reading device_classes with a re-derive fallback, and the seam where a
// future structural signal can make the stamp beat the name. IsPowerRail (the Model core) and the
// net-role relations (stdlib/relations) both read through it, so the trust rule has one home.
func NetHasRole(n *ir.Net, role string, nameMatch func(string) bool) bool {
	if roles := n.GetRoles(); len(roles) > 0 {
		return slices.ContainsFunc(roles, func(r *ir.NetRole) bool { return r.GetRole() == role })
	}
	return nameMatch(n.GetName())
}

// NetRoleSource reports the evidence that established a role on a net, and whether the net carries
// that role at all. It is the "how do we know" question NetHasRole deliberately does not answer,
// separated because almost every caller wants the boolean and would otherwise have to ignore a
// second return value.
//
// A net whose role came from the NAME fallback (no stamped set at all, e.g. a hand-authored test IR)
// reports ROLE_SOURCE_CONVENTION, because that is exactly what the fallback is: a naming convention
// read at the point of use rather than at ingestion.
func NetRoleSource(n *ir.Net, role string, nameMatch func(string) bool) (ir.RoleSource, bool) {
	for _, r := range n.GetRoles() {
		if r.GetRole() == role {
			return r.GetSource(), true
		}
	}
	if len(n.GetRoles()) == 0 && nameMatch(n.GetName()) {
		return ir.RoleSource_ROLE_SOURCE_CONVENTION, true
	}
	return ir.RoleSource_ROLE_SOURCE_UNSPECIFIED, false
}
