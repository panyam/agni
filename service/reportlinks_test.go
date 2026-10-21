package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// linkNames is what a report's caller can name a design by, and the viewer path its links must open.
// The folder and a declared companion both resolve to the entry; an undeclared sibling is read as
// named, so its links keep its own path (agni issue 1120).
var linkNames = []struct {
	name, uri, want string
}{
	{"the design's folder", "mount://m/d", "/designs/m/d/board.edn/view?"},
	{"the entry", "mount://m/d/board.edn", "/designs/m/d/board.edn/view?"},
	{"a declared companion", "mount://m/d/board.eds", "/designs/m/d/board.edn/view?"},
	{"an undeclared sibling", "mount://m/d/other.edn", "/designs/m/d/other.edn/view?"},
}

func TestTheChecklistPageLinksTheEntryWhateverTheCallerNamed(t *testing.T) {
	svc := NewReviewService(nil, nil, check.DefaultCatalog(), nil, nil, testReviewEnv, "", &ProjectResolver{Store: declaringStore{design: withCompanion()}})
	for _, tc := range linkNames {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.RenderReviewReport(context.Background(), &webapi.RenderReviewReportRequest{
				Review: ReviewOf("", twoAreas()),
				Links:  &webapi.ReportLinks{UrlBase: "http://localhost:8080", DesignUri: tc.uri},
			})
			if err != nil {
				t.Fatal(err)
			}
			assertLinks(t, string(resp.GetContent()), tc.want)
		})
	}
}

func TestTheVerdictPageLinksTheEntryWhateverTheCallerNamed(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{{
		Name:        "SDA",
		Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
	}}}
	svc := NewCheckService(fakeLoader{design: d, hash: "sha256:abc"}, check.DefaultCatalog(), nil, "", nil,
		&ProjectResolver{Store: declaringStore{design: withCompanion()}})
	snap := snapOf(t, svc.Snapshots(), "mount://m/d/board.edn")
	run, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snap, Rules: []string{"i2c-pull-up"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.GetVerdicts()) == 0 {
		t.Fatal("the run has no verdicts, so its page has nothing to link")
	}
	for _, tc := range linkNames {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := svc.RenderCheckReport(context.Background(), &webapi.RenderCheckReportRequest{
				Snapshot: snap, Rules: []string{"i2c-pull-up"}, Run: run,
				Links:  &webapi.ReportLinks{UrlBase: "http://localhost:8080", DesignUri: tc.uri},
				Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertLinks(t, string(resp.GetContent()), tc.want)
		})
	}
}

// assertLinks checks that the page links into the viewer, and that every link opens want.
func assertLinks(t *testing.T, page, want string) {
	t.Helper()
	links := 0
	for _, part := range strings.Split(page, `href="http://localhost:8080`)[1:] {
		links++
		if !strings.HasPrefix(part, want) {
			t.Errorf("a link opens %q, want %q", part[:min(len(part), len(want)+20)], want)
		}
	}
	if links == 0 {
		t.Fatal("the page carries no link into the viewer, so nothing above was checked")
	}
}
