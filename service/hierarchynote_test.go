package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// hierSentence is what every surface says about readers/edif/testdata/hier.edn, whose top cell
// places one instance of the sub-cell SUB, which holds one instance of its own.
const hierSentence = "hier.edn is hierarchical and only its top cell was read. 1 sub-cell holding 1 instance was not extracted (SUB: 1). Counts and checks cover the top cell alone."

// manifestLoader is a fakeLoader that also answers the review port's Manifest, which no test here
// asks for since the manifest travels as a value.
type manifestLoader struct{ fakeLoader }

func (manifestLoader) Manifest(context.Context, artifact.URI) (review.Manifest, error) {
	return review.Manifest{}, nil
}

// readFixture reads an EDIF fixture through the Loader, so the diagnostic is the reader's own and not
// a hand-built one.
func readFixture(t *testing.T, name string) fakeLoader {
	t.Helper()
	d, err := (&formats.Loader{}).ReadDesign("../readers/edif/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return fakeLoader{design: d, geom: twoSheetGeom(), hash: "sha256:abc"}
}

// surfaces renders every surface that states what a read left out: the design summary the viewer
// opens with, the check report page and results document, and the review's document and checklist
// page. Each is built from the run its caller holds, as the CLI and the viewer build them.
type surfaces struct {
	designNote   string
	checkRunLen  int
	checkDocLen  int
	checkPage    string
	reviewDocLen int
	reviewPage   string
}

func renderSurfaces(t *testing.T, name string) surfaces {
	t.Helper()
	ctx := context.Background()
	l := readFixture(t, name)
	uri := "mount://m/" + name
	var out surfaces

	dsvc := NewDesignService(l, noNative{}, render.Style{}, nil)
	d, err := dsvc.GetDesign(ctx, &webapi.GetDesignRequest{Snapshot: snapOf(t, dsvc.Snapshots(), uri)})
	if err != nil {
		t.Fatal(err)
	}
	out.designNote = d.GetHierarchyNote()

	csvc := NewCheckService(l, check.DefaultCatalog(), nil, "", nil, nil)
	csnap := snapOf(t, csvc.Snapshots(), uri)
	run, err := csvc.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: csnap})
	if err != nil {
		t.Fatal(err)
	}
	out.checkRunLen = len(run.GetUnexpandedHierarchy())
	page, err := csvc.RenderCheckReport(ctx, &webapi.RenderCheckReportRequest{Snapshot: csnap, Run: run, Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML})
	if err != nil {
		t.Fatal(err)
	}
	out.checkPage = string(page.GetContent())
	js, err := csvc.RenderCheckReport(ctx, &webapi.RenderCheckReportRequest{Snapshot: csnap, Run: run, Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_RESULTS_JSON})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := results.Parse(js.GetContent())
	if err != nil {
		t.Fatal(err)
	}
	out.checkDocLen = len(doc.GetDesign().GetUnexpandedHierarchy())

	m, err := review.Load(strings.NewReader("name: hierarchy\nareas:\n  - name: Nets\n    items:\n      - {id: N1, title: no net has one pin, rule: single-pin-net}\n"))
	if err != nil {
		t.Fatal(err)
	}
	rsvc := NewReviewService(manifestLoader{l}, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "", nil)
	rv, err := rsvc.CreateReview(ctx, &webapi.CreateReviewRequest{Snapshot: snapOf(t, rsvc.Snapshots(), uri), Manifest: ManifestProto(m), ValidateOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	out.reviewDocLen = len(rv.GetResults().GetDesign().GetUnexpandedHierarchy())
	rpage, err := rsvc.RenderReviewReport(ctx, &webapi.RenderReviewReportRequest{Review: rv})
	if err != nil {
		t.Fatal(err)
	}
	out.reviewPage = string(rpage.GetContent())
	return out
}

// TestEverySurfaceSaysWhatAHierarchicalReadLeftOut holds the design summary, the check report and the
// checklist page to the sentence the CLI prints (agni issue 717). Before it, a hierarchical .edn said
// so on stderr alone, and a report someone opened later read as the whole design. The flat design is
// the control, so a note rendered unconditionally fails too.
func TestEverySurfaceSaysWhatAHierarchicalReadLeftOut(t *testing.T) {
	hier := renderSurfaces(t, "hier.edn")
	if hier.designNote != hierSentence {
		t.Errorf("GetDesign hierarchy_note = %q, want %q", hier.designNote, hierSentence)
	}
	if hier.checkRunLen != 1 || hier.checkDocLen != 1 || hier.reviewDocLen != 1 {
		t.Errorf("unexpanded_hierarchy entries: check run %d, check document %d, review document %d; want 1 each",
			hier.checkRunLen, hier.checkDocLen, hier.reviewDocLen)
	}
	want := `<div class="read-limit">` + hierSentence + `</div>`
	if !strings.Contains(hier.checkPage, want) {
		t.Errorf("the check report page does not state what the read left out; want %s", want)
	}
	if !strings.Contains(hier.reviewPage, want) {
		t.Errorf("the checklist page does not state what the read left out; want %s", want)
	}
	// The note qualifies the counts, so it comes before them.
	for name, page := range map[string]string{"check": hier.checkPage, "checklist": hier.reviewPage} {
		if strings.Index(page, want) > strings.Index(page, `<div class="stats">`) {
			t.Errorf("the %s page states the read limit after the counts it qualifies", name)
		}
	}

	flat := renderSurfaces(t, "basic.edn")
	if flat.designNote != "" || flat.checkRunLen != 0 || flat.checkDocLen != 0 || flat.reviewDocLen != 0 {
		t.Errorf("a flat design reports a read limit: %+v", flat)
	}
	for name, page := range map[string]string{"check": flat.checkPage, "checklist": flat.reviewPage} {
		if strings.Contains(page, "is hierarchical") || strings.Contains(page, `class="read-limit"`) {
			t.Errorf("the %s page of a flat design states a read limit", name)
		}
	}
}
