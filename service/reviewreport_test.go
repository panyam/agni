package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// These moved from cmd/agni with the page they describe (agni issue 1030): the checklist page is
// built here, from the review's document, for the CLI and the viewer alike.

func reviewItem(id, title, outcome string, fs ...*checkspb.Finding) *checkspb.ReviewItem {
	return &checkspb.ReviewItem{Id: id, Title: title, Outcome: outcome, Findings: fs}
}

func netFinding(rule, net, msg string) *checkspb.Finding {
	return &checkspb.Finding{Rule: rule, Subject: &checkspb.Subject{Kind: check.KindNet, Ref: net}, Message: msg}
}

// twoAreas is a run whose areas and items are NOT in alphabetical or worst-first order, so a
// renderer that sorts is visible.
func twoAreas() *checkspb.CheckResults {
	return &checkspb.CheckResults{
		Design:   &checkspb.DesignRef{Source: "mount://gw/board.edn", ContentHash: "abc123"},
		Manifest: "House checklist",
		Areas: []*checkspb.ReviewArea{
			{Name: "Power", Items: []*checkspb.ReviewItem{
				reviewItem("P1", "every rail carries a bulk capacitor", "pass"),
				reviewItem("P2", "every rail carries decoupling", "fail", netFinding("decoupling-present", "V12", "no decoupling")),
			}},
			{Name: "Interfaces", Items: []*checkspb.ReviewItem{
				reviewItem("I1", "the CAN interface is terminated", "fail", netFinding("can-term", "CANH", "not terminated")),
			}},
		},
	}
}

// TestChecklistKeepsTheManifestOrder guards the checklist's order. The check report sorts rules
// worst-first because nobody authored that order, but a checklist's order is the team's. An item
// that has been P1 in their process for years has to stay first, and an area must not move because
// the board got worse this week.
func TestChecklistKeepsTheManifestOrder(t *testing.T) {
	c := ChecklistOf(twoAreas(), report.Checklist{})
	if len(c.Areas) != 2 {
		t.Fatalf("areas = %d, want 2", len(c.Areas))
	}
	if c.Areas[0].Name != "Power" || c.Areas[1].Name != "Interfaces" {
		t.Errorf("areas = %q, %q; want Power then Interfaces, the manifest's order", c.Areas[0].Name, c.Areas[1].Name)
	}
	ids := []string{c.Areas[0].Items[0].ID, c.Areas[0].Items[1].ID}
	if ids[0] != "P1" || ids[1] != "P2" {
		t.Errorf("items = %v; want P1 then P2, with the passing item still first", ids)
	}
	if c.Name != "House checklist" || c.Total != 3 || !strings.Contains(c.Summary, "1 pass") {
		t.Errorf("header = %q, %d items, %q; want the manifest's name, 3 items and its tally", c.Name, c.Total, c.Summary)
	}
}

// TestChecklistCarriesEveryFinding exists because the markdown Detail cell caps at three, and its
// comment says ItemResult.Findings keeps the full list. A broad rule fires on hundreds of nets, and
// a page that silently shows three of them is the same false-coverage failure the review layer
// exists to remove.
func TestChecklistCarriesEveryFinding(t *testing.T) {
	var fs []*checkspb.Finding
	for _, n := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		fs = append(fs, netFinding("esd-protection", n, "no protection"))
	}
	doc := &checkspb.CheckResults{Areas: []*checkspb.ReviewArea{{
		Name:  "Interfaces",
		Items: []*checkspb.ReviewItem{reviewItem("I3", "exposed signals have ESD protection", "fail", fs...)},
	}}}
	got := ChecklistOf(doc, report.Checklist{}).Areas[0].Items[0].Evidence
	if len(got) != len(fs) {
		t.Errorf("evidence rows = %d, want %d; the html report must not inherit the markdown cap", len(got), len(fs))
	}
}

// TestChecklistPromisesNoLinkWithoutAMount applies the same rule as `check --server`. A base
// address alone is half the pair, and a link built from the other half missing resolves on nobody's
// server.
func TestChecklistPromisesNoLinkWithoutAMount(t *testing.T) {
	c := ChecklistOf(twoAreas(), report.Checklist{URLBase: "http://localhost:8080"})
	rows := 0
	for _, a := range c.Areas {
		for _, it := range a.Items {
			for _, e := range it.Evidence {
				rows++
				if e.URL != "" {
					t.Errorf("item %s linked to %q with no mount path; a base address alone cannot address a design", it.ID, e.URL)
				}
			}
		}
	}
	if rows != 2 {
		t.Fatalf("evidence rows = %d, want 2, so the loop above checked something", rows)
	}
	linked := ChecklistOf(twoAreas(), report.Checklist{URLBase: "http://localhost:8080", MountPath: "gw/board.edn"})
	e := linked.Areas[0].Items[1].Evidence[0]
	if !strings.Contains(e.URL, "verdict=") {
		t.Errorf("with both halves present the link is %q, want one naming a verdict", e.URL)
	}
}

func renderReview(t *testing.T, links *webapi.ReportLinks) *webapi.RenderReviewReportResponse {
	t.Helper()
	svc := NewReviewService(nil, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "", nil)
	resp, err := svc.RenderReviewReport(context.Background(), &webapi.RenderReviewReportRequest{
		Review: ReviewOf("", twoAreas()), Links: links,
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// The page a review renders to names every item in the manifest's order, with its findings, and is
// named for the design. It needs no store, so an unstored run renders as a stored one does.
func TestRenderReviewReportWritesTheChecklistPage(t *testing.T) {
	resp := renderReview(t, nil)
	page := string(resp.GetContent())
	if resp.GetFilename() != "board.agni-review.html" || !strings.HasPrefix(resp.GetContentType(), "text/html") {
		t.Errorf("file = %q (%q), want board.agni-review.html as html", resp.GetFilename(), resp.GetContentType())
	}
	p1, p2, i1 := strings.Index(page, "every rail carries a bulk capacitor"), strings.Index(page, "every rail carries decoupling"), strings.Index(page, "the CAN interface is terminated")
	if p1 < 0 || p2 < p1 || i1 < p2 {
		t.Errorf("items at %d, %d, %d; want all three, in the manifest's order", p1, p2, i1)
	}
	if !strings.Contains(page, "House checklist") || !strings.Contains(page, "no decoupling") {
		t.Error("the page lacks the checklist's name or a finding's message")
	}
	if strings.Contains(page, "verdict=") {
		t.Error("a page asked for no links links its findings")
	}
}

// Links point at the viewer the caller names, and a reason links were refused is carried into the
// page, since whoever opens the file later never saw it (agni issue 626).
func TestRenderReviewReportLinksOrSaysWhyNot(t *testing.T) {
	page := string(renderReview(t, &webapi.ReportLinks{UrlBase: "http://localhost:8080/", DesignUri: "mount://gw/board.edn"}).GetContent())
	if !strings.Contains(page, "http://localhost:8080/designs/gw/board.edn/view?") || !strings.Contains(page, "abc123") {
		t.Errorf("a linked page does not link into the viewer at the run's revision:\n%s", page)
	}
	withheld := string(renderReview(t, &webapi.ReportLinks{UrlBase: "http://localhost:8080", Withheld: "mount gw is not served there"}).GetContent())
	if !strings.Contains(withheld, "mount gw is not served there") {
		t.Error("a page whose links were refused does not say why")
	}
}

func TestRenderReviewReportNeedsAReview(t *testing.T) {
	svc := NewReviewService(nil, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "", nil)
	if _, err := svc.RenderReviewReport(context.Background(), &webapi.RenderReviewReportRequest{}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("err = %v, want an invalid argument", err)
	}
}
