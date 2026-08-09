package main

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skhttp "github.com/panyam/servicekit/http"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/relations"
	"github.com/panyam/agni/stdlib/rules/builtin"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// serveCmd hosts the web viewer over HTTP for local development. It serves three things on
// one listener: the server-rendered pages (goapplib/templar, the border-layout shell) at
// `/`, the esbuild bundle and other frontend assets under `/static/`, and the WS9 web API
// as Connect handlers under their proto service paths. This replaces `npx serve .`: the
// browser tooling (esbuild bundle, buf gen, vitest/playwright) stays npm, but serving is
// the Go binary, one fewer dependency in the dev loop.
//
// The page shell is server-rendered and routing is server-owned (CONSTRAINTS C11); the API
// is proto-defined and served over Connect (C2), so the same contract drives the Go server
// and the TS client with no hand-written JSON.
func serveCmd() *cobra.Command {
	var addr string
	var mountSpecs []string
	var nativeTools []string
	var pdf2docCmd string
	var theme string
	var paramsDir, profilePath, intentPath, conventions string
	c := &cobra.Command{
		Use:   "serve [dir]",
		Short: "Serve the web viewer (static assets + Connect API) over HTTP for local development",
		Long: "serve hosts the server-rendered viewer shell, the esbuild bundle under /static/,\n" +
			"and the Connect web API on one listener. Build the bundle first (pnpm build in web/).\n" +
			"Pass --mount name=path (repeatable) to expose design folders to the file browser.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			style, ok := render.Themes[theme]
			if !ok {
				return fmt.Errorf("unknown --theme %q (have: %s)", theme, strings.Join(themeNames(), ", "))
			}
			dir := "web"
			if len(args) == 1 {
				dir = args[0]
			}
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				return fmt.Errorf("%q is not a directory", dir)
			}
			if err := checkWebAssets(dir); err != nil {
				return err
			}
			mounts, err := mounts.Parse(mountSpecs)
			if err != nil {
				return err
			}
			// The datasheet knowledge base for the params panel (WS9-035), loaded once at startup.
			// Absent --params leaves it nil, so the params RPC returns no joined specs (never an
			// error) — the same optional posture as the CLI's --params (main.go readModelWithParams).
			var specs param.ParamProvider
			if paramsDir != "" {
				set, err := param.LoadSet(os.DirFS(paramsDir))
				if err != nil {
					return fmt.Errorf("--params %q: %w", paramsDir, err)
				}
				specs = set
			}

			mux := http.NewServeMux()
			// Connect handlers register under their fully-qualified service path
			// (/agni.v1.webapi.WorkspaceService/). Static assets (the esbuild bundle) live
			// under /static/. The goapplib page catches the rest at "/". The services are the
			// transport-neutral internal/service implementations; internal/server wraps them
			// for Connect (C13).
			loader := &osLoader{mounts: mounts, loader: newLoader()}
			enabledNative := map[string]bool{}
			for _, t := range nativeTools {
				enabledNative[t] = true
			}
			nativeR := &osNative{mounts: mounts, enabled: enabledNative, cache: native.NewCache()}
			wsPath, wsHandler := webapiconnect.NewWorkspaceServiceHandler(server.NewWorkspace(service.NewWorkspaceService(&osWorkspace{mounts: mounts})))
			mux.Handle(wsPath, wsHandler)
			dsPath, dsHandler := webapiconnect.NewDesignServiceHandler(server.NewDesign(service.NewDesignService(loader, nativeR, style)))
			mux.Handle(dsPath, dsHandler)
			// Overlay interface profiles are startup config for the WHOLE server, not just the review
			// surface (WS3-048). Loaded once here and composed into both the CheckService catalog below
			// and the ReviewService catalog further down, so a customer's overlay-declared interface is
			// visible wherever rules run — the web check panel and ListRules included. Before this, a
			// profile authored in an overlay fired on the CLI and was invisible in the viewer, which is
			// where the interface-aware view (WS9-041) presents it.
			overlayProfiles, err := loadOverlayProfiles(profilePath)
			if err != nil {
				return err
			}
			checkCatalog := serveCheckCatalog(overlayProfiles)
			noteSupersededRules(cmd.ErrOrStderr(), checkCatalog)
			ckPath, ckHandler := webapiconnect.NewCheckServiceHandler(server.NewCheck(service.NewCheckService(loader, checkCatalog, specs)))
			mux.Handle(ckPath, ckHandler)
			diffPath, diffHandler := webapiconnect.NewDiffServiceHandler(server.NewDiff(service.NewDiffService(loader)))
			mux.Handle(diffPath, diffHandler)
			dtPath, dtHandler := webapiconnect.NewDatasheetServiceHandler(server.NewDatasheet(service.NewDatasheetService(&osDocLoader{mounts: mounts}, &osPartSpecStore{mounts: mounts}, &osDocExtractor{mounts: mounts, cmd: strings.Fields(pdf2docCmd)}, &osAnnotationStore{mounts: mounts})))
			mux.Handle(dtPath, dtHandler)
			qPath, qHandler := webapiconnect.NewQueryServiceHandler(server.NewQuery(service.NewQueryService(loader, specs)))
			mux.Handle(qPath, qHandler)
			// ReviewService (WS9-047): the served analogue of `agni review`. The overlay knobs are
			// startup config (like --params above), composed ONCE into the catalog + profile index the
			// service holds, so a RunReview request only names the manifest, design(s), and board. A bad
			// --profile-path/--intent-path fails serve startup rather than every review request.
			reviewCatalog, reviewProfiles, err := composeReviewInputsFrom(overlayProfiles, intentPath)
			if err != nil {
				return err
			}
			// --conventions is the DEPLOYMENT default for this server's project (WS3-102): composed into
			// the catalog here and installed as the process-level vocabulary, which is the one legitimate
			// use of a process-wide install (startup, before any request, never mutated after). A request
			// that names its own conventions_path overrides both per request, with its lexicon travelling
			// with that read; that is why the override is safe under concurrency (WS3-106).
			if conventions != "" {
				cfg, err := naming.Load(conventions)
				if err != nil {
					return err
				}
				if err := naming.ApplyLexicon(cfg); err != nil {
					return err
				}
				if len(cfg.Rules) > 0 {
					src, err := naming.Source(cfg)
					if err != nil {
						return err
					}
					reviewCatalog = check.CatalogWith(src)
				}
			}
			rvPath, rvHandler := webapiconnect.NewReviewServiceHandler(server.NewReview(service.NewReviewService(loader, reviewCatalog, reviewProfiles, specs)))
			mux.Handle(rvPath, rvHandler)
			mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(dir, "static")))))
			// The rule-doc explainer diagrams (embedded beside each rule's markdown, WS3-025) are
			// served read-only so the rules/expectations panels resolve their relative image refs
			// (WS9-030). Images only, from the embed FS only — no filesystem access. The built-in and
			// intent rule docs live in separate embed FSes (WS3-093), but a rule's Detail references its
			// card as images/<name> under this one route regardless of source, so the two handlers are
			// composed (first non-404 wins; card basenames are unique across sources).
			mux.Handle("/rule-docs/", http.StripPrefix("/rule-docs/",
				firstImageHandler(builtin.RuleDocImageHandler(), intent.RuleDocImageHandler())))
			// The per-relation fact-doc schematic cards (WS14-005), same read-only image-only posture
			// as the rule docs, so the query panel resolves a relation Detail's image refs.
			mux.Handle("/relation-docs/", http.StripPrefix("/relation-docs/", relations.RelationDocImageHandler()))
			// The datasheets workbench renders the source PDF in the browser (pdf.js), so its raw
			// bytes are served from the mounts. A more-specific prefix than the /datasheets/ page,
			// so ServeMux routes /datasheets/raw/... here and the page space elsewhere.
			mux.Handle("/datasheets/raw/", http.StripPrefix("/datasheets/raw/", rawDatasheetHandler(mounts)))
			registerPages(newPageApp(dir, &serveApp{mounts: mounts}), mux)

			srv := &http.Server{Addr: addr, Handler: mux}
			fmt.Fprintf(os.Stderr, "serving %s at http://%s/ with %d mount(s) (Ctrl-C to stop)\n", dir, addr, len(mounts))
			// servicekit drains in-flight requests on SIGINT/SIGTERM instead of dropping them.
			return skhttp.ListenAndServeGraceful(srv)
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	c.Flags().StringArrayVar(&mountSpecs, "mount", nil, "expose a design folder as name=path (repeatable)")
	c.Flags().StringArrayVar(&nativeTools, "enable-native", nil, "allow a native golden renderer by tool name, e.g. kicad-cli (repeatable; off by default)")
	c.Flags().StringVar(&pdf2docCmd, "pdf2doc", "", "command that derives a datasheet's doc-IR, e.g. \"python3 tools/pdf2doc/pdf2doc.py\"; empty disables the /datasheets Extract (first pass) action")
	c.Flags().StringVar(&theme, "theme", "default", "render palette: "+strings.Join(themeNames(), " | ")+" (applies to SVG and WebGL)")
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos; enables the datasheet params panel")
	c.Flags().StringVar(&conventions, "conventions", "", "an operator naming-convention config (YAML) used as this server's default: its rules join the catalog and its lexicon becomes the default naming vocabulary. A request may name its own instead")
	c.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations composed into the catalog every rule-running surface uses: the check panel and ListRules as well as reviews")
	c.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration composed into the ReviewService catalog so intent-bound review items resolve")
	return c
}

// checkWebAssets verifies dir is the web-assets directory (the viewer template plus the built
// esbuild bundle) before the server starts, so a misdirected `serve <design-folder>` fails upfront
// with guidance instead of a cryptic template-not-found on the first request. The positional arg
// is the assets dir (defaults to "web"); design folders are exposed with --mount, not this arg.
func checkWebAssets(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "templates", "ViewerPage.html")); err != nil {
		return fmt.Errorf("%q has no templates/ViewerPage.html: the positional arg is the web-assets dir (defaults to \"web\"), not a folder to browse; mount design folders with --mount name=path", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "app.js")); err != nil {
		return fmt.Errorf("%q has no static/app.js: build the frontend bundle first with `cd %s && pnpm build`", dir, dir)
	}
	// The extraction workbench (WS13-006) is a second server-rendered page with its own bundle;
	// both must be present or /datasheets 500s/404s on the first request.
	if _, err := os.Stat(filepath.Join(dir, "templates", "DatasheetsPage.html")); err != nil {
		return fmt.Errorf("%q has no templates/DatasheetsPage.html (the datasheets workbench page)", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "datasheets.js")); err != nil {
		return fmt.Errorf("%q has no static/datasheets.js: build the frontend bundle first with `cd %s && pnpm build`", dir, dir)
	}
	return nil
}

// firstImageHandler composes several rule-source image handlers into one, serving the response of the
// first that does not 404 (WS3-093). Each source's rule-doc images live in its own embed FS behind its
// own handler, but the web resolves every rule's card under the single /rule-docs/ route, so a request
// is tried against each in turn and the first hit is copied through. Card basenames are unique across
// sources, so order does not affect which card a request resolves to. Each handler is image-only and
// 404s cleanly on a miss, so the buffered probe never has a side effect to undo.
func firstImageHandler(handlers ...http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range handlers {
			rec := &bufferedResponse{header: http.Header{}}
			h.ServeHTTP(rec, r)
			if rec.status == http.StatusNotFound {
				continue
			}
			maps.Copy(w.Header(), rec.header)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			w.WriteHeader(rec.status)
			w.Write(rec.body.Bytes())
			return
		}
		http.NotFound(w, r)
	})
}

// bufferedResponse captures a handler's status, headers, and body in memory so firstImageHandler can
// discard a 404 and try the next source without having written anything to the client.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header { return b.header }
func (b *bufferedResponse) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

// themeNames returns the available --theme values in sorted order, for flag help and the
// unknown-theme error.
func themeNames() []string {
	names := make([]string, 0, len(render.Themes))
	for n := range render.Themes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// serveCheckCatalog is the rule catalog the served CheckService runs: the built-ins plus any overlay
// interface profiles this server was started with (WS3-048). It exists as a named function so the
// invariant is testable without standing up a listener — the regression it guards is a future edit
// handing NewCheckService a bare DefaultCatalog again, which compiles, serves, and silently omits a
// customer's profiles from the check panel and ListRules.
//
// The namespace is "profile-overlay", matching `agni check --profile-path`, so a rule has the same
// name in the viewer as on the CLI.
func serveCheckCatalog(overlay []profiles.Profile) *check.Catalog {
	if len(overlay) == 0 {
		return check.DefaultCatalog()
	}
	return check.CatalogWith(profiles.Source("profile-overlay", overlay))
}
