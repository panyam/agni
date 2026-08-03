package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewerPageRendersShell(t *testing.T) {
	// newPageApp joins dir + "/templates"; "../../web" resolves to the repo's web/templates
	// relative to this package (cmd/agni), the go-test working directory.
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="file-tree"`,            // hole for the WS9-002 file tree
		`id="view"`,                 // canvas region
		`id="dock"`,                 // WS9-021: dockview mounts here
		`id="panel-park"`,           // WS9-021: server-rendered holes park here
		`data-dock-panel="details"`, // WS9-021: the details hole the dock adopts
		`data-dock-panel="checks"`,  // WS9: the merged checks panel hole (findings + report)
		`data-component`,            // islands mount by this marker
		"/static/app.js",            // the bundle reference
		"Agni viewer",              // page title from Load
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

// TestDatasheetsPageRendersShell asserts the extraction workbench page (WS13-006) serves its own
// shell at /datasheets/: the tree and region-viewer holes, its own bundle, and its title.
func TestDatasheetsPageRendersShell(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/datasheets/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /datasheets/ = %d, want 200; body:\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="ds-tree"`,               // hole for the datasheet tree island
		`id="ds-view"`,               // hole for the region viewer island
		`data-component="ds-tree"`,   // islands mount by this marker
		"/static/datasheets.js",      // the workbench's own bundle (not the viewer's app.js)
		"Agni datasheets",           // page title from Load
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered datasheets page missing %q", want)
		}
	}
}
