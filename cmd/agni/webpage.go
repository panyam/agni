package main

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/internal/mounts"
	goal "github.com/panyam/goapplib"
)

// serveApp is the goapplib application context for `agni serve`. It carries the configured
// mounts so pages (and later, page data loaders) can reach them.
type serveApp struct {
	mounts []mounts.Mount
}

// ViewerPage is the server-rendered work page of the web viewer. Its template
// (web/templates/ViewerPage.html) renders the border-layout shell (a file-tree sidebar, the WebGL
// canvas region, and a detail panel) with holes that frontend islands mount into (WS9-002+).
// goapplib maps this type to ViewerPage.html by name.
type ViewerPage struct {
	Title string
}

// Load populates the page before render. The shell is static and per-file data arrives over
// the Connect API, so there is nothing to fetch here yet.
func (p *ViewerPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni viewer"
	return nil, false
}

// BrowsePage is the server-rendered shell of the design browser (WS9-049 phase 2), the file list
// plus a read-only preview of the selected design's first sheet. It is for choosing what to work
// on, so its template has no holes for the presenter, the WebGL canvas, or the checks/query/diff
// panels, and its own bundle (static/browse.js) keeps dockview and the renderer out of the
// download. goapplib maps this type to BrowsePage.html by name.
type BrowsePage struct {
	Title string
}

// Load populates the browse page before render. The shell is static. The mount listing and each
// preview arrive over the Connect API, and which folder is open lives in the URL.
func (p *BrowsePage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni designs"
	return nil, false
}

// DatasheetsPage is the server-rendered shell of the extraction workbench (WS13-006), a separate
// page because it serves the once-per-component author (load a datasheet, select and correct
// regions) rather than someone reading a schematic. Its template (web/templates/DatasheetsPage.html)
// renders a datasheet-tree sidebar and the region viewer hole, and its own bundle
// (static/datasheets.js) keeps the viewer bundle lean. goapplib maps this type to
// DatasheetsPage.html by name.
type DatasheetsPage struct {
	Title string
}

// Load populates the workbench page before render. The shell is static, since a datasheet's doc-IR
// and source PDF arrive over the Connect API and the raw endpoint.
func (p *DatasheetsPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni datasheets"
	return nil, false
}

// LandingPage is the server-rendered shell of "/", the page that routes rather than browses (#318).
// The shell carries the destinations as plain links, so it works with no JavaScript, plus two island
// holes for what this browser opened lately and the designs the server's projects declare by name.
// Its own bundle (static/landing.js) keeps the trees, the renderer, and pdf.js out of the first page
// anyone loads. goapplib maps this type to LandingPage.html by name.
type LandingPage struct {
	Title string
}

// Load populates the landing page before render. Nothing is fetched here, since the recents are
// browser state the server cannot see and the project listing arrives over the Connect API. That
// keeps the shell identical for every visitor and cacheable.
func (p *LandingPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni"
	return nil, false
}

// redirectLegacyFiles permanently redirects the pre-WS9-049 deep-link space to its replacement:
// /files/<mount>/<path> becomes /designs/<mount>/<path>/view, and a folder URL (trailing slash)
// becomes the same folder under /designs/. The sheet and view knobs ride in the query string in
// both spaces, so they carry over untouched. It is a plain handler rather than a proto service
// because it serves no message, and C2 governs the API surface, not HTTP-level redirects.
func redirectLegacyFiles(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/files/")
	target := "/designs/" + rest
	// A folder keeps its trailing slash, which IS the folder marker, and a design gains /view.
	if rest != "" && !strings.HasSuffix(rest, "/") {
		target += viewSegment
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

// newPageApp builds the goapplib App that renders the viewer's pages from templates under
// dir/templates.
func newPageApp(dir string, sa *serveApp) *goal.App[*serveApp] {
	return goal.NewApp(sa, goal.SetupTemplates(filepath.Join(dir, "templates")))
}

// viewSegment terminates a design's work-page URL, the mirror of VIEW_SEGMENT in web/src/router.ts.
// It tells a design apart from a folder without depending on the path's shape, and here it is also
// the routing discriminator (see designsRouter).
const viewSegment = "/view"

// pageHandler renders one goapplib page as a standalone http.Handler. goal.Register only registers
// onto a mux, so the page gets a private mux whose single "/" pattern matches everything reaching
// it. The browse/work split needs this because a ServeMux {path...} wildcard must be the LAST
// segment, so "/designs/{mount}/{rest...}/view" is not writable and one handler has to choose.
func pageHandler[V goal.View[*serveApp]](app *goal.App[*serveApp]) http.Handler {
	return goal.Register[V](app, nil, "/")
}

// designsRouter splits the one /designs/ URL space between its two pages by the trailing verb. A
// design's work page ends in /view, and everything else is a folder, which the browse page renders.
// Folders are the default so that the space root ("/designs/", naming no mount) and a bare mount
// root land on the browser instead of 404ing, matching what router.ts's parseUrl treats as
// addressable.
func designsRouter(browse, work http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, viewSegment) {
			work.ServeHTTP(w, r)
			return
		}
		browse.ServeHTTP(w, r)
	})
}

// registerPages mounts the viewer's server-rendered pages on mux. Routing is server-owned (C11)
// but per-page state lives in the URL, so each shell is identical for every path it serves and the
// frontend reads the URL on load to reopen the design or folder. The /designs/ space carries two
// pages behind one pattern (see designsRouter).
//
// "/" is the landing page and also the catch-all, so a URL matching no other pattern lands on a page
// offering the destinations rather than on an empty file tree (#318).
//
// datasheetsErr is non-nil when the workbench's assets are absent, and /datasheets/ then answers with
// it instead of a page that would fail to load its bundle (agni issue 735).
func registerPages(app *goal.App[*serveApp], mux *http.ServeMux, datasheetsErr error) {
	browse := pageHandler[*BrowsePage](app)
	mux.Handle("/", pageHandler[*LandingPage](app))
	mux.Handle("/designs/", designsRouter(browse, pageHandler[*ViewerPage](app)))
	// The retired /files/ space (WS9-049) redirects rather than 404s, so shared links keep resolving.
	mux.HandleFunc("/files/", redirectLegacyFiles)
	// The extraction workbench (WS13-006) is its own page space. Like the viewer, the shell is
	// identical for every path and per-datasheet state lives in the URL (/datasheets/files/<mount>/
	// <path>), so a refresh or shared link reopens the datasheet.
	if datasheetsErr != nil {
		mux.Handle("/datasheets/", unavailableHandler(datasheetsErr))
		return
	}
	goal.Register[*DatasheetsPage](app, mux, "/datasheets/")
}
