package main

import (
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/core/review"
)

// reportsFromDocs maps stored review documents back to the Go view-model the renderers consume — the
// CLI analogue of the web tier's reportFromWire, so both surfaces start from the one shape a review
// run produces. The document's item note is the already-joined runtime+manifest note
// (reviewAreaProtos), placed in Item.Note so detail()/JSON reproduce it in every outcome case (they
// read Item.Note for pass/not-automated and join it for not-applicable — one field satisfies both).
func reportsFromDocs(docs []*checkspb.CheckResults) []review.Report {
	reports := make([]review.Report, 0, len(docs))
	for _, doc := range docs {
		r := review.Report{Manifest: doc.GetManifest(), Design: doc.GetDesign().GetSource()}
		for _, pa := range doc.GetAreas() {
			ar := review.AreaResult{Area: review.Area{Name: pa.GetName()}}
			for _, pi := range pa.GetItems() {
				ar.Items = append(ar.Items, review.ItemResult{
					Item:     review.Item{ID: pi.GetId(), Title: pi.GetTitle(), Note: pi.GetNote()},
					Outcome:  review.Outcome(pi.GetOutcome()),
					Findings: findingsFromProto(pi.GetFindings()),
				})
			}
			r.Areas = append(r.Areas, ar)
		}
		reports = append(reports, r)
	}
	return reports
}

// findingsFromProto reconstructs the check.Finding list a review item's renderers read (rule, subject,
// message, provenance -> source file, and the datasheet citation the JSON surface prints). The subject
// name is the proto Subject.Ref for every kind (FindingProto sets it, plus BusId for a bus).
func findingsFromProto(fs []*checkspb.Finding) []check.Finding {
	out := make([]check.Finding, 0, len(fs))
	for _, f := range fs {
		out = append(out, check.Finding{
			Rule:          f.GetRule(),
			Severity:      f.GetSeverity(),
			Kind:          f.GetSubject().GetKind(),
			Subject:       f.GetSubject().GetRef(),
			Pin:           f.GetSubject().GetPin(),
			NetID:         f.GetSubject().GetNetId(),
			Message:       f.GetMessage(),
			Prov:          f.GetProvenance(),
			DatasheetProv: datasheetsFromProto(f.GetDatasheets()),
		})
	}
	return out
}

// datasheetsFromProto reconstructs a finding's citations from the wire form, preserving order.
func datasheetsFromProto(cs []*checkspb.DatasheetCitation) []*check.DatasheetCitation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*check.DatasheetCitation, 0, len(cs))
	for _, c := range cs {
		if dc := datasheetFromProto(c); dc != nil {
			out = append(out, dc)
		}
	}
	return out
}

// datasheetFromProto reconstructs a finding's datasheet citation from the wire form, nil when the
// finding carries none.
func datasheetFromProto(c *checkspb.DatasheetCitation) *check.DatasheetCitation {
	if c == nil {
		return nil
	}
	return &check.DatasheetCitation{
		Doc:        c.GetDoc(),
		DocRef:     c.GetDocRef(),
		Page:       c.GetPage(),
		Section:    c.GetSection(),
		Method:     c.GetMethod(),
		Confidence: c.GetConfidence(),
	}
}
