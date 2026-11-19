package main

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	goal "github.com/panyam/goapplib"
)

// serveApp is the goapplib application context for `agni serve`. It holds the configured mounts. It
// also holds where the datasheets workbench is served, when the workbench is hosted at all
// (--datasheets-url).
type serveApp struct {
	mounts        *mountset.Table
	datasheetsURL string
	// engine is which engine the viewer runs a design on, "server" or "wasm" (--engine, agni issue
	// 853). The page carries it, and a `?engine=` on the URL overrides it.
	engine string
	// wasmMaxBytes is the largest design file set the page analyses in the browser (--wasm-max-bytes,
	// agni issue 852). A bigger design goes to the server engine.
	wasmMaxBytes int64
	// embedOrigins lists other origins whose pages may drive the viewer over postMessage, besides
	// the page's own (--embed-origin, agni issue 1152).
	embedOrigins []string
	// base is the path the pages are served under, "/" for `agni serve` and the static demo's prefix
	// for `agni site` (agni issue 856).
	base string
	// static marks pages written as plain files for a host with no agni server behind it.
	static bool
	// viewerRooms says the server lets agents drive its viewer tabs (--viewer-rooms, agni issue 1154).
	viewerRooms bool
	// siteSeeds are the example boards the static demo's landing page lists.
	siteSeeds []siteSeed
	// socialImage is the absolute image URL for the link card of the static demo's landing page.
	socialImage string
	// designs looks up a design by name, so /designs/<id>/view opens its current files (agni issue
	// 1697). Nil on a server that holds no designs by name.
	designs service.DesignLookup
}

// ViewerPage is the server-rendered work page of the web viewer. Its template is
// web/templates/ViewerPage.html, and it renders the border-layout shell. The shell holds a file-tree
// sidebar, the WebGL canvas region and a detail panel, and frontend islands mount into its holes
// (WS9-002+). goapplib maps this type to ViewerPage.html by name.
type ViewerPage struct {
	Title string
	// Engine is the engine the page analyses its design on, written into the shell for the bundle
	// to read. The mount in the URL says where the design's files live and this says who reads them,
	// so a design keeps one URL under either (agni issue 853).
	Engine string
	// WasmMaxBytes is the largest design file set the page analyses in the browser. The page learns a
	// design's size from ListDesignFiles before reading anything, and sends a bigger one to the
	// server engine, saying why.
	WasmMaxBytes int64
	// Base is the path the page is served under, ending in "/". Every asset and link in the shell
	// goes through it. If a page is served under a prefix and its assets are root-relative, it
	// returns 200 and renders with no CSS (agni issue 856).
	Base string
	// Host is "static" when the page is a plain file with no agni server behind it, so the bundle
	// reads every design's files as static files and answers everything in the worker.
	Host string
	// Build names the agni that wrote the page. The Load page keys its record of a read design on it
	// and the viewer checks it, since another build may read a file differently and keeps a store of
	// its own (agni issue 1072).
	Build string
	// EmbedOrigins is a space-separated list of other origins whose pages may drive this viewer
	// over postMessage (agni issue 1152). Empty allows only the page's own origin.
	EmbedOrigins string
	// ViewerRooms is "on" when the server lets agents drive its viewer tabs (--viewer-rooms, agni
	// issue 1154), so the page asks ViewerRoomService for its room and offers its tools there. Empty
	// on every other server and on a static page.
	ViewerRooms string
}

// Load populates the page before render. The shell is static and per-file data arrives over
// the Connect API, so there is nothing to fetch here yet.
func (p *ViewerPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni viewer"
	p.fill(app.Context)
	return nil, false
}

// fill sets the shell fields the viewer and the Load page both carry.
func (p *ViewerPage) fill(a *serveApp) {
	p.Engine = a.engine
	if p.Engine == "" {
		p.Engine = engineServer
	}
	p.WasmMaxBytes = a.wasmMaxBytes
	if p.WasmMaxBytes <= 0 {
		p.WasmMaxBytes = defaultWasmMaxBytes
	}
	p.Base = a.basePath()
	if a.static {
		p.Host = "static"
	}
	p.Build = version.Version()
	p.EmbedOrigins = strings.Join(a.embedOrigins, " ")
	if a.viewerRooms {
		p.ViewerRooms = "on"
	}
}

// LoadPage reads a design into the engine's store before the viewer opens it. Files are also
// dropped on it (agni issue 1072). It carries the viewer's shell fields. It starts the engine that
// the viewer will ask, so it must name the same build. goapplib maps this type to LoadPage.html by
// name.
type LoadPage struct {
	ViewerPage
}

// Load populates the Load page before render. Like the viewer's, the shell is the same for every
// design, and the bundle reads the design from the URL.
func (p *LoadPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni: loading a design"
	p.fill(app.Context)
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
	p.Title = "Agni files"
	return nil, false
}

// LandingPage is the server-rendered shell of "/", the page that routes rather than browses (#318).
// The shell carries the destinations as plain links, so it works with no JavaScript. It also has
// two island holes. One lists what this browser opened lately, and the other lists the designs that
// the server's projects declare by name. Its own bundle (static/landing.js) keeps the trees, the
// renderer, and pdf.js out of the first page anyone loads. goapplib maps this type to
// LandingPage.html by name.
type LandingPage struct {
	Title string
	// DatasheetsURL is the workbench's base URL, and the card and datasheet recents render only when
	// it is set, since the workbench is a separate service (agni issue 744).
	DatasheetsURL string
}

// Load populates the landing page before render. Nothing is fetched here, since the recents are
// browser state the server cannot see and the project listing arrives over the Connect API. That
// keeps the shell identical for every visitor and cacheable.
func (p *LandingPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni"
	p.DatasheetsURL = app.Context.datasheetsURL
	return nil, false
}

// LibraryPage is the server-rendered shell of the store's pages (agni issues 1719, 1697). /designs/
// lists the designs a server holds, stored and mounted, and holds the form to make one, and
// /designs/<id> shows one design's artifacts and reports. /projects/ and /packs/ do the same for
// projects and packs. The page's bundle (static/library.js) reads which from the URL, so the shell is
// the same for every path. goapplib maps this type to LibraryPage.html by name.
type LibraryPage struct {
	Title string
}

// Load populates the library page before render. The designs arrive over the Connect API.
func (p *LibraryPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	switch {
	case strings.HasPrefix(r.URL.Path, projectsPrefix):
		p.Title = "Agni projects"
	case strings.HasPrefix(r.URL.Path, packsPrefix):
		p.Title = "Agni packs"
	default:
		p.Title = "Agni designs"
	}
	return nil, false
}

// designsPrefix is the designs' URL space (agni issue 1719), the mirror of DESIGNS_PREFIX in
// web/src/libraryupload.ts. /designs/ lists every design the server holds and /designs/<id> shows
// one. A design id holds no slash, so a longer path is a link from before the mount browser moved
// out of /designs/.
const designsPrefix = "/designs/"

// projectsPrefix and packsPrefix are the store's other two URL spaces (agni issue 1697), the mirrors
// of PROJECTS_PREFIX and PACKS_PREFIX in web/src/storepages.ts. Each lists its resources and shows
// one by id, through the designs page's shell.
const (
	projectsPrefix = "/projects/"
	packsPrefix    = "/packs/"
)

// mountsPrefix is the mount browser's URL space (agni issue 1734), the mirror of the prefix
// web/src/router.ts builds. /mounts/<mount>/<dir>/ is a folder, and /view or /load ends a file's
// work or Load page.
const mountsPrefix = "/mounts/"

// oldFilesPrefix is where the mount browser lived between agni issues 1719 and 1734.
const oldFilesPrefix = "/files/"

// toMounts redirects permanently to rest under /mounts/, query and all, so a shared link keeps
// opening what it opened. A redirect is a plain handler rather than a proto service because it
// serves no message, and C2 governs the API surface, not HTTP-level redirects.
func toMounts(w http.ResponseWriter, r *http.Request, rest string) {
	target := mountsPrefix + rest
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710 -- target always starts /mounts/, so the redirect stays on this site
}

// oldFilesSpace redirects every /files/ link to the same path under /mounts/.
func oldFilesSpace() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		toMounts(w, r, strings.TrimPrefix(r.URL.Path, oldFilesPrefix))
	})
}

// designsSpace serves /designs/ and /designs/<id> with the designs page. /designs/<id>/view opens
// the design's current files in the viewer, read by the design's name (agni issue 1697). Any other
// longer path is a link from when /designs/ held the mount browser, /designs/<mount>/<path>/view and
// the rest, and it redirects to the same path under /mounts/ in one hop.
func designsSpace(designs http.Handler, lookup service.DesignLookup) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, designsPrefix)
		if !strings.Contains(rest, "/") {
			designs.ServeHTTP(w, r)
			return
		}
		// An old mount-browser link names a mount and a path, so its part before /view holds a slash.
		if id, ok := strings.CutSuffix(rest, viewSegment); ok && lookup != nil && !strings.Contains(id, "/") {
			viewDesign(w, r, lookup, id)
			return
		}
		toMounts(w, r, rest)
	})
}

// viewDesign redirects /designs/<id>/view to the viewer on the design's entry, with `design=` naming
// the design so the page reads it by name, and the request's own query, such as `pack=`, kept. The
// redirect is not permanent, since the entry moves to a new revision's folder on every write.
func viewDesign(w http.ResponseWriter, r *http.Request, lookup service.DesignLookup, id string) {
	name := service.StandaloneDesignName(id)
	d, err := lookup.Design(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	u, err := mounts.ParseURI(d.GetEntryUri())
	if err != nil || d.GetEntryUri() == "" {
		http.Error(w, name+" has no file a reader opens yet; put its netlist, schematic or board first", http.StatusNotFound)
		return
	}
	segs := []string{url.PathEscape(u.Mount)}
	for _, p := range strings.Split(u.Path, "/") {
		segs = append(segs, url.PathEscape(p))
	}
	q := r.URL.Query()
	q.Set("design", name)
	http.Redirect(w, r, mountsPrefix+strings.Join(segs, "/")+viewSegment+"?"+q.Encode(), http.StatusFound) // #nosec G710 -- the target always starts /mounts/, so the redirect stays on this site
}

// newPageApp builds the goapplib App that renders the viewer's pages from templates under
// dir/templates.
func newPageApp(dir string, sa *serveApp) *goal.App[*serveApp] {
	return goal.NewApp(sa, goal.SetupTemplates(filepath.Join(dir, "templates")))
}

// viewSegment terminates a design's work-page URL, the mirror of VIEW_SEGMENT in web/src/router.ts.
// It tells a design apart from a folder without depending on the path's shape, and here it is also
// the routing discriminator (see mountsRouter).
const viewSegment = "/view"

// loadSegment terminates a design's Load page URL, the mirror of LOAD_SEGMENT in web/src/router.ts
// (agni issue 1072). The browser mount's Load page, where files are dropped, names no design and ends
// in "/load/".
const loadSegment = "/load"

// pageHandler renders one goapplib page as a standalone http.Handler. goal.Register only registers
// onto a mux, so the page gets a private mux whose single "/" pattern matches everything reaching
// it. The browse/work split needs this because a ServeMux {path...} wildcard must be the LAST
// segment, so "/mounts/{mount}/{rest...}/view" is not writable and one handler has to choose.
func pageHandler[V goal.View[*serveApp]](app *goal.App[*serveApp]) http.Handler {
	return goal.Register[V](app, nil, "/")
}

// mountsRouter splits the one /mounts/ URL space between its pages by the trailing verb. A
// design's work page ends in /view, its Load page in /load (or /load/, where files are dropped), and
// everything else is a folder, which the browse page renders. Folders are the default, so the space
// root and a bare mount root open the browser instead of 404ing. The space root is "/mounts/" and
// names no mount. This matches what router.ts's parseUrl treats as addressable.
func mountsRouter(browse, work, load http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, viewSegment):
			work.ServeHTTP(w, r)
		case strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), loadSegment):
			load.ServeHTTP(w, r)
		default:
			browse.ServeHTTP(w, r)
		}
	})
}

// registerPages mounts the viewer's server-rendered pages on mux. Routing is server-owned (C11)
// but per-page state lives in the URL, so each shell is identical for every path it serves and the
// frontend reads the URL on load to reopen the design or folder. The /mounts/ space carries three
// pages behind one pattern (see mountsRouter), and /designs/ the designs page (see designsSpace).
//
// "/" is the landing page and also the catch-all. So a URL matching no other pattern opens a page
// offering the destinations rather than an empty file tree (#318).
func registerPages(app *goal.App[*serveApp], mux *http.ServeMux) {
	mux.Handle("/", pageHandler[*LandingPage](app))
	mux.Handle(oldFilesPrefix, oldFilesSpace())
	mux.Handle(mountsPrefix, mountsRouter(pageHandler[*BrowsePage](app), pageHandler[*ViewerPage](app), pageHandler[*LoadPage](app)))
	mux.Handle(designsPrefix, designsSpace(pageHandler[*LibraryPage](app), app.Context.designs))
	mux.Handle(projectsPrefix, pageHandler[*LibraryPage](app))
	mux.Handle(packsPrefix, pageHandler[*LibraryPage](app))
}

// basePath is the path the pages are served under, "/" unless set, always ending in "/".
func (a *serveApp) basePath() string {
	if a.base == "" {
		return "/"
	}
	return strings.TrimSuffix(a.base, "/") + "/"
}
