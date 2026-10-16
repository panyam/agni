package main

import (
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/service"
)

// reportsFromDocs maps stored review documents back to the Go view-model the renderers consume, the
// CLI analogue of the web tier's reportFromWire. The document's item note is the already-joined
// runtime+manifest note (reviewAreaProtos), placed in Item.Note so detail() and JSON reproduce it in
// every outcome case, since they read Item.Note for pass/not-automated and join it for not-applicable.
func reportsFromDocs(docs []*checkspb.CheckResults) []review.Report {
	reports := make([]review.Report, 0, len(docs))
	for _, doc := range docs {
		r := review.Report{Manifest: doc.GetManifest(), Design: displayName(doc.GetDesign().GetSource())}
		for _, pa := range doc.GetAreas() {
			ar := review.AreaResult{Area: review.Area{Name: pa.GetName()}}
			for _, pi := range pa.GetItems() {
				ar.Items = append(ar.Items, review.ItemResult{
					Item:     review.Item{ID: pi.GetId(), Title: pi.GetTitle(), Note: pi.GetNote()},
					Outcome:  review.Outcome(pi.GetOutcome()),
					Findings: findingsFromProto(pi.GetFindings()),
					Unmet:    unmetFromProto(pi.GetUnmet()),
				})
			}
			r.Areas = append(r.Areas, ar)
		}
		reports = append(reports, r)
	}
	return reports
}

// findingsFromProto reads a document's findings back through the service's one inverse of
// FindingProto, so the CLI's renderers see every field the wire carries, verdict ids included.
func findingsFromProto(fs []*checkspb.Finding) []check.Finding {
	return service.FindingsFromProto(fs)
}

// designSourcesOf returns each document's design URI, positionally matching reportsFromDocs.
//
// The reports carry a READING name and the documents carry the address, which a renderer that opens
// the file needs. Parallel slices rather than a URI on review.Report keep `core/review` free of any
// notion of how an artifact is addressed.
func designSourcesOf(docs []*checkspb.CheckResults) []string {
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.GetDesign().GetSource())
	}
	return out
}

// unmetFromProto reconstructs a needs-data item's unmet dependencies, so a results document
// re-rendered from the wire carries the same work list the run produced (agni issue 258).
func unmetFromProto(ps []*checkspb.UnmetDependency) []check.UnmetDependency {
	if len(ps) == 0 {
		return nil
	}
	out := make([]check.UnmetDependency, 0, len(ps))
	for _, p := range ps {
		out = append(out, check.UnmetDependency{
			MPN: p.GetMpn(), Manufacturer: p.GetManufacturer(),
			Symbol: p.GetSymbol(), SpecAbsent: p.GetSpecAbsent(),
		})
	}
	return out
}
