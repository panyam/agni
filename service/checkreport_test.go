package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/results"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// reportRun runs the catalog over a design with one unpulled I2C net, so the run has a finding, a
// failing verdict and passing ones, and returns the service that ran it.
func reportRun(t *testing.T) (*CheckService, *webapi.CheckDesignResponse) {
	t.Helper()
	d := &ir.Design{Nets: []*ir.Net{{
		Name:        "SDA",
		Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
	}}}
	svc := NewCheckService(fakeLoader{design: d, hash: "sha256:abc"}, check.DefaultCatalog(), nil, "", nil, nil).
		WithEnv(ReviewEnv{ProducerVersion: "v9.9.9-test", Intent: true})
	run, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/board.edn", Rules: []string{"i2c-pull-up"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GetFindings()) == 0 || len(run.GetVerdicts()) == 0 {
		t.Fatalf("the run found %d findings and %d verdicts, so the report has nothing to carry", len(run.GetFindings()), len(run.GetVerdicts()))
	}
	return svc, run
}

// TestRenderCheckReportWritesTheResultsDocument holds the JSON form to the document `agni check
// --results-out` writes: the producer, the design's hash, the selected catalog and the run's findings.
func TestRenderCheckReportWritesTheResultsDocument(t *testing.T) {
	svc, run := reportRun(t)
	resp, err := svc.RenderCheckReport(context.Background(), &webapi.RenderCheckReportRequest{
		Uri: "mount://m/board.edn", Rules: []string{"i2c-pull-up"}, Run: run,
		Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_RESULTS_JSON,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetContentType() != "application/json" || resp.GetFilename() != "board.agni-check.json" {
		t.Errorf("content type %q and filename %q, want application/json and board.agni-check.json", resp.GetContentType(), resp.GetFilename())
	}
	doc, err := results.Parse(resp.GetContent())
	if err != nil {
		t.Fatalf("the document does not parse as a results document: %v", err)
	}
	m := doc.GetMeta()
	if m.GetSchema() != results.Schema || m.GetProducer() != results.Producer || m.GetProducerVersion() != "v9.9.9-test" || m.GetCreatedAt() == "" {
		t.Errorf("meta = %+v, want the results schema and producer, version v9.9.9-test and a timestamp", m)
	}
	if doc.GetDesign().GetSource() != "mount://m/board.edn" || doc.GetDesign().GetContentHash() != "sha256:abc" {
		t.Errorf("design = %+v, want the request's uri and the loader's hash", doc.GetDesign())
	}
	if len(doc.GetCatalog()) != 1 || doc.GetCatalog()[0].GetName() != "i2c-pull-up" {
		t.Errorf("catalog = %v, want the one selected rule", doc.GetCatalog())
	}
	if !doc.GetRun().GetIntent() {
		t.Error("the env's intent flag did not reach the run's provenance")
	}
	if len(doc.GetFindings()) != len(run.GetFindings()) {
		t.Errorf("the document carries %d findings, the run %d", len(doc.GetFindings()), len(run.GetFindings()))
	}
}

// TestRenderCheckReportWritesTheVerdictPage holds the HTML form to the run's verdicts, and its links
// to what the caller asked for.
func TestRenderCheckReportWritesTheVerdictPage(t *testing.T) {
	svc, run := reportRun(t)
	render := func(links *webapi.ReportLinks) string {
		t.Helper()
		resp, err := svc.RenderCheckReport(context.Background(), &webapi.RenderCheckReportRequest{
			Uri: "mount://m/board.edn", Rules: []string{"i2c-pull-up"}, Run: run, Links: links,
			Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(resp.GetContentType(), "text/html") || resp.GetFilename() != "board.agni-check.html" {
			t.Errorf("content type %q and filename %q", resp.GetContentType(), resp.GetFilename())
		}
		return string(resp.GetContent())
	}
	page := render(nil)
	if !strings.Contains(page, "i2c-pull-up") || !strings.Contains(page, "SDA") {
		t.Error("the page does not name the rule and the subject the run failed")
	}
	if strings.Contains(page, "/designs/") {
		t.Error("a page asked for no links carries one")
	}
	linked := render(&webapi.ReportLinks{UrlBase: "https://example.org/agni/demo/", DesignUri: "mount://gateway/designs/gateway"})
	if !strings.Contains(linked, "https://example.org/agni/demo/designs/gateway/designs/gateway/view") {
		t.Error("a page asked for links against a base carries no link into that viewer")
	}
	withheld := render(&webapi.ReportLinks{Withheld: "this design was opened from your machine"})
	if !strings.Contains(withheld, "this design was opened from your machine") {
		t.Error("a page whose links were withheld does not say why")
	}
}

func TestRenderCheckReportRefusesAnIncompleteRequest(t *testing.T) {
	svc, run := reportRun(t)
	for name, req := range map[string]*webapi.RenderCheckReportRequest{
		"no run":    {Uri: "mount://m/board.edn", Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML},
		"no format": {Uri: "mount://m/board.edn", Run: run},
	} {
		if _, err := svc.RenderCheckReport(context.Background(), req); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}
