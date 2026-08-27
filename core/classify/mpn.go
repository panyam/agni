package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// MPNAliases are the attribute keys a source spells a manufacturer part number with, in preference
// order. Sources disagree on separator and case, and this is the whole of that variance.
//
// Exported because a reader needs the same vocabulary to find the fact in its own grammar (EDIF
// scans cell `property` nodes by name). Extracting the fact from a format is reader work; deciding
// where it lands is this pass's. They must agree on the SPELLINGS, so there is one list.
//
// Note these are read-only: nothing writes a canonical MPN attribute any more. The answer is the
// typed ir.Component.mpn field, and these are the raw keys it is derived FROM.
var MPNAliases = []string{"MPN", "Manufacturer_PN", "Manufacturer PN", "mpn"}

// StampMPN fills ir.Component.mpn once at ingestion, for every format. It is a
// DERIVED-NORMALIZATION pass under C9: no reader populates the field, one shared pass does, and a
// design built without the pass leaves it empty, which consumers read as "no part number stated"
// rather than as a fact about the design.
//
// WHY THIS IS A SHARED PASS AND NOT A READER'S JOB, which is the whole point of agni issue 519.
// The EDIF reader used to carry both halves of this privately, so EDIF designs resolved part numbers
// and nothing else did. Telesis records its part number on the PART TYPE, every consumer read the
// component, and the two never met: `component.mpn` came back empty for every component of every
// .tel design. Since the datasheet join is component.mpn -> param, that silently disabled the entire
// parameter tier on that format. Nothing reported an error, because a parameter rule that finds no
// part number cannot tell "no datasheet seeded" from "this format never delivers one".
//
// Three sources, most specific first, and it stops at the first that answers:
//
//  1. THE COMPONENT'S OWN ATTRIBUTES, under any alias. The usual case: a part number is stated per
//     placement, because a library symbol is coarser than an orderable product.
//  2. ITS PART TYPE's typed mpn, for the sources that model the type AS an orderable part.
//  3. ITS PART TYPE's attributes, under any alias, for a reader that has not been converted to the
//     typed field.
//
// It resolves the part through PartIndex/FirstPart, the same resolution Stamp and check.NewModel use,
// so a component's class and its part number cannot disagree about which part type it has.
//
// Idempotent: a component that already has one is skipped, so re-running is a no-op.
func StampMPN(d *ir.Design) {
	index := PartIndex(d)
	for _, c := range d.GetComponents() {
		if c.GetMpn() != "" {
			continue
		}
		if v := mpnFrom(c.GetAttributes()); v != "" {
			c.Mpn = v
			continue
		}
		if p := FirstPart(index, c); p != nil {
			if v := p.GetMpn(); v != "" {
				c.Mpn = v
			} else if v := mpnFrom(p.GetAttributes()); v != "" {
				c.Mpn = v
			}
		}
	}
}

// mpnFrom returns the first alias present in an attribute map, or "".
func mpnFrom(attrs map[string]string) string {
	for _, a := range MPNAliases {
		if v := attrs[a]; v != "" {
			return v
		}
	}
	return ""
}
