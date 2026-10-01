// Package refdes holds what a reference designator MEANS, for the layers that have to agree on it.
//
// It holds one predicate plus the diagnostic built from it. A designator is the join key between a
// schematic symbol, a BOM line and a board footprint, so readers (is this placement a real part?)
// and the check model (does this pin have an identity?) both key on it, and when they disagree they
// silently answer different questions about the same design.
//
// Readers import nothing from core and core imports no reader, so internal/ is the one place both
// reach.
package refdes

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// IsPlaceholder reports whether a reference designator is an unannotated placeholder rather than an
// identity: "R?", "C?", "REF**", and the partly-assigned "C?1845" a tool leaves when only some
// digits are filled in.
//
// A placeholder is annotation STATE, not a name, and keying on it merges unrelated parts. On one
// export 176 distinct resistors shared "R?", so the pin-uniqueness index saw one pin on 129 nets.
// Callers use this to decline rather than merge: a reader skips a placeholder-named footprint, and
// the check model declines to assert pin uniqueness over one.
//
// The "?" matches anywhere, not only as a suffix, because a partly-assigned designator puts it
// mid-name. No tool allows "?" in a real designator, so the wider match is safe.
func IsPlaceholder(ref string) bool {
	return strings.Contains(ref, "?") || strings.HasSuffix(ref, "**")
}

// Unannotated groups the components whose designator is still a placeholder into the
// unannotated_components input diagnostic, one entry per PLACEHOLDER rather than per part, so "176
// parts are still called R?" is one entry. Order follows first appearance, so the diagnostic is
// stable across reads of one file.
//
// A reader that keeps unannotated parts calls this rather than grouping them itself (agni issue
// 311). The board readers SKIP them instead, since an unannotated footprint is usually a fiducial,
// and do not call this.
func Unannotated(comps []*ir.Component) []*ir.UnannotatedComponent {
	byRef := map[string]*ir.UnannotatedComponent{}
	var order []string
	for _, c := range comps {
		if !IsPlaceholder(c.GetRefDes()) {
			continue
		}
		u := byRef[c.GetRefDes()]
		if u == nil {
			u = &ir.UnannotatedComponent{RefDes: c.GetRefDes()}
			byRef[c.GetRefDes()] = u
			order = append(order, c.GetRefDes())
		}
		// One placement per SECTION, since that is what a placement is in the source; a component
		// merged from several unannotated instances carries each.
		for _, s := range c.GetSections() {
			if p := s.GetProv(); p != nil {
				u.Instances = append(u.Instances, p)
			}
		}
	}
	out := make([]*ir.UnannotatedComponent, 0, len(order))
	for _, ref := range order {
		out = append(out, byRef[ref])
	}
	return out
}
