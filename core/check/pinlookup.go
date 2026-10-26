package check

import (
	"sort"

	"github.com/panyam/agni/core/ident"
)

// PinLookupKind is what a pin lookup found. The five stay apart because each sends a reader
// somewhere different: a device the design lacks, a device the read gave no pins for, a spelling
// no pin answers to, and one that several pins answer to.
type PinLookupKind string

const (
	// PinFound means exactly one pin answers to the spelling.
	PinFound PinLookupKind = "found"
	// PinAmbiguous means several pins answer to it, as a name a part prints on four GND pins does.
	PinAmbiguous PinLookupKind = "ambiguous"
	// PinUnknown means the part has pins and none answers to the spelling.
	PinUnknown PinLookupKind = "unknown"
	// PinNoComponent means the design has no component of that ref-des.
	PinNoComponent PinLookupKind = "no-component"
	// PinNoPinList means the read gave the component no part-type pins, so no spelling can resolve.
	PinNoPinList PinLookupKind = "no-pin-list"
)

// PinLookup is the answer to which design pin a spelling names.
type PinLookup struct {
	Kind PinLookupKind
	// Designator is the design's own spelling of the pin, set when Kind is PinFound. Every later
	// lookup (a net, a connection) uses it.
	Designator string
	// Via is how the spelling matched that pin, so a caller can say a match was inferred.
	Via ident.Result
	// Candidates are the designators that matched, in order, when Kind is PinAmbiguous.
	Candidates []string
}

// LookupPin resolves a pin spelled as a package designator or as the part type's functional name
// (agni issues 517, 1193). Both compare through core/ident, so `PTE7`, `PTE07` and a name carrying
// an invisible character pasted from a spreadsheet reach the same pin. The designator wins when a
// pin matches both ways, since it is the netlist's own key.
//
// It is the one implementation of that question, read by the IO-map rules and by trace, so a pin a
// map can name is a pin a trace can start from.
func LookupPin(m Model, refDes, spelled string) PinLookup {
	if !m.HasComponent(refDes) {
		return PinLookup{Kind: PinNoComponent}
	}
	var pins []string
	for _, p := range m.Pins() {
		if p.Component.GetRefDes() == refDes {
			pins = append(pins, p.Designator)
		}
	}
	if len(pins) == 0 {
		return PinLookup{Kind: PinNoPinList}
	}
	sort.Strings(pins)
	type hit struct {
		designator string
		via        ident.Result
	}
	var hits []hit
	for _, des := range pins {
		byDes := ident.Compare(spelled, des)
		byName := ident.Result{Match: ident.None}
		if name := m.PinName(refDes, des); name != "" && name != "~" {
			byName = ident.Compare(spelled, name)
		}
		if best := strongerMatch(byDes, byName); best.Match != ident.None {
			hits = append(hits, hit{des, best})
		}
	}
	switch len(hits) {
	case 0:
		return PinLookup{Kind: PinUnknown}
	case 1:
		return PinLookup{Kind: PinFound, Designator: hits[0].designator, Via: hits[0].via}
	}
	out := PinLookup{Kind: PinAmbiguous}
	for _, h := range hits {
		out.Candidates = append(out.Candidates, h.designator)
	}
	return out
}

// strongerMatch picks the more confident of two readings of one pin, so a designator hit is never
// downgraded by a weaker name hit on the same pin.
func strongerMatch(a, b ident.Result) ident.Result {
	if MatchRank(a.Match) >= MatchRank(b.Match) {
		return a
	}
	return b
}

// MatchRank orders ident matches by confidence, exact highest and none lowest, for a caller keeping
// the best of several comparisons.
func MatchRank(m ident.Match) int {
	switch m {
	case ident.Exact:
		return 3
	case ident.Normalized:
		return 2
	case ident.Fuzzy:
		return 1
	}
	return 0
}
