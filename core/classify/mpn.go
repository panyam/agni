package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// MPNAliases are the attribute keys a source spells a manufacturer part number with, in preference
// order. Sources disagree on separator and case, and this list covers that variance.
//
// Exported because a reader needs the same spellings to find the fact in its own grammar (EDIF scans
// cell `property` nodes by name), while deciding where the fact lands is this pass's job.
//
// These are read-only. Nothing writes a canonical MPN attribute; the answer is the typed
// ir.Component.mpn field, and these are the raw keys it is derived FROM.
var MPNAliases = []string{"MPN", "Manufacturer_PN", "Manufacturer PN", "mpn"}

// StampMPN fills ir.Component.mpn once at ingestion, for every format. It is a
// DERIVED-NORMALIZATION pass under C9: no reader populates the field, and a design built without the
// pass leaves it empty, which consumers read as "no part number stated".
//
// It is a shared pass rather than reader work (agni issue 519), because a reader promoting the
// number privately leaves every other format with an empty component.mpn and a silently disabled
// parameter tier. See docsite/content/build/format-reader.md#the-part-number-is-the-one-to-get-wrong-quietly.
//
// Three sources, most specific first, and it stops at the first that answers:
//
//  1. THE COMPONENT'S OWN ATTRIBUTES, under any alias. The usual case, since a part number is stated per
//     placement and a library symbol is coarser than an orderable product.
//  2. ITS PART TYPE's typed mpn, for the sources that model the type AS an orderable part.
//  3. ITS PART TYPE's attributes, under any alias, for a reader that has not been converted to the
//     typed field.
//
// It resolves the part through PartIndex/FirstPart, the same resolution Stamp and check.NewModel use,
// so a component's class and its part number cannot disagree about which part type it has.
//
// Idempotent, since a component that already has one is skipped.
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
