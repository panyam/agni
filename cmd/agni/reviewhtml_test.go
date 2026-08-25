package main

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/review"
)

// item is a one-line ItemResult for the tests below.
func item(id, title string, o review.Outcome, fs ...check.Finding) review.ItemResult {
	return review.ItemResult{Item: review.Item{ID: id, Title: title}, Outcome: o, Findings: fs}
}

func finding(rule, net, msg string) check.Finding {
	return check.Finding{Rule: rule, Subject: check.Entity{Kind: check.KindNet, Ref: net}, Message: msg}
}

// twoAreas is a report whose areas and items are deliberately NOT in alphabetical or worst-first
// order, so a renderer that sorts is visible.
func twoAreas() review.Report {
	return review.Report{
		Manifest: "House checklist",
		Areas: []review.AreaResult{
			{Area: review.Area{Name: "Power"}, Items: []review.ItemResult{
				item("P1", "every rail carries a bulk capacitor", review.Pass),
				item("P2", "every rail carries decoupling", review.Fail, finding("decoupling-present", "V12", "no decoupling")),
			}},
			{Area: review.Area{Name: "Interfaces"}, Items: []review.ItemResult{
				item("I1", "the CAN interface is terminated", review.Fail, finding("can-term", "CANH", "not terminated")),
			}},
		},
	}
}

// TestChecklistKeepsTheManifestOrder: the check report sorts rules worst-first because nobody
// authored that order. A checklist's order is the team's. An item that has been P1 in their process
// for years has to stay first, and an area must not move because the board got worse this week.
func TestChecklistKeepsTheManifestOrder(t *testing.T) {
	c := buildChecklist(twoAreas(), rpt.Checklist{})
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
}

// TestChecklistCarriesEveryFinding: the markdown Detail cell caps at three, and its comment says the
// web surface is where the full list lives. A broad rule fires on hundreds of nets, and a page that
// silently shows three of them is the same false-coverage failure the review layer exists to remove.
func TestChecklistCarriesEveryFinding(t *testing.T) {
	var fs []check.Finding
	for _, n := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		fs = append(fs, finding("esd-protection", n, "no protection"))
	}
	r := review.Report{Areas: []review.AreaResult{{
		Area:  review.Area{Name: "Interfaces"},
		Items: []review.ItemResult{item("I3", "exposed signals have ESD protection", review.Fail, fs...)},
	}}}
	got := buildChecklist(r, rpt.Checklist{}).Areas[0].Items[0].Evidence
	if len(got) != len(fs) {
		t.Errorf("evidence rows = %d, want %d; the html report must not inherit the markdown cap", len(got), len(fs))
	}
}

// TestChecklistPromisesNoLinkWithoutAMount: same rule as `check --url-base`. A base address alone is
// half the pair, and a link built from the other half missing resolves on nobody's server.
func TestChecklistPromisesNoLinkWithoutAMount(t *testing.T) {
	c := buildChecklist(twoAreas(), rpt.Checklist{URLBase: "http://localhost:8080"})
	for _, a := range c.Areas {
		for _, it := range a.Items {
			for _, e := range it.Evidence {
				if e.URL != "" {
					t.Errorf("item %s linked to %q with no mount path; a base address alone cannot address a design", it.ID, e.URL)
				}
			}
		}
	}
	linked := buildChecklist(twoAreas(), rpt.Checklist{URLBase: "http://localhost:8080", MountPath: "gw/board.edn"})
	e := linked.Areas[0].Items[1].Evidence[0]
	if !strings.Contains(e.URL, "verdict=") {
		t.Errorf("with both halves present the link is %q, want one naming a verdict", e.URL)
	}
}
