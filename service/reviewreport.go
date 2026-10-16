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

// evidenceOf turns an item's findings into linked rows.
//
// THE LINK IS SYNTHESIZED. A review item carries findings, not verdicts, because core/review keeps
// what fired rather than what was considered. VerdictID is derived from (rule, subjects), so the id a
// verdict for this finding would carry can be computed without one in hand.
//
// KNOWN LIMIT (agni issue 349): a rule whose verdict names a TUPLE (a clearance between two nets)
// files its finding under one subject, so the id built here names one entity and the real verdict
// names two. The viewer treats that id as a stale link, opening the design with the checks run and
// nothing highlighted.
func evidenceOf(it *checkspb.ReviewItem, meta report.Checklist) []report.ChecklistEvidence {
	if len(it.GetFindings()) == 0 {
		return nil
	}
	out := make([]report.ChecklistEvidence, 0, len(it.GetFindings()))
	for _, f := range it.GetFindings() {
		subject := check.Entity{Kind: f.GetSubject().GetKind(), Ref: f.GetSubject().GetRef(), Pin: f.GetSubject().GetPin()}
		id := check.VerdictID(check.Verdict{Rule: f.GetRule(), Subjects: []check.Entity{subject}})
		out = append(out, report.ChecklistEvidence{
			Rule:    f.GetRule(),
			Subject: check.EntityRef(subject),
			Message: f.GetMessage(),
			URL: report.VerdictURL(report.Report{
				URLBase: meta.URLBase, MountPath: meta.MountPath, ContentHash: meta.ContentHash,
			}, id, f.GetRule()),
		})
	}
	return out
}
