// Package refdes holds what a reference designator MEANS, for the layers that have to agree on it.
//
// It exists for one predicate, plus the one diagnostic built from it. A designator is the join key
// between a schematic symbol, a BOM line and a footprint on the board, so several layers key on it
// independently: readers decide whether a placement is a real part, and the check model decides
// whether a pin has an identity. When those layers disagree about what counts as a designator they
// do not fail loudly — they quietly answer different questions about the same design.
//
// Readers deliberately import nothing from core, and core imports no reader, so neither could own
// this. internal/ is the one place both already reach.
package refdes

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// IsPlaceholder reports whether a reference designator is an unannotated placeholder rather than an
// identity: "R?", "C?", "REF**", and the partly-assigned "C?1845" a tool leaves when only some
// digits are filled in.
//
// A placeholder is annotation STATE, not a name. Every unannotated resistor on a sheet reads "R?",
// so treating it as a key merges parts that have nothing to do with each other: on one export 176
// distinct resistors shared it, and the pin-uniqueness index consequently saw one pin sitting on
// 129 nets. Callers use this to decline rather than to merge — a reader skips a placeholder-named
// footprint, and the check model declines to assert pin uniqueness over one.
//
// The "?" match is deliberately anywhere in the string, not a suffix: a partly-assigned designator
// puts it mid-name. "?" is not a legal character in a reference designator in any tool that writes
// one, so the wider match cannot swallow a real designator.
func IsPlaceholder(ref string) bool {
	return strings.Contains(ref, "?") || strings.HasSuffix(ref, "**")
}

// Unannotated groups the components whose designator is still a placeholder into the
// unannotated_components input diagnostic, one entry per PLACEHOLDER rather than per part: "176
// parts are still called R?" is the reviewable fact, and 176 identical entries is the same sentence
// 176 times. Order follows first appearance so the diagnostic is stable across reads of one file.
//
// It lives beside the predicate because a reader that keeps unannotated parts has to say so, and a
// reader deriving its own grouping is how the predicate stopped being single in the first place
// (agni issue 311). A reader that SKIPS them instead — the board readers do, since an unannotated
// footprint is usually a fiducial rather than a part — has nothing to report and does not call this.
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
