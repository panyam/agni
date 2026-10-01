package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skhttp "github.com/panyam/servicekit/http"
	"github.com/spf13/cobra"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/datasheet/param"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/relations"
	"github.com/panyam/agni/stdlib/rules/builtin"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// serveCmd hosts the web viewer over HTTP for local development. One listener serves the
// server-rendered pages (goapplib/templar) at `/`, the esbuild bundle under `/static/`, and the
// WS9 web API as Connect handlers under their proto service paths. The browser tooling stays npm;
// only serving is the Go binary.
//
// Routing is server-owned (CONSTRAINTS C11) and the API is proto-defined over Connect (C2), so
// one contract drives the Go server and the TS client.
func serveCmd() *cobra.Command {
	var addr string
	var mountRoot string
	var nativeTools []string
	var pdf2docCmd string
	var theme string
	var paramsDir, profilePath, intentPath, conventions string
	var reviewStorePath string
	var webDir string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the web viewer (static assets + Connect API) over HTTP for local development",
		Long: "serve hosts the server-rendered viewer shell, the esbuild bundle under /static/,\n" +
			"and the Connect web API on one listener. Build the bundle first (pnpm build in web/).\n" +
			"Pass --mount name=path (repeatable) to expose design folders to the file browser.\n" +
			"The viewer's own assets come from --web-dir, defaulting to ./web. With no web dir named\n" +
			"anywhere and no ./web, it serves the API alone, which is what an installed binary has.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runViewer(cmd, viewerOpts{
				addr: addr, webDir: webDir, mountRoot: mountRoot, nativeTools: nativeTools,
				pdf2docCmd: pdf2docCmd, theme: theme, paramsDir: paramsDir,
				profilePath: profilePath, intentPath: intentPath, conventions: conventions,
				reviewStorePath: reviewStorePath,
			})
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	c.Flags().StringVar(&webDir, "web-dir", "",
		"directory holding the viewer's OWN assets (templates/ and the built static/*.js), not a folder "+
			"of designs to browse: mount those with --mount name=path. Defaults to "+defaultWebDir+
			", which is where a repo checkout keeps them, then to web_dir in the nearest agni.yaml, then to "+
			envWebDir+". An installed binary run from a design directory has no relative answer, which is "+
			"what the last two are for")
	c.Flags().StringVar(&mountRoot, "mount-root", "", "expose every subdirectory of this path as a mount named after it, so folders can be bind-mounted in without a --mount flag each; an explicit --mount of the same name wins, and a missing root yields no mounts rather than an error")
	c.Flags().StringArrayVar(&nativeTools, "enable-native", nil, "allow a native golden renderer by tool name, e.g. kicad-cli (repeatable; off by default)")
	c.Flags().StringVar(&pdf2docCmd, "pdf2doc", "", "command that derives a datasheet's doc-IR, e.g. \"python3 tools/pdf2doc/pdf2doc.py\"; empty disables the /datasheets Extract (first pass) action")
	c.Flags().StringVar(&theme, "theme", "default", "render palette: "+strings.Join(themeNames(), " | ")+" (applies to SVG and WebGL)")
	c.Flags().StringVar(&paramsDir, "params", "", "directory of seeded PartSpec textprotos; enables the datasheet params panel")
	c.Flags().StringVar(&conventions, "conventions", "", "an operator naming-convention config (YAML) used as this server's DEFAULT: its rules join the catalog every rule-running surface uses, and its lexicon becomes the default naming vocabulary. A request may carry its own, which REPLACES this one for that request (both halves); reusing this config's name is fine and is the natural way to refine it")
	c.Flags().StringVar(&profilePath, "profile-path", "", "directory of YAML interface-profile declarations composed into the catalog every rule-running surface uses")
	c.Flags().StringVar(&intentPath, "intent-path", "", "a YAML design-intent declaration composed into the catalog every rule-running surface uses, so intent-bound review items resolve and intent rules appear in the check panel")
	c.Flags().StringVar(&reviewStorePath, "review-store", "", "a WRITABLE directory that stored review runs are kept in, created if absent; in a container, mount a volume here (docker run -v agni-reviews:/var/lib/agni/reviews --review-store /var/lib/agni/reviews). It is deliberately separate from the read-only design mounts. Without it the review resource methods report that this server stores no reviews. Runs saved here are visible to every client of this server; there is no per-user separation yet")
	return c
}

// viewerOpts is everything the viewer server is built from, so `serve` and `open` compose the same
// process from different inputs.
//
// `serve` fills every field from its flags. `open` fills three and leaves the rest zero, since it
// offers one design, no deployment config and no review store.
type viewerOpts struct {
	addr        string
	webDir      string
	mountRoot   string
	nativeTools []string
	pdf2docCmd  string
	theme       string
	paramsDir   string
	profilePath string
	intentPath  string
	conventions string

	reviewStorePath string
	// listener, when non-nil, is a listener the caller already BOUND, and the server serves on it
	// instead of binding addr itself. `--server self` holds the port from the moment the flag is
	// parsed, so a taken port fails before the command does its work and nothing can take the port
	// between the check and the serve.
	listener net.Listener
	// extraMounts are mounts the caller composed itself, merged with the declared and discovered ones.
	// `open` uses it to serve the mount it minted for the design named on the command line.
	extraMounts []mounts.Mount
	// banner replaces the default "serving ... at ..." line when non-nil. `open` prints the design's
	// own URL instead, so it lands on the design rather than a browse tree.
	banner func(urls []string, mountCount int)

	// requireViewer refuses to start without the viewer's assets, for `open`, which exists to show a
	// page. `serve` leaves it false and may run the API alone (resolveServeAssets).
	requireViewer bool
}

// runViewer builds and runs the viewer server for both `serve` and `open`.
func runViewer(cmd *cobra.Command, o viewerOpts) error {
	addr, webDir, mountRoot := o.addr, o.webDir, o.mountRoot
	nativeTools, pdf2docCmd, theme := o.nativeTools, o.pdf2docCmd, o.theme
	paramsDir, profilePath, intentPath, conventions := o.paramsDir, o.profilePath, o.intentPath, o.conventions
	reviewStorePath := o.reviewStorePath
	if theme == "" {
		theme = "default"
	}
	style, ok := render.Themes[theme]
	if !ok {
		return fmt.Errorf("unknown --theme %q (have: %s)", theme, strings.Join(themeNames(), ", "))
	}
	// The viewer's assets come from --web-dir, never a positional argument, since a positional
	// invited passing a DESIGN folder; checkWebAssets still guards that mistake (#468).
	var assets webAssets
	if o.requireViewer {
		dir, source, err := resolveWebAssets(webDir, os.Getenv)
		if err != nil {
			return err
		}
		assets = webAssets{dir: dir, source: source, viewer: true, datasheetsErr: checkDatasheetAssets(dir)}
	} else {
		var err error
		if assets, err = resolveServeAssets(webDir, os.Getenv); err != nil {
			return err
		}
	}
	dir, source := assets.dir, assets.source
	// Narrated only for the ENVIRONMENT. applyEnvConfig already names the agni.yaml it read and the
	// serving line prints the directory, but nothing else reports an AGNI_WEB_DIR exported into a
	// shell months ago.
	if source == envWebDir {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: serving web assets from %s (%s).\n", dir, source)
	}
	explicit, err := mounts.Parse(cliMountSpecs)
	if err != nil {
		return err
	}
	// --mount-root is the container's zero-flag path. Every subdirectory under it becomes a mount
	// named after itself, so `-v ~/boards:/workspace/boards` needs no --mount. Explicit --mount
	// values win on a name collision (see mounts.Merge). Empty disables discovery, as a local
	// `make serve` wants.
	var discovered []mounts.Mount
	if mountRoot != "" {
		if discovered, err = mounts.Discover(mountRoot); err != nil {
			return err
		}
	}
	// A caller-composed mount is layered in last, so `open`'s minted mount sits alongside anything a
	// flag or an agni.yaml declared.
	mounts := mounts.Merge(discovered, append(explicit, o.extraMounts...))
	// The datasheet knowledge base for the params panel (WS9-035), loaded once at startup.
	// Absent --params leaves it nil, so the params RPC returns no joined specs and never an
	// error, like the CLI's --params (main.go readModelWithParams).
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
	// (/agni.v1.webapi.WorkspaceService/). Static assets live under /static/, and the goapplib
	// page catches the rest at "/". The services are the transport-neutral service package
	// implementations, which internal/server wraps for Connect (C13).
	loader := &osLoader{mounts: mounts, loader: newLoader()}
	// Tier-1 fallback, as in resolveWebDir. The flag wins OUTRIGHT, so an operator who named the
	// renderers answers for the whole set and agni.yaml cannot widen it.
	if len(nativeTools) == 0 {
		nativeTools = envConfigNativeTools
	}
	enabledNative := map[string]bool{}
	for _, t := range nativeTools {
		enabledNative[t] = true
	}
	nativeR := &osNative{mounts: mounts, enabled: enabledNative, cache: native.NewCache()}
	wsPath, wsHandler := webapiconnect.NewWorkspaceServiceHandler(server.NewWorkspace(service.NewWorkspaceService(&osWorkspace{mounts: mounts})))
	mux.Handle(wsPath, wsHandler)
	// ProjectService (agni issue 170) resolves the project/design descriptors in the mounts and
	// needs no flag. A mount with no descriptors resolves to nothing, so one project's config never
	// reaches another project's design.
	//
	// Each mount becomes one tree in the filesystem-backed store. This is the ONLY place the serve
	// wiring knows projects live in directories, so swapping in another service.ProjectStore is
	// this line alone.
	projectStore := projects.NewFSStore(projectTrees(mounts)...)
	// One resolver for every rule-running surface. Per-design config reaches a run through this,
	// and the startup flags below are only the DEFAULT for a design with no project (agni issue 173).
	projectResolver := &service.ProjectResolver{Store: projectStore, Config: &osProjectConfig{mounts: mounts}}
	prPath, prHandler := webapiconnect.NewProjectServiceHandler(server.NewProject(service.NewProjectService(projectStore)))
	mux.Handle(prPath, prHandler)
	dsPath, dsHandler := webapiconnect.NewDesignServiceHandler(server.NewDesign(service.NewDesignService(loader, nativeR, style, projectResolver)))
	mux.Handle(dsPath, dsHandler)
	// --conventions is the DEPLOYMENT default for this server's project (WS3-102). Its lexicon is
	// installed process-wide, at startup and never mutated after (C22). Its RULES join the catalog
	// inside serveRuleServices. A request that names its own conventions REPLACES both halves for
	// that request, with that lexicon travelling with the read (WS3-106, WS3-124).
	var conventionCfg *configpb.NamingConvention
	if conventions != "" {
		cfg, err := naming.Load(conventions)
		if err != nil {
			return err
		}
		if err := naming.ApplyLexicon(cfg); err != nil {
			return err
		}
		conventionCfg = cfg
	}
	// --review-store is the writable volume stored runs live in, separate from the read-only design
	// mounts. Absent, the review resource methods report that they are not configured rather than
	// discarding runs.
	var reviewStore service.ReviewStore
	if reviewStorePath != "" {
		st, err := newOSReviewStore(reviewStorePath)
		if err != nil {
			return err
		}
		reviewStore = st
	}
	checkSvc, reviewSvc, err := serveRuleServices(loader, reviewStore, specs, profilePath, intentPath, conventionCfg, projectResolver, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	ckPath, ckHandler := webapiconnect.NewCheckServiceHandler(server.NewCheck(checkSvc))
	mux.Handle(ckPath, ckHandler)
	diffPath, diffHandler := webapiconnect.NewDiffServiceHandler(server.NewDiff(service.NewDiffService(loader, projectResolver)))
	mux.Handle(diffPath, diffHandler)
	dtPath, dtHandler := webapiconnect.NewDatasheetServiceHandler(server.NewDatasheet(service.NewDatasheetService(&osDocLoader{mounts: mounts}, &osPartSpecStore{mounts: mounts}, &osDocExtractor{mounts: mounts, cmd: strings.Fields(pdf2docCmd)}, &osAnnotationStore{mounts: mounts})))
	mux.Handle(dtPath, dtHandler)
	qPath, qHandler := webapiconnect.NewQueryServiceHandler(server.NewQuery(service.NewQueryService(loader, specs, projectResolver)))
	mux.Handle(qPath, qHandler)
	// ReviewService (WS9-047) is the served `agni review`, built with the CheckService above from
	// one composed catalog.
	rvPath, rvHandler := webapiconnect.NewReviewServiceHandler(server.NewReview(reviewSvc))
	mux.Handle(rvPath, rvHandler)
	if assets.viewer {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(dir, "static")))))
	}
	// The rule-doc explainer diagrams (WS3-025) are served read-only, images only and from the embed
	// FS only, so the rules/expectations panels resolve their relative image refs (WS9-030). The
	// built-in and intent docs live in separate embed FSes (WS3-093) behind one route, so the two
	// handlers are composed by firstImageHandler.
	mux.Handle("/rule-docs/", http.StripPrefix("/rule-docs/",
		firstImageHandler(builtin.RuleDocImageHandler(), intent.RuleDocImageHandler())))
	// The per-relation fact-doc cards (WS14-005), read-only and image-only like the rule docs, so
	// the query panel resolves a relation Detail's image refs.
	mux.Handle("/relation-docs/", http.StripPrefix("/relation-docs/", relations.RelationDocImageHandler()))
	// The datasheets workbench renders the source PDF with pdf.js, so its raw bytes are served
	// from the mounts. The prefix is more specific than the /datasheets/ page, so ServeMux routes
	// /datasheets/raw/... here.
	mux.Handle("/datasheets/raw/", http.StripPrefix("/datasheets/raw/", rawDatasheetHandler(mounts)))
	mux.Handle("GET /healthz", healthHandler())
	switch {
	case !assets.viewer:
		mux.Handle("/", apiOnlyHandler([]string{wsPath, prPath, dsPath, ckPath, diffPath, dtPath, qPath, rvPath}))
		fmt.Fprintf(cmd.ErrOrStderr(), "note: no web dir was named and there is no ./%s here, so this serves the API without the viewer. Point --web-dir, web_dir in an agni.yaml, or %s at a built web/ directory for the viewer.\n", defaultWebDir, envWebDir)
	default:
		if assets.datasheetsErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "note: %v\n", assets.datasheetsErr)
		}
		registerPages(newPageApp(dir, &serveApp{mounts: mounts}), mux, assets.datasheetsErr)
	}

	srv := &http.Server{Addr: addr, Handler: mux}
	urls := serveURLs(addr, lanIPs)
	if o.banner != nil {
		o.banner(urls, len(mounts))
	} else {
		what := dir
		if !assets.viewer {
			what = "the API"
		}
		fmt.Fprintf(os.Stderr, "serving %s at %s with %d mount(s) (Ctrl-C to stop)\n", what, urls[0], len(mounts))
		for _, u := range urls[1:] {
			fmt.Fprintf(os.Stderr, "  on this network: %s (all interfaces, no auth)\n", u)
		}
	}
	// servicekit drains in-flight requests on SIGINT/SIGTERM instead of dropping them.
	if o.listener != nil {
		return skhttp.ListenAndServeGraceful(srv, skhttp.WithListener(o.listener))
	}
	return skhttp.ListenAndServeGraceful(srv)
}

// projectTrees maps the configured mounts onto the filesystem-backed project store's trees. It is
// all of the OS adapter for projects. Each mount gets one os.DirFS, which makes containment
// structural, since an fs.FS has no parent to climb into.
func projectTrees(ms []mounts.Mount) []projects.Tree {
	out := make([]projects.Tree, 0, len(ms))
	for _, m := range ms {
		out = append(out, projects.Tree{Mount: m.Name, FS: os.DirFS(m.Root)})
	}
	return out
}

// serveLoader is what the two rule-running services need between them. The intersection is named
// once, by the engine facade, because an embedder building the same pair needs it too.
type serveLoader = agni.RuleLoader

// serveRuleServices builds the two services that RUN rules from one composed catalog: the
// CheckService behind the check panel and ListRules, and the ReviewService behind the review resources.
//
// Both come from one catalog because every overlay knob is config for the whole server, and a rule
// missing from the check panel's catalog looks the same as a rule that ran and found nothing
// (WS3-109). Returning the services rather than the catalog means a caller never holds the catalog,
// so it cannot hand the two surfaces different ones.
//
// conventions may be the zero Config, which contributes nothing. Its lexicon is NOT applied here,
// since that is a process-global install done once at startup, and doing it in a function tests call
// would leak one test's vocabulary into the next.
func serveRuleServices(loader serveLoader, store service.ReviewStore, specs param.ParamProvider, profilePath, intentPath string, conventions *configpb.NamingConvention, projects *service.ProjectResolver, notes io.Writer) (*service.CheckService, *service.ReviewService, error) {
	overlay, err := loadOverlayProfiles(profilePath)
	if err != nil {
		return nil, nil, err
	}
	var extra []check.RuleSource
	if len(conventions.GetRules()) > 0 {
		src, err := naming.Source(conventions)
		if err != nil {
			return nil, nil, err
		}
		extra = append(extra, src)
	}
	e, err := newEngine(overlay, intentPath, extra, agni.WithProjectResolver(projects))
	if err != nil {
		return nil, nil, err
	}
	if notes != nil {
		noteSupersededRules(notes, e.Catalog())
		for _, w := range e.Warnings() {
			fmt.Fprintln(notes, "note:", w)
		}
	}
	checkSvc, reviewSvc := e.RuleServices(agni.RuleServiceDeps{
		Loader:         loader,
		ReviewStore:    store,
		Specs:          specs,
		BaseConvention: conventions.GetName(),
	})
	return checkSvc, reviewSvc, nil
}

// serveURLs turns a listen address into URLs a human can click. The default ":8080" alone would
// print as "http://:8080/", which no browser opens.
//
// An empty, "0.0.0.0", or "::" host is the WILDCARD, reachable on every interface. The first entry
// returned is the one to click on this machine, and any others are the LAN addresses. Those are
// printed because this server has no authentication, so a wildcard bind exposes the mounted design
// folders to anything that can route here. Bind 127.0.0.1 to remove that.
//
// Only IPv4 is enumerated, because a link-local IPv6 address needs a zone suffix to be usable.
//
// ips is a parameter so tests do not depend on the host's interfaces; production passes lanIPs.
func serveURLs(addr string, ips func() []string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Not host:port. Nothing to shape, so echo it rather than guess at it.
		return []string{"http://" + addr + "/"}
	}
	if host != "" && host != "0.0.0.0" && host != "::" {
		return []string{hostURL(host, port)}
	}
	out := []string{hostURL("localhost", port)}
	for _, ip := range ips() {
		out = append(out, hostURL(ip, port))
	}
	return out
}

// hostURL joins a host and port into a URL. JoinHostPort already brackets an IPv6 literal, so do
// not bracket it here too.
func hostURL(host, port string) string {
	return "http://" + net.JoinHostPort(host, port) + "/"
}

// lanIPs reports this host's non-loopback IPv4 addresses, for the wildcard case in serveURLs. An
// interface that is down or has no usable address contributes nothing, and an enumeration error
// yields no addresses rather than an error, since this only decorates a startup line.
//
// Inside a container it reports nothing. A container interface address (172.17.0.2 and the like)
// is reachable only from the container network, and the published HOST port lives in the runtime
// where the process cannot see it. The operator set that port with `docker run -p` and knows it.
func lanIPs() []string {
	if inContainer() {
		return nil
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := n.IP.To4()
		// IsGlobalUnicast excludes loopback and multicast; IsLinkLocalUnicast drops the 169.254/16
		// self-assigned range, which routes nowhere useful.
		if ip == nil || !ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, ip.String())
	}
	sort.Strings(out)
	return out
}

// inContainer reports whether this process is running inside a container, which lanIPs uses to
// decide that its own interface addresses are not worth printing. Docker writes /.dockerenv and
// Podman writes /run/.containerenv.
//
// A wrong answer costs one startup line either way, so this does not go further (parsing
// /proc/1/cgroup, checking pid 1) to catch runtimes that leave no marker.
func inContainer() bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}

// healthHandler answers the container orchestrator's liveness/readiness probe. It is registered on
// the exact path "GET /healthz" so it does not shadow the page space, and it reports only that this
// process is up and serving HTTP.
//
// It does NOT re-probe the mounts, the rule catalog, or the params set. A bad --mount,
// --profile-path, --intent-path, or --params fails before the listener exists, so a 200 says
// nothing about configuration and should not be read as if it did.
func healthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok\n")
	})
}

// webAssets is what a resolved --web-dir can serve. The viewer group (landing, browse and design
// pages) and the datasheets workbench are separate because the workbench carries the pdf.js bundle,
// about two thirds of the compressed assets, and is a tool for building a parameter corpus rather
// than for looking at a board (agni issue 735).
type webAssets struct {
	dir    string
	source string // where dir came from, as resolveWebDir reports it
	// viewer is false only for API-only serving, when nothing named a web dir and the default is absent.
	viewer bool
	// datasheetsErr says why the workbench is not served, and is nil when it is.
	datasheetsErr error
}

// resolveWebAssets resolves --web-dir through its fallback chain and requires the VIEWER to be
// servable from what it found. It returns the directory and where the value came from, so a caller
// can narrate the provenance. The datasheets workbench is not required here; see resolveServeAssets.
//
// `--server self` calls it BEFORE the command it wraps does its work, so a run that cannot serve
// fails while its artifact is still unwritten, as a taken port does (agni issue 637).
func resolveWebAssets(flag string, getenv func(string) string) (dir, source string, err error) {
	dir, source = resolveWebDir(flag, getenv)
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() {
		return dir, source, fmt.Errorf("--web-dir %q is not a directory. Set it with --web-dir, or web_dir in an agni.yaml, or %s. A checkout has one at ./%s", dir, envWebDir, defaultWebDir)
	}
	if err := checkWebAssets(dir); err != nil {
		return dir, source, err
	}
	return dir, source, nil
}

// resolveServeAssets is `serve`'s version of resolveWebAssets, which also admits no viewer at all.
// When NOTHING named a web dir (no flag, no agni.yaml, no environment) and the default ./web does not
// exist, serve runs the API alone, as an installed binary outside a checkout must. Every other
// failure still fails, because a wrong named directory is a typo and a ./web without its bundle is a
// checkout that forgot `make ui` (agni issue 735).
//
// `open` and `--server self` call resolveWebAssets instead, since for them no viewer is an error.
func resolveServeAssets(flag string, getenv func(string) string) (webAssets, error) {
	dir, source := resolveWebDir(flag, getenv)
	if flag == "" && source == "" {
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			return webAssets{dir: dir}, nil
		}
	}
	dir, source, err := resolveWebAssets(flag, getenv)
	if err != nil {
		return webAssets{}, err
	}
	return webAssets{dir: dir, source: source, viewer: true, datasheetsErr: checkDatasheetAssets(dir)}, nil
}

// checkWebAssets verifies dir holds the viewer (its templates plus the built esbuild bundle) before
// the server starts, so a misdirected `serve --web-dir <design-folder>` fails upfront with guidance
// instead of a cryptic template-not-found on the first request. Design folders are exposed with
// --mount, not this flag.
func checkWebAssets(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "templates", "ViewerPage.html")); err != nil {
		return fmt.Errorf("%q has no templates/ViewerPage.html: --web-dir is the viewer's own assets dir (defaults to %q), not a folder to browse; mount design folders with --mount name=path", dir, defaultWebDir)
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "app.js")); err != nil {
		return fmt.Errorf("%q has no static/app.js: build the frontend bundle first with `cd %s && pnpm build`", dir, dir)
	}
	// The design browser (WS9-049) is the second server-rendered page with its own bundle. It is
	// also what "/" serves, so a missing browse asset breaks the landing page, not a side route.
	if _, err := os.Stat(filepath.Join(dir, "templates", "BrowsePage.html")); err != nil {
		return fmt.Errorf("%q has no templates/BrowsePage.html (the design browse page)", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "browse.js")); err != nil {
		return fmt.Errorf("%q has no static/browse.js: build the frontend bundle first with `cd %s && pnpm build`", dir, dir)
	}
	return nil
}

// checkDatasheetAssets reports whether the extraction workbench (WS13-006) can be served: its page,
// its bundle, and the standalone pdf.js worker the page loads. A missing one leaves the rest of the
// viewer serving, and /datasheets/ answers with this error rather than a broken page.
func checkDatasheetAssets(dir string) error {
	for _, f := range []string{"templates/DatasheetsPage.html", "static/datasheets.js", "static/pdf.worker.js"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			return fmt.Errorf("%q has no %s, so the datasheets workbench is off: build it with `cd %s && pnpm build`", dir, f, dir)
		}
	}
	return nil
}

// apiOnlyHandler answers every page URL when serve runs without a viewer. "/" gets a 200 so a probe or
// a person checking the server sees it is up; any other page path is a 404. Both bodies say what the
// server does serve and how to get the viewer, since the person reading it is the one who expected a
// page.
func apiOnlyHandler(services []string) http.Handler {
	var b strings.Builder
	b.WriteString("agni serve is running WITHOUT the viewer: no web dir was named and there is no ./web here.\n\n")
	b.WriteString("The Connect API answers POST /<service>/<method> with Content-Type: application/json. Services:\n")
	for _, s := range services {
		fmt.Fprintf(&b, "  %s\n", strings.TrimSuffix(s, "/"))
	}
	fmt.Fprintf(&b, "\nTo serve the viewer, point --web-dir, web_dir in an agni.yaml, or %s at a built web/ directory\n"+
		"(from a checkout: make ui), or run the container image, which carries one.\n", envWebDir)
	body := b.String()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		io.WriteString(w, body)
	})
}

// unavailableHandler answers a page space whose assets are absent, naming what is missing.
func unavailableHandler(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, err.Error(), http.StatusNotFound)
	})
}

// firstImageHandler composes several rule-source image handlers into one, serving the response of the
// first that does not 404 (WS3-093). The web resolves every rule's card under the single /rule-docs/
// route, so a request is tried against each source in turn. Card basenames are unique across
// sources, so order does not matter. Each handler is image-only and 404s cleanly on a miss, so the
// buffered probe has no side effect to undo.
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
