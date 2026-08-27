package classify

import ir "github.com/panyam/agni/gen/go/agni/v1/ir"

// MPNAttr is the CANONICAL component attribute every consumer reads a manufacturer part number from.
// One spelling, on the component, whatever the source called it and wherever the source hung it.
const MPNAttr = "MPN"

// MPNAliases are the spellings a source uses for a part number, in preference order. Sources disagree
// on separator and case, and one format writes the same fact under a different key on the part type
// than on the component, so the variance is data rather than a branch per reader.
//
// "mpn" is here because a reader that meant the canonical key and spelled it lowercase has said the
// same thing; accepting it costs nothing and its absence cost a whole tier (see StampMPN).
//
// Exported because a reader needs the same vocabulary to find the fact in its own grammar (EDIF
// scans cell `property` nodes by name). Extracting the fact from a format is reader work; deciding
// where it lives is this pass's. They must agree on the SPELLINGS, so there is one list.
var MPNAliases = []string{"Manufacturer_PN", "Manufacturer PN", "mpn"}

// StampMPN promotes each component's manufacturer part number to MPNAttr once at ingestion, for every
// format. It is a DERIVED-NORMALIZATION pass under C9's fill variant: no reader populates the
// canonical key, one shared pass does, and a design built without the pass keeps whatever its reader
// wrote (consumers still read the raw attribute, so absence degrades to the old behaviour rather than
// to a false fact).
//
// WHY THIS IS A SHARED PASS AND NOT A READER'S JOB, which is the whole point of agni issue 519.
// The EDIF reader used to carry both halves of this privately, so EDIF designs resolved part numbers
// and nothing else did. Telesis records its part number on the PART TYPE, the model only ever looked
// at the COMPONENT, and the two never met: `component.mpn` came back empty for every component of
// every .tel design. Since the datasheet join is component.mpn -> param, that silently disabled the
// entire parameter tier on that format. Nothing reported an error, because a parameter rule that
// finds no part number cannot tell "no datasheet seeded" from "this format never delivers one".
//
// Two steps, most specific first, and neither ever overwrites a value already present:
//
//  1. ALIAS PROMOTION on the component. The part number is usually already there under the source's
//     own spelling.
//  2. PART-TYPE FALLBACK. A component with none inherits its part type's, because a part number is a
//     property of the PART and a source may state it once per type rather than once per placement.
//
// It resolves the part through PartIndex/FirstPart, the same resolution Stamp and check.NewModel use,
// so a component's class and its part number cannot disagree about which part type it has. The
// reader-local version this replaces joined on the first section's bare PartRef, which missed a part
// whose display name differs from its native id.
//
// Idempotent: re-running finds the canonical key already set and does nothing.
func StampMPN(d *ir.Design) {
	index := PartIndex(d)
	for _, c := range d.GetComponents() {
		if c.GetAttributes()[MPNAttr] != "" {
			continue
		}
		if v := mpnFrom(c.GetAttributes()); v != "" {
			setMPN(c, v)
			continue
		}
		if p := FirstPart(index, c); p != nil {
			if v := partMPN(p); v != "" {
				setMPN(c, v)
			}
		}
	}
}

// partMPN reads a part type's number under the canonical key or any alias.
func partMPN(p *ir.PartType) string {
	if v := p.GetAttributes()[MPNAttr]; v != "" {
		return v
	}
	return mpnFrom(p.GetAttributes())
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

// setMPN writes the canonical key, leaving the source's own spelling in place. Nothing is lost or
// rewritten: a reader inspecting the original attribute still finds it.
func setMPN(c *ir.Component, v string) {
	if c.Attributes == nil {
		c.Attributes = map[string]string{}
	}
	c.Attributes[MPNAttr] = v
}
