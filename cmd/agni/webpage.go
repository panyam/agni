package main

import (
	"net/http"
	"path/filepath"

	goal "github.com/panyam/goapplib"
	"github.com/panyam/agni/internal/mounts"
)

// serveApp is the goapplib application context for `agni serve`. It carries the configured
// mounts so pages (and later, page data loaders) can reach them.
type serveApp struct {
	mounts []mounts.Mount
}

// ViewerPage is the single server-rendered page of the web viewer. Its template
// (web/templates/ViewerPage.html) renders the border-layout shell: a file-tree sidebar, the
// WebGL canvas region, and a detail panel. The interactive regions are holes that frontend
// islands mount into (WS9-002+). goapplib maps this type to ViewerPage.html by name.
type ViewerPage struct {
	Title string
}

// Load populates the page before render. The shell is static and per-file data arrives over
// the Connect API, so there is nothing to fetch here yet.
func (p *ViewerPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni viewer"
	return nil, false
}

// DatasheetsPage is the server-rendered shell of the extraction workbench (WS13-006), a page
// distinct from the viewer: the workbench serves the once-per-component author (load a datasheet,
// select and correct regions), a different persona and runtime than the schematic viewer. Its
// template (web/templates/DatasheetsPage.html) renders a datasheet-tree sidebar and the region
// viewer hole; its own bundle (static/datasheets.js) keeps the viewer bundle lean. goapplib maps
// this type to DatasheetsPage.html by name.
type DatasheetsPage struct {
	Title string
}

// Load populates the workbench page before render. The shell is static; a datasheet's doc-IR and
// source PDF arrive over the Connect API and the raw endpoint, so there is nothing to fetch here.
func (p *DatasheetsPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni datasheets"
	return nil, false
}

// newPageApp builds the goapplib App that renders the viewer's pages from templates under
// dir/templates.
func newPageApp(dir string, sa *serveApp) *goal.App[*serveApp] {
	return goal.NewApp(sa, goal.SetupTemplates(filepath.Join(dir, "templates")))
}

// registerPages mounts the viewer's server-rendered pages on mux. The same shell serves both
// "/" and the deep-link space "/files/<mount>/<path>?sheet=…": routing is server-owned (C11), but
// per-file state lives in the URL, so the frontend reads the URL on load and reopens that file.
// The shell is identical for every path (the file's data still arrives over the Connect API), so
// a refresh or a shared link lands on the same design/sheet instead of the empty root.
func registerPages(app *goal.App[*serveApp], mux *http.ServeMux) {
	goal.Register[*ViewerPage](app, mux, "/")
	goal.Register[*ViewerPage](app, mux, "/files/")
	// The extraction workbench (WS13-006) is its own page space. Like the viewer, the shell is
	// identical for every path and per-datasheet state lives in the URL (/datasheets/files/<mount>/
	// <path>), so a refresh or shared link reopens the datasheet.
	goal.Register[*DatasheetsPage](app, mux, "/datasheets/")
}
