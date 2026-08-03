package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skhttp "github.com/panyam/servicekit/http"
	"github.com/spf13/cobra"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/param"
	"github.com/panyam/agni/render"
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
	var paramsDir, profilePath, intentPath string
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
			ckPath, ckHandler := webapiconnect.NewCheckServiceHandler(server.NewCheck(service.NewCheckService(loader, check.DefaultCatalog(), specs)))
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
			reviewCatalog, reviewProfiles, err := composeReviewInputs(profilePath, intentPath)
			if err != nil {
				return err
			}
			rvPath, rvHandler := webapiconnect.NewReviewServiceHandler(server.NewReview(service.NewReviewService(loader, reviewCatalog, reviewProfiles, specs)))
			mux.Handle(rvPath, rvHandler)
			mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(dir, "static")))))
			// The rule-doc explainer diagrams (embedded beside each rule's markdown, WS3-025) are
			// served read-only so the rules/expectations panels resolve their relative image refs
			// (WS9-030). Images only, from the embed FS only — no filesystem access.
			mux.Handle("/rule-docs/", http.StripPrefix("/rule-docs/", check.RuleDocImageHandler()))
			// The per-relation fact-doc schematic cards (WS14-005), same read-only image-only posture
			// as the rule docs, so the query panel resolves a relation Detail's image refs.
			mux.Handle("/relation-docs/", http.StripPrefix("/relation-docs/", check.RelationDocImageHandler()))
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
	c.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations composed into the ReviewService catalog")
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
