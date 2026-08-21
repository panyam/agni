package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Helpers shared by the converted rule bodies (agni issue 391). They exist because a witness has to
// NAME the part its statement rests on, not merely assert that one exists: a reader told a rail is
// decoupled still has to know which capacitor said so before they can judge whether the answer is
// right. `check.Exists` answers the predicate and throws the entity away, so every converted rule
// that used it needs the entity back.

// firstOnNet names the first part of a class on a net, in connection order, or "" when the net
// carries none. Connection order rather than any ranking, because the witness is evidence that ONE
// exists and the rule's own question is existential; picking a "best" one would imply a judgement
// the rule never made.
func firstOnNet(m check.Model, n *ir.Net, class check.ComponentClass) string {
	for _, c := range n.Connections {
		if m.HasClass(c.ComponentRef, class) {
			return c.ComponentRef
		}
	}
	return ""
}

// firstPassive names the first passive part on a net in connection order, or "" when there is none.
// Broader than firstOnNet with one class: `check.IsPassiveClass` spans the R/C/L/ferrite/fuse/
// test-point family that makes a net not-provably-floating.
func firstPassive(m check.Model, n *ir.Net) string {
	for _, c := range n.Connections {
		if check.IsPassiveClass(m.ComponentClass(c.ComponentRef)) {
			return c.ComponentRef
		}
	}
	return ""
}

// compContext is the one-entity Context a witness attaches when it names a part. Role is the
// author's word for what the part is doing in the proof, which is what a viewer renders beside the
// chip it lights up.
func compContext(ref, role string) []check.ContextSubject {
	return []check.ContextSubject{{Entity: check.Entity{Kind: check.KindComponent, Ref: ref}, Role: role}}
}
