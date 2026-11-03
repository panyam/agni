package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// MPNAliases are the built-in attribute keys a source spells a manufacturer part number with, in
// preference order, and the default of the lexicon's attribute.mpn. A project adds its own there.
//
// Exported because a reader needs the same spellings to find the fact in its own grammar (EDIF scans
// cell `property` nodes by name), while deciding where the fact lands is this pass's job. A reader
// reads no configuration, so a project's own keys never reach that scan.
//
// These are read-only. Nothing writes a canonical MPN attribute; the answer is the typed
// ir.Component.mpn field, and these are the raw keys it is derived FROM. KiCad's "Part Number" field
// is here since public boards (HackRF One, Cynthion) carry their part numbers in it (agni 1285).
var MPNAliases = []string{"MPN", "Manufacturer_PN", "Manufacturer PN", "mpn", "Part Number"}

// StampMPN fills ir.Component.mpn once at ingestion, for every format. It is a
// DERIVED-NORMALIZATION pass under C9: no reader populates the field, and a design built without the
// pass leaves it empty, which consumers read as "no part number stated".
//
// It is a shared pass rather than reader work (agni issue 519), because a reader promoting the
// number privately leaves every other format with an empty component.mpn and a silently disabled
// parameter tier. See docsite/content/build/format-reader.md#getting-the-part-number-wrong-fails-quietly.
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
// Idempotent, since a component that already has one is skipped. The keys come from the read's
// lexicon (attribute.mpn), which is why the pass runs in DeriveDesign rather than with the parse.
func (l *Lexicon) StampMPN(d *ir.Design) {
	v := l.role()
	index := PartIndex(d)
	for _, c := range d.GetComponents() {
		if c.GetMpn() != "" {
			continue
		}
		if m := componentAttr(c, v.attrs.mpn); m != "" {
			c.Mpn = m
			continue
		}
		if p := FirstPart(index, c); p != nil {
			if m := p.GetMpn(); m != "" {
				c.Mpn = m
			} else if m := attrValue(p.GetAttributes(), v.attrs.mpn); m != "" {
				c.Mpn = m
			}
		}
	}
}

// StampMPN is the process-level form of (*Lexicon).StampMPN.
func StampMPN(d *ir.Design) { ActiveLexicon().StampMPN(d) }
