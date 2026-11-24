package foreign

import (
	"strings"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// Join attaches each imported finding to an entity in the model, and reports what it could not attach.
//
// The join is by NAME, since a name is the only identity the two tools share. The names are the
// ref_des, pin and net that a foreign checker printed into its item description. KiCad's uuids remain
// as provenance, but they name board and schematic OBJECTS (a track, a pad), and our netlist model has
// nothing to join those to.
//
// A finding joins only to an entity the model HAS, because a wrong join pins a real violation on an
// innocent part. An unjoined finding is KEPT and counted in the summary, never dropped. See
// docsite/content/architecture/checks-contract.md#importing-another-tools-results.
func Join(m check.Model, doc *checkspb.CheckResults) {
	unjoined := map[string][]string{}
	joined := 0
	for _, f := range doc.GetFindings() {
		desc := itemDescription(f.GetMessage())
		ref := parseItem(desc)
		switch {
		case ref.RefDes != "" && m.HasComponent(ref.RefDes):
			f.Subject = &checkspb.Subject{Kind: subjectKind(ref), Ref: ref.RefDes, Pin: ref.Pin}
			joined++
		case ref.Net != "" && hasNet(m, ref.Net):
			f.Subject = &checkspb.Subject{Kind: check.KindNet, Ref: ref.Net}
			joined++
		case ref.empty():
			unjoined[residueClass(desc)] = append(unjoined[residueClass(desc)], desc)
		default:
			unjoined[notInDesign] = append(unjoined[notInDesign], desc)
		}
	}
	doc.ImportSummary = summarize(len(doc.GetFindings()), joined, unjoined)
}

// notInDesign is the residue class that matters most. The import UNDERSTOOD the description and the
// entity is not in the model we loaded. Usually the report ran against a different revision, or
// against the board while we read the schematic. It is kept distinct from "we did not recognize the
// shape".
const notInDesign = "an entity the description names but the loaded design does not contain"

// itemDescription recovers the per-item half of a finding message. findings() appends the item
// description wrapped in itemOpen and itemClose. A violation reported with no items has no wrapper
// and yields "". Results imported before #1875 joined the two halves with a spaced em-dash instead,
// and a stored results file keeps that form, so it still parses.
func itemDescription(msg string) string {
	if strings.HasSuffix(msg, itemClose) {
		if i := strings.LastIndex(msg, itemOpen); i >= 0 {
			return msg[i+len(itemOpen) : len(msg)-len(itemClose)]
		}
	}
	if i := strings.LastIndex(msg, legacyItemSeparator); i >= 0 {
		return msg[i+len(legacyItemSeparator):]
	}
	return ""
}

// legacyItemSeparator joined a violation and its item in results imported before #1875.
const legacyItemSeparator = " \u2014 "

// subjectKind reports whether a parsed reference names a pin of a component or the component itself.
// The kind decides how a consumer highlights it.
func subjectKind(r itemRef) string {
	if r.Pin != "" {
		return check.KindPin
	}
	return check.KindComponent
}

// hasNet reports whether the model carries a net of this name. Names are not unique
// (duplicate-net-name is a shipped rule), so this answers presence only, and a foreign report carries
// nothing that could pick between two same-named nets.
func hasNet(m check.Model, name string) bool {
	for _, n := range m.Nets() {
		if n.GetName() == name {
			return true
		}
	}
	return false
}
