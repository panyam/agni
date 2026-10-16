package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// RenderReviewReport writes a review the caller holds as the checklist page (agni issue 1030). It
// reads neither the store nor the design, since the review's document carries the manifest's items,
// their outcomes and findings, and the content hash of the bytes the run read. `agni review --format
// html` and the viewer's Save report both call it, so the two pages differ only in their timestamps
// and in how each names the design.
//
// A request with no review, or with links naming a design that is not a mount URI, is an invalid
// argument.
func (s *ReviewService) RenderReviewReport(_ context.Context, req *webapi.RenderReviewReportRequest) (*webapi.RenderReviewReportResponse, error) {
	doc := req.GetReview().GetResults()
	if doc == nil {
		return nil, fmt.Errorf("%w: RenderReviewReport needs the review to write", ErrInvalidArgument)
	}
	links := req.GetLinks()
	mountPath := ""
	if links.GetDesignUri() != "" {
		lu, err := ParseArtifactURI(links.GetDesignUri())
		if err != nil {
			return nil, err
		}
		mountPath = strings.TrimPrefix(lu.String(), "mount://")
	}
	meta := report.Checklist{
		Design:        doc.GetDesign().GetSource(),
		Generated:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ContentHash:   doc.GetDesign().GetContentHash(),
		URLBase:       strings.TrimSuffix(links.GetUrlBase(), "/"),
		MountPath:     mountPath,
		LinksWithheld: links.GetWithheld(),
		// The document's record of what the read left out, in the sentence the CLI prints (agni issue
		// 717).
		HierarchyNote: report.HierarchyNote(doc.GetDesign().GetSource(), doc.GetDesign().GetUnexpandedHierarchy()),
	}
	var buf bytes.Buffer
	if err := report.ChecklistHTML(&buf, ChecklistOf(doc, meta)); err != nil {
		return nil, err
	}
	name := reportBaseName(strings.TrimPrefix(doc.GetDesign().GetSource(), "mount://"))
	return &webapi.RenderReviewReportResponse{Content: buf.Bytes(), ContentType: "text/html; charset=utf-8", Filename: name + ".agni-review.html"}, nil
}

// ChecklistOf maps one review's document onto the checklist page's model, with meta supplying the
// header and the links.
//
// Area and item ORDER is the manifest's, untouched. The check report sorts rules worst-first because
// nobody authored that order, while a checklist's order is the team's.
func ChecklistOf(doc *checkspb.CheckResults, meta report.Checklist) report.Checklist {
	out := meta
	out.Name = doc.GetManifest()
	if out.Name == "" {
		out.Name = "Review"
	}
	var all []review.Outcome
	for _, a := range doc.GetAreas() {
		var outcomes []review.Outcome
		area := report.ChecklistArea{Name: a.GetName()}
		for _, it := range a.GetItems() {
			outcomes = append(outcomes, review.Outcome(it.GetOutcome()))
			area.Items = append(area.Items, report.ChecklistItem{
				ID:       it.GetId(),
				Title:    it.GetTitle(),
				Outcome:  it.GetOutcome(),
				Note:     it.GetNote(),
				Evidence: evidenceOf(it, meta),
			})
		}
		area.Summary = review.TallyOf(outcomes...).String()
		out.Areas = append(out.Areas, area)
		all = append(all, outcomes...)
	}
	t := review.TallyOf(all...)
	out.Summary = t.String()
	out.Covered, out.Answered, out.Total = t.Covered(), t.Answered(), t.Total
	return out
}

// evidenceOf turns an item's findings into rows, each linked to the verdict the finding was
// projected from (agni issue 485). A finding names its verdicts in verdict_ids, and the row links the
// first; a finding from a check run carries exactly one. A finding with none, from a rule that states
// no considered set, gets no link, since there is no verdict for a viewer to open.
func evidenceOf(it *checkspb.ReviewItem, meta report.Checklist) []report.ChecklistEvidence {
	if len(it.GetFindings()) == 0 {
		return nil
	}
	out := make([]report.ChecklistEvidence, 0, len(it.GetFindings()))
	for _, f := range it.GetFindings() {
		subject := check.Entity{Kind: f.GetSubject().GetKind(), Ref: f.GetSubject().GetRef(), Pin: f.GetSubject().GetPin()}
		e := report.ChecklistEvidence{
			Rule:    f.GetRule(),
			Subject: check.EntityRef(subject),
			Message: f.GetMessage(),
		}
		if ids := f.GetVerdictIds(); len(ids) > 0 {
			e.URL = report.VerdictURL(report.Report{
				URLBase: meta.URLBase, MountPath: meta.MountPath, ContentHash: meta.ContentHash,
			}, ids[0], f.GetRule())
		}
		out = append(out, e)
	}
	return out
}
