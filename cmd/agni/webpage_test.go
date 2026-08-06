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
		`id="sheet-tabs"`,           // WS9-049: hole for the visited-sheet tab strip
		"/static/app.js",            // the bundle reference
		"Agni viewer",              // page title from Load
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

// TestWorkPageServesDesignsSpace asserts the WS9-049 work-page URL space renders the shell, AND
// that it does so because /designs/ is registered rather than because the root pattern catches
// everything. The pattern check is the load-bearing half: "/" is a catch-all, so a 200 alone would
// pass even with no /designs/ registration at all, and phase 2 needs the space to be a real route
// it can hang the browse page off.
func TestWorkPageServesDesignsSpace(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	for _, path := range []string{
		"/designs/",                              // the space root
		"/designs/corpus/",                       // a mount root (folder form)
		"/designs/corpus/boards/b.kicad_sch/view", // a design (work page)
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if _, pattern := mux.Handler(req); pattern != "/designs/" {
			t.Errorf("GET %s matched pattern %q, want %q", path, pattern, "/designs/")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), `id="dock"`) {
			t.Errorf("GET %s did not render the viewer shell", path)
		}
	}
}

// TestLegacyFilesRedirect pins the WS9-049 migration promise: a /files/ deep link shared before
// the split still resolves, landing on the same design and sheet in the new space.
func TestLegacyFilesRedirect(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	for _, tc := range []struct{ from, want string }{
		{"/files/corpus/boards/b.kicad_sch", "/designs/corpus/boards/b.kicad_sch/view"},
		// The knobs ride in the query string in both spaces, so they carry over verbatim.
		{"/files/corpus/b.eds?sheet=root&mode=svg", "/designs/corpus/b.eds/view?sheet=root&mode=svg"},
		// A folder keeps its trailing slash and does NOT gain /view.
		{"/files/corpus/boards/", "/designs/corpus/boards/"},
		{"/files/corpus/", "/designs/corpus/"},
		{"/files/", "/designs/"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.from, nil))
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s = %d, want 301", tc.from, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Errorf("GET %s redirected to %q, want %q", tc.from, got, tc.want)
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
