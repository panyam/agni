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

	// A work-page URL, not "/": since WS9-049 phase 2 the root serves the browse page, so the
	// viewer shell is reached by addressing a design.
	const workURL = "/designs/corpus/boards/b.kicad_sch/view"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, workURL, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200; body:\n%s", workURL, rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`id="compare-picker"`,       // WS9-049 phase 3: the compare picker modal
		`id="compare-tree"`,         // its file-tree island hole
		`id="view"`,                 // canvas region
		`id="dock"`,                 // WS9-021: dockview mounts here
		`id="panel-park"`,           // WS9-021: server-rendered holes park here
		`data-dock-panel="details"`, // WS9-021: the details hole the dock adopts
		`data-dock-panel="checks"`,  // WS9: the merged checks panel hole (findings + report)
		`data-component`,            // islands mount by this marker
		`id="sheet-tabs"`,           // WS9-049: hole for the visited-sheet tab strip
		"/static/app.js",            // the bundle reference
		"Agni viewer",               // page title from Load
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
	// The Files dock panel is retired (WS9-049 phase 3). Its HOLE is what matters here: the dock
	// adopts panels by finding a data-dock-panel element, so a stale hole would let a saved layout
	// resurrect the panel even with the registry entry deleted.
	for _, deny := range []string{
		`data-dock-panel="files"`,
		`id="file-tree"`,
	} {
		if strings.Contains(body, deny) {
			t.Errorf("rendered page still contains retired Files panel markup %q", deny)
		}
	}
}

// TestWorkPageServesDesignsSpace asserts the WS9-049 work-page URL space renders the shell, AND
// that it does so because /designs/ is registered rather than because the root pattern catches
// everything. The pattern check is the load-bearing half: "/" is a catch-all, so a 200 alone would
// pass even with no /designs/ registration at all, and phase 2 hangs the browse page off this
// same pattern (the /view suffix is what splits them, since a ServeMux pattern cannot put a
// wildcard segment before a literal one).
func TestWorkPageServesDesignsSpace(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	for _, path := range []string{
		"/designs/",        // the space root
		"/designs/corpus/", // a mount root (folder form)
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
		}
	}
}

// TestDesignsSpaceSplitsBrowseFromWork is the phase-2 contract: one registered pattern, two
// different shells, chosen by the /view suffix. Each case asserts BOTH that the expected shell
// rendered and that the other one did not, because both pages descend from BasePage and share
// enough markup that a one-sided check would pass if the dispatcher sent every request to the
// same page.
// "/" is the landing page rather than the design browser, and it is also the catch-all: a URL
// matching no other pattern lands somewhere that offers the destinations instead of on an empty
// tree. The browser did not move, so the same test asserts /designs/ still serves it.
func TestRootServesLandingPage(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	for _, path := range []string{"/", "/not-a-page"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
		body := rec.Body.String()
		// Both destinations are plain server-rendered links, so the page routes with no JavaScript.
		for _, want := range []string{`href="/designs/"`, `href="/datasheets/"`, `id="landing-recents"`, "/static/landing.js"} {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s missing %q", path, want)
			}
		}
		// The browser's own markers must be absent, or "/" is still serving the tree.
		for _, deny := range []string{`id="browse-tree"`, "/static/browse.js", "/static/app.js"} {
			if strings.Contains(body, deny) {
				t.Errorf("GET %s unexpectedly contains %q (landing page is serving another shell)", path, deny)
			}
		}
	}
}

// Every page a person can land on offers a way back to the landing page. This is a cross-page
// omission test, and it is written that way because the omission is invisible from inside any one
// page: the viewer shipped with no route out of itself, and a design URL is the one people share, so
// it is the page most likely to be somebody's first (PR 318 deferred it, this closes it).
//
// A page is checked for the crumb's own markup rather than a bare href="/", which any stylesheet or
// icon reference would satisfy.
func TestEveryPageOffersAWayHome(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	for _, path := range []string{
		"/designs/",                          // the folder browser
		"/designs/m/boards/b.kicad_sch/view", // a design's work page
		"/datasheets/files/m/vendor/x.pdf",   // the extraction workbench
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `class="ld-home" href="/"`) {
			t.Errorf("GET %s has no link back to the landing page", path)
		}
	}
}

func TestDesignsSpaceSplitsBrowseFromWork(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	// browse markers / work markers. The bundle reference is the sharpest discriminator: a page
	// loading app.js IS the viewer, whatever else it renders.
	browseMarkers := []string{`id="browse-tree"`, `id="browse-preview"`, "/static/browse.js"}
	workMarkers := []string{`id="dock"`, `id="panel-park"`, "/static/app.js"}

	for _, tc := range []struct {
		path       string
		want, deny []string
	}{
		{"/designs/", browseMarkers, workMarkers},               // the space root
		{"/designs/corpus/", browseMarkers, workMarkers},        // a mount root
		{"/designs/corpus/boards/", browseMarkers, workMarkers}, // a subfolder
		{"/designs/corpus/b.kicad_sch/view", workMarkers, browseMarkers},
		{"/designs/corpus/boards/b.kicad_sch/view?sheet=root", workMarkers, browseMarkers},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", tc.path, rec.Code)
			continue
		}
		body := rec.Body.String()
		for _, want := range tc.want {
			if !strings.Contains(body, want) {
				t.Errorf("GET %s missing %q", tc.path, want)
			}
		}
		for _, deny := range tc.deny {
			if strings.Contains(body, deny) {
				t.Errorf("GET %s unexpectedly contains %q (wrong page served)", tc.path, deny)
			}
		}
	}
}

// TestBrowsePageOmitsAnalysisChrome pins the ticket's structural promise: the browse preview is
// read-only, so the page must not ship the viewer's analysis surfaces. These are template-level
// holes, so their absence is what keeps the presenter, WebGL canvas, and checks machinery from
// ever mounting (the islands resolve their holes by id at boot and bail when they are missing).
func TestBrowsePageOmitsAnalysisChrome(t *testing.T) {
	mux := http.NewServeMux()
	registerPages(newPageApp(filepath.Join("..", "..", "web"), &serveApp{}), mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/designs/", nil))

	body := rec.Body.String()
	for _, deny := range []string{
		`id="view"`,     // the WebGL canvas
		`id="controls"`, // the render-mode / layout control bar
		`id="findings"`, // the checks panel
		`id="rules"`,    // the rule catalog
		`id="query-panel"`,
		`id="diff-bar"`,   // the comparison chrome (phase 3 initiates a diff, it does not host one)
		`id="sheet-tabs"`, // sheet navigation belongs to the work page
	} {
		if strings.Contains(body, deny) {
			t.Errorf("browse page unexpectedly contains %q", deny)
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
		`id="ds-tree"`,             // hole for the datasheet tree island
		`id="ds-view"`,             // hole for the region viewer island
		`data-component="ds-tree"`, // islands mount by this marker
		"/static/datasheets.js",    // the workbench's own bundle (not the viewer's app.js)
		"Agni datasheets",          // page title from Load
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered datasheets page missing %q", want)
		}
	}
}
