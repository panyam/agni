package builtin

import (
	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Helpers shared by the converted rule bodies (agni issue 391). They exist because a witness has to
// NAME the part its statement rests on, not merely assert that one exists. A reader told a rail is
// decoupled still has to know which capacitor said so before they can judge whether the answer is
// right. `check.Exists` answers the predicate and throws the entity away, so every converted rule
// that used it needs the entity back.

// firstOnNet names the first part of a class on a net, in connection order, or "" when the net
// carries none. Connection order rather than any ranking, because the witness is evidence that ONE
// exists and the rule's own question is existential; picking a "best" one would imply a judgement
// the rule never made. A part marked do-not-populate is skipped, since the assembled board has none
// to credit (agni issue 961).
func firstOnNet(m check.Model, n *ir.Net, class check.ComponentClass) string {
	for _, c := range n.Connections {
		if m.HasClass(c.ComponentRef, class) && m.IsFitted(c.ComponentRef) {
			return c.ComponentRef
		}
	}
	return ""
}

// firstPassive names the first passive part on a net in connection order, or "" when there is none.
// Broader than firstOnNet with one class, since `check.IsPassiveClass` spans the R/C/L/ferrite/fuse/
// test-point family that makes a net not-provably-floating. A part marked do-not-populate is skipped,
// since it is no pull on the assembled board (agni issue 1082).
func firstPassive(m check.Model, n *ir.Net) string {
	for _, c := range n.Connections {
		if check.IsPassiveClass(m.ComponentClass(c.ComponentRef)) && m.IsFitted(c.ComponentRef) {
			return c.ComponentRef
		}
	}
	return ""
}

// railCapacitorWitness is the pass evidence for a capacitor check.FirstOnRail found. A capacitor
// across a zero-ohm link names the link and the net it sits on, so a reviewer looking at the rail
// itself is told where to look. label is the rule's word for the capacitor.
func railCapacitorWitness(ref string, at check.JoinedNet, label string) (*check.Witness, []check.ContextSubject) {
	if at.Via == "" {
		return &check.Witness{
			Statement: "capacitor " + ref + " sits on the rail",
			Terms:     []check.WitnessTerm{{Label: label, Value: ref}},
		}, compContext(ref, "capacitor on the rail")
	}
	return &check.Witness{
		Statement: "capacitor " + ref + " sits on " + at.Net.GetName() + ", which zero-ohm link " + at.Via + " joins to the rail",
		Terms:     []check.WitnessTerm{{Label: label, Value: ref}, {Label: "zero-ohm link", Value: at.Via}},
	}, []check.ContextSubject{
		{Entity: check.Entity{Kind: check.KindComponent, Ref: ref}, Role: "capacitor on the rail"},
		{Entity: check.Entity{Kind: check.KindComponent, Ref: at.Via}, Role: "zero-ohm link"},
	}
}

// compContext is the one-entity Context a witness attaches when it names a part. Role is the
// author's word for what the part is doing in the proof, which is what a viewer renders beside the
// chip it lights up.
func compContext(ref, role string) []check.ContextSubject {
	return []check.ContextSubject{{Entity: check.Entity{Kind: check.KindComponent, Ref: ref}, Role: role}}
}
