package service

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/encoding/protojson"
)

// A finding's structured datasheet citation reaches the Review as its own message, so a consumer of
// `review --format json` shows the source and can flag a low confidence without parsing the message
// (agni issue 734 moved this guarantee from the hand-rolled encoder to the wire message). A finding
// with no datasheet backing carries none.
func TestAReviewCarriesAFindingsDatasheetCitation(t *testing.T) {
	rep := review.Report{Manifest: "m", Design: "d", Areas: []review.AreaResult{{
		Area: review.Area{Name: "power"},
		Items: []review.ItemResult{{
			Item:    review.Item{ID: "18", Title: "regulator output ratings"},
			Outcome: review.Fail,
			Findings: []check.Finding{
				{Subject: check.Entity{Kind: check.KindComponent, Ref: "U7000"}, Rule: "review/18", Severity: "warning", Message: "IOUT below requirement", DatasheetProv: []*check.DatasheetCitation{{
					Doc: "LMR60410-Q1 (SNAS870B Rev. B)", DocRef: "snas870b", Page: 5,
					Section: "6.3 Recommended Operating Conditions", Method: "hand", Confidence: 1.0,
				}}},
				{Subject: check.Entity{Kind: check.KindComponent, Ref: "U9"}, Rule: "review/other", Severity: "warning", Message: "no datasheet backing"},
			},
		}},
	}}}
	areas := reviewAreaProtos(rep)
	fs := areas[0].GetItems()[0].GetFindings()
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2", len(fs))
	}
	c := fs[0].GetDatasheets()
	if len(c) != 1 || c[0].GetDoc() != "LMR60410-Q1 (SNAS870B Rev. B)" || c[0].GetPage() != 5 || c[0].GetSection() != "6.3 Recommended Operating Conditions" || c[0].GetConfidence() != 1 {
		t.Errorf("the backed finding's citation = %v, want the LMR60410-Q1 page 5 citation", c)
	}
	if len(fs[1].GetDatasheets()) != 0 {
		t.Errorf("the unbacked finding carries citations %v, want none", fs[1].GetDatasheets())
	}
	doc := ReviewOf("", &checkspb.CheckResults{Areas: areas})
	b, err := protojson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"datasheets":[{"doc":"LMR60410-Q1 (SNAS870B Rev. B)"`) {
		t.Errorf("the json omits the citation: %s", b)
	}
	if doc.GetSummary().GetFail() != 1 || doc.GetSummary().GetTotal() != 1 {
		t.Errorf("summary = %v, want one item, failing", doc.GetSummary())
	}
}

// The Review carries the FULL finding list for a failing item, including findings past the cap the
// markdown Detail cell applies, each with its subject and source file, so tooling loses nothing.
func TestAReviewCarriesEveryFindingUncapped(t *testing.T) {
	var fs []check.Finding
	for i := 0; i < 10; i++ {
		fs = append(fs, check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: "NET_" + string(rune('A'+i))}, Rule: "esd-protection", Message: "no ESD device", Prov: &ir.Provenance{SourceFile: "evt.edn"}})
	}
	rep := review.Report{Manifest: "t", Design: "evt", Areas: []review.AreaResult{{Area: review.Area{Name: "A"}, Items: []review.ItemResult{
		{Item: review.Item{ID: "esd", Title: "ESD"}, Outcome: review.Fail, Findings: fs},
	}}}}
	areas := reviewAreaProtos(rep)
	if len(areas) != 1 || len(areas[0].GetItems()) != 1 {
		t.Fatalf("shape: %v", areas)
	}
	got := areas[0].GetItems()[0].GetFindings()
	if len(got) != 10 {
		t.Errorf("want all 10 findings uncapped, got %d", len(got))
	}
	if last := got[len(got)-1]; last.GetSubject().GetRef() != "NET_J" || last.GetProvenance().GetSourceFile() != "evt.edn" {
		t.Errorf("last finding = %v, want subject NET_J from evt.edn", last)
	}
}

// An item bound to nothing converts with no binding, as a client building the manifest leaves it, so
// the CLI's run (manifest read from YAML) and a served one snapshot the same checklist identically.
func TestAnUnboundItemConvertsWithNoBinding(t *testing.T) {
	p := ManifestProto(review.Manifest{Name: "m", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "hand", Title: "checked by hand"},
		{ID: "r", Title: "a rule", Binding: review.Binding{Rule: "i2c-pull-up"}},
	}}}})
	items := p.GetAreas()[0].GetItems()
	if items[0].Binding != nil {
		t.Errorf("the unbound item carries binding %v, want none", items[0].Binding)
	}
	if items[1].GetBinding().GetRule() != "i2c-pull-up" {
		t.Errorf("the rule item lost its binding: %v", items[1].GetBinding())
	}
}
