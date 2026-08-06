package foreign

import (
	"strings"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// Join attaches each imported finding to an entity in the model, and reports what it could not attach.
//
// The join is by NAME — the ref_des, pin, and net a foreign checker printed into its item description
// — because that is the only identity the two tools share. KiCad's uuids are carried through as
// provenance, but they identify board and schematic OBJECTS (this track, this pad), and our model is a
// netlist of components and nets, so there is nothing to join a track uuid to.
//
// Two rules govern it, and both exist so that an import cannot quietly overstate itself:
//
// A finding is only joined to an entity the model actually HAS. A parsed ref_des that names no
// component leaves the finding unjoined rather than inventing a subject, because a wrong join attaches
// a real violation to an innocent part, which is worse than no join at all.
//
// An unjoined finding is KEPT, never dropped, and counted in the summary. Dropping it would make the
// import report fewer problems than the tool found, with nothing to say so.
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

// notInDesign is the residue class that matters most: the import UNDERSTOOD the description and the
// entity is simply not in the model we loaded. That is a real signal — the report was run against a
// different revision, or against the board while we read the schematic — and it is deliberately
// distinct from "we did not recognize the shape".
const notInDesign = "an entity the description names but the loaded design does not contain"

// itemDescription recovers the per-item half of a finding message. findings() joins the violation
// description and the item description with an em-dash-spaced separator, so the last segment is the
// item. A violation reported with no items has no separator and yields "".
func itemDescription(msg string) string {
	i := strings.LastIndex(msg, " — ")
	if i < 0 {
		return ""
	}
	return msg[i+len(" — "):]
}

// subjectKind reports whether a parsed reference names a pin of a component or the component itself,
// which is what decides how a consumer highlights it.
func subjectKind(r itemRef) string {
	if r.Pin != "" {
		return check.KindPin
	}
	return check.KindComponent
}

// hasNet reports whether the model carries a net of this name. Names are not unique (duplicate-net-name
// is a shipped rule), so this answers presence only; a foreign report carries nothing that could pick
// between two same-named nets, and inventing a choice would be a guess.
func hasNet(m check.Model, name string) bool {
	for _, n := range m.Nets() {
		if n.GetName() == name {
			return true
		}
	}
	return false
}
