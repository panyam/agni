// Package refdes holds what a reference designator MEANS, for the layers that have to agree on it.
//
// It exists for one predicate. A designator is the join key between a schematic symbol, a BOM line
// and a footprint on the board, so several layers key on it independently: readers decide whether a
// placement is a real part, and the check model decides whether a pin has an identity. When those
// layers disagree about what counts as a designator they do not fail loudly — they quietly answer
// different questions about the same design.
//
// Readers deliberately import nothing from core, and core imports no reader, so neither could own
// this. internal/ is the one place both already reach.
package refdes

import "strings"

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
