package check

import "github.com/panyam/agni/core/classify"

// The naming lexicon (RoleVocab and its build/active-vocab machinery) moved to package classify in
// WS3-072 so the ingestion pass can stamp net.role without importing check; check re-exports the names
// (see aliases.go). The is*Name helpers below stay here as the name-match FALLBACK the core uses when a
// net carries no stamped role (a hand-authored test IR that skipped the loader), and as the projection
// the spec-language FFIs (rail_name / ground_name / feedback_name) call on a bare name literal.

// isFeedbackName reports whether a net name is a regulator feedback / sense node (a high-impedance
// divider tap that must not be probed): a rail-NAMED net like "VCC1.2_ETH_FB" reads as feedback, not a
// probe-able rail. Consults the active lexicon (WS3-069), now owned by classify.
func isFeedbackName(name string) bool { return classify.ActiveRoleVocab().IsFeedback(name) }
