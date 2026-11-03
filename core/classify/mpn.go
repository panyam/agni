package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// MPNAliases lists the built-in attribute keys for a manufacturer part number, in preference order.
// It is the default of the lexicon's attribute.mpn, and a project adds its own keys there.
//
// It is exported because a reader needs the same spellings to find the fact in its own grammar (EDIF
// scans cell `property` nodes by name). This pass decides which field the fact goes into. A reader
// reads no configuration, so a project's own keys never reach that scan.
//
// These are read-only. Nothing writes a canonical MPN attribute. The answer goes in the typed
// ir.Component.mpn field, and the pass derives it FROM these raw keys. KiCad's "Part Number" field is
// here since public boards (HackRF One, Cynthion) put their part numbers in it (agni 1285).
var MPNAliases = []string{"MPN", "Manufacturer_PN", "Manufacturer PN", "mpn", "Part Number"}

// StampMPN fills ir.Component.mpn once at ingestion, for every format. It is a
// DERIVED-NORMALIZATION pass under C9. No reader populates the field. A design built without the pass
// leaves it empty, and consumers read that as "no part number stated".
//
// It is a shared pass rather than reader work (agni issue 519). If one reader promoted the number
// privately, every other format would have an empty component.mpn and a silently disabled parameter
// tier. See docsite/content/build/format-reader.md#the-part-number-is-the-one-to-get-wrong-quietly.
//
// It tries three sources, most specific first, and stops at the first that answers.
//
//  1. THE COMPONENT'S OWN ATTRIBUTES, under any alias. The usual case, since a part number is stated per
//     placement and a library symbol is coarser than an orderable product.
//  2. ITS PART TYPE's typed mpn, for the sources that model the type AS an orderable part.
//  3. ITS PART TYPE's attributes, under any alias, for a reader that has not been converted to the
//     typed field.
//
// It resolves the part through PartIndex/FirstPart. Stamp and check.NewModel resolve it the same way,
// so a component's class and its part number cannot disagree about which part type it has.
//
// It is idempotent, since it skips a component that already has an MPN. The keys come from the read's
// lexicon (attribute.mpn), so the pass runs in DeriveDesign rather than with the parse.
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
