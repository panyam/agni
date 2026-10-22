package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// manifestFakeLoader is fakeLoader with the Manifest method ReviewLoader adds, so a review reads the
// same in-memory design and geometry a check run does.
type manifestFakeLoader struct{ fakeLoader }

func (manifestFakeLoader) Manifest(context.Context, artifact.URI) (review.Manifest, error) {
	return review.Manifest{}, nil
}

// TestReviewFindingsCarryTheirSheets pins agni issue 1132. A review finding names the sheets its
// subject is drawn on, the same ids CheckDesign gives the same finding, so a click in the Review
// panel opens that sheet, and an undrawn subject carries the reason the Checks panel shows.
func TestReviewFindingsCarryTheirSheets(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R9"}, {RefDes: "R7"}},
	}
	// R9 is placed on S2 and R7 nowhere, so one finding badges a sheet and the other gives a reason.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "S1"},
		{Id: "S2", Placements: []*geom.SymbolPlacement{{RefDes: "R9", Transform: &geom.Transform{Origin: &geom.Point{X: 5, Y: 5}}}}},
	}}
	l := manifestFakeLoader{fakeLoader{design: d, geom: g}}
	svc := NewReviewService(l, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "", nil)
	man := &checkspb.ReviewManifest{Name: "sheets", Areas: []*checkspb.ManifestArea{{
		Name:  "Parts",
		Items: []*checkspb.ManifestItem{{Id: "p1", Title: "every part connected", Binding: &checkspb.ItemBinding{Rule: "unconnected-component"}}},
	}}}
	resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		Manifest:     man,
		Snapshot:     snapOf(t, svc.Snapshots(), "mount://m/x.edn"),
		ValidateOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*checkspb.Finding{}
	for _, a := range resp.GetResults().GetAreas() {
		for _, it := range a.GetItems() {
			for _, f := range it.GetFindings() {
				got[f.GetSubject().GetRef()] = f
			}
		}
	}
	if len(got) != 2 {
		t.Fatalf("review findings = %d subjects, want R9 and R7: %v", len(got), got)
	}
	if s := got["R9"].GetSheets(); !slicesEqual(s, []string{"S2"}) {
		t.Errorf("R9 sheets = %v, want [S2]", s)
	}
	if r := got["R7"]; len(r.GetSheets()) != 0 || r.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_NO_GEOMETRY {
		t.Errorf("R7 sheets = %v reason = %v, want none and NO_GEOMETRY", r.GetSheets(), r.GetLocateReason())
	}
}
