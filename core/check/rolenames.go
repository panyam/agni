package check

import (
	"slices"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The naming lexicon (RoleVocab) lives in package classify so the ingestion pass can stamp net.role
// without importing check (WS3-072); aliases.go re-exports it. The is*Name helpers below match against
// the process-wide active lexicon. A Model's own Is*Name methods (query.go) match against the design's
// lexicon instead, and those are what NetHasRole's callers and the spec FFIs (rail_name, feedback_name)
// pass.

// IsFeedbackName reports whether a net name is a regulator feedback or sense node, a high-impedance
// divider tap that must not be probed. A rail-NAMED net like "VCC1.2_ETH_FB" reads as feedback rather
// than as a rail. Consults the active lexicon (WS3-069).
func IsFeedbackName(name string) bool { return classify.ActiveRoleVocab().IsFeedback(name) }

// IsSwitchingName reports whether a net name is a regulator's power-stage node (the switch node, its
// bootstrap cap, or the same node under a vendor spelling). "12V_SW" is named after the rail it
// produces and is not one. Read together with IsFeedbackName wherever a rule asks whether a rail-named
// net is really a rail.
func IsSwitchingName(name string) bool { return classify.ActiveRoleVocab().IsSwitching(name) }

// IsControlName reports whether a net name is a regulator's enable or mode-select input. "20V_EN"
// enables the 20V converter and sits at the sequencer's logic level, not at 20V. These are safe to
// probe, unlike a feedback tap or a switch node, and are not rails because the voltage token names the
// converter rather than the net.
func IsControlName(name string) bool { return classify.ActiveRoleVocab().IsControl(name) }

// IsGateDriveName reports whether a net name is the supply a regulator's gate driver runs from
// ("12V_VDRV"). It is a supply, hence separate from IsControlName, but it sits at the driver's own
// rail rather than at the converter's output.
func IsGateDriveName(name string) bool { return classify.ActiveRoleVocab().IsGateDrive(name) }

// NetHasRole reports whether a net carries a naming role (rail, ground, feedback, ...). When the net
// has ANY stamped role (ir.Net.roles, filled at ingestion by classify.StampNetRoles, WS3-072) the set
// is authoritative and a role is present iff it is in the set. Only an empty set, from a path that
// skipped the loader such as a hand-authored test IR, falls back to nameMatch. Same shape as the Model
// reading device_classes with a re-derive fallback. The Model's IsGroundNet/IsRailNet and the net-role
// relations (stdlib/relations) all read through it.
func NetHasRole(n *ir.Net, role ir.Role, nameMatch func(string) bool) bool {
	if roles := n.GetRoles(); len(roles) > 0 {
		return slices.ContainsFunc(roles, func(r *ir.NetRole) bool { return r.GetRoleKind() == role })
	}
	return nameMatch(n.GetName())
}

// NetRoleSource reports the evidence that established a role on a net, and whether the net carries
// that role at all. It is separate from NetHasRole because almost every caller wants only the boolean.
//
// A net whose role came from the NAME fallback (no stamped set at all, e.g. a hand-authored test IR)
// reports ROLE_SOURCE_CONVENTION, since the fallback is a naming convention read at the point of use
// rather than at ingestion.
func NetRoleSource(n *ir.Net, role ir.Role, nameMatch func(string) bool) (ir.RoleSource, bool) {
	for _, r := range n.GetRoles() {
		if r.GetRoleKind() == role {
			return r.GetSource(), true
		}
	}
	if len(n.GetRoles()) == 0 && nameMatch(n.GetName()) {
		return ir.RoleSource_ROLE_SOURCE_CONVENTION, true
	}
	return ir.RoleSource_ROLE_SOURCE_UNSPECIFIED, false
}
