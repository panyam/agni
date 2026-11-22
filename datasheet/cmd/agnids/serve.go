package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	skmw "github.com/panyam/servicekit/middleware"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/datasheet/dsserver"
	"github.com/panyam/agni/datasheet/dsservice"
	"github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi/dsapiconnect"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/param/paramconnect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/storeurl"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	goal "github.com/panyam/goapplib"
	"github.com/panyam/goapplib/storage/mounts"
	skhttp "github.com/panyam/servicekit/http"
)

// serveCmd serves the extraction workbench's page, the raw PDFs it renders, and DatasheetService.
// DatasheetService holds the workbench's whole API, including its folder tree, so agnids can be
// hosted apart from the engine's `agni serve` (agni issue 744).
func serveCmd() *cobra.Command {
	var addr, webDir, pdf2doc, viewerURL, mountRoot, corpusDir, storeURL, uploadsURL string
	var auth, tlsCert, tlsKey string
	var trustedProxy []string
	var access tableAccess
	var specs []string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the datasheets workbench and its API over HTTP",
		Long: "serve hosts the extraction workbench at /datasheets/, the source PDFs it renders, and the\n" +
			"DatasheetService API on one listener. Pass --mount name=path (repeatable) for the folders\n" +
			"holding datasheets, and --web-dir for the built workbench assets.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			access.auth = auth != ""
			if access.connectionsKey != "" && (!access.auth || storeURL == "" || uploadsURL == "") {
				return fmt.Errorf("--connections-key checks each caller's own connections, so it needs --auth %s, --store and --uploads", authgate.Tokens)
			}
			st, uploadsDir, err := storeurl.Open(storeURL, uploadsURL)
			if err != nil {
				return err
			}
			defer st.Close()
			table, err := serveTable(specs, mountRoot, st, uploadsDir, access)
			if err != nil {
				return err
			}
			proxies, err := authgate.ParseProxies(trustedProxy)
			if err != nil {
				return err
			}
			if err := authgate.CheckFlags(auth, storeURL, tlsCert, tlsKey, addr, proxies); err != nil {
				return err
			}
			var tokens authgate.Keys
			if auth != "" {
				if tokens, err = st.Tokens(); err != nil {
					return err
				}
			}
			// With NO web dir named and no default one present, serve the API alone, as `agni serve`
			// does (agni 735). A deployment that only publishes a corpus for --params-url has no
			// workbench to show. A named dir, or a default one that exists, must hold a built
			// workbench. A broken one is then a mistake the operator needs to hear about.
			if !cmd.Flags().Changed("web-dir") {
				if _, err := os.Stat(webDir); errors.Is(err, os.ErrNotExist) {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: no web dir was named and %s is absent, so this serves the API without the workbench\n", webDir)
					webDir = ""
				}
			}
			if webDir != "" {
				if err := checkWorkbenchAssets(webDir); err != nil {
					return err
				}
			}
			var store param.Fetcher
			if corpusDir != "" {
				if store, err = openCorpus(corpusDir, cmd.ErrOrStderr()); err != nil {
					return fmt.Errorf("--corpus %q: %w", corpusDir, err)
				}
			}
			// With --connections-key a caller sees a private mount when their own connection reads its
			// repo, as on agni serve (agni issue 1778).
			wa := workbenchAccess{hideRoots: access.auth}
			if access.checks() {
				wa.guard = service.NewAccessGuard(table, "")
			}
			mux := newWorkbenchMux(table, wa, webDir, strings.Fields(pdf2doc), strings.TrimSuffix(viewerURL, "/"), corpusDir, store)
			scheme := "http"
			if tlsCert != "" {
				scheme = "https"
			}
			if webDir == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "serving the datasheet API at %s://%s/ with %d mount(s) (Ctrl-C to stop)\n", scheme, displayAddr(addr), len(table.List()))
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "serving the datasheets workbench from %s at %s://%s/datasheets/ with %d mount(s) (Ctrl-C to stop)\n", webDir, scheme, displayAddr(addr), len(table.List()))
			}
			return listen(&http.Server{Addr: addr, Handler: guard(mux, tokens, proxies), ReadHeaderTimeout: 10 * time.Second}, tlsCert, tlsKey)
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8090", "address to listen on")
	c.Flags().StringArrayVar(&specs, "mount", nil, "expose a folder of datasheets as name=path (repeatable)")
	c.Flags().StringVar(&mountRoot, "mount-root", "", "expose every subdirectory of this path as a mount named after it, so folders can be bind-mounted in without a --mount flag each; an explicit --mount of the same name wins, and a missing root yields no mounts rather than an error")
	c.Flags().StringVar(&webDir, "web-dir", "datasheet/web", "directory holding the workbench's built assets (templates/DatasheetsPage.html, static/datasheets.js, static/pdf.worker.js); the default is where a repo checkout keeps them, after `make ui`")
	c.Flags().StringVar(&viewerURL, "viewer-url", "", "where the viewer (`agni serve`) is served, e.g. http://host:8080; the workbench's heading links home there, and is plain text when this is empty")
	c.Flags().StringVar(&corpusDir, "corpus", "", "the corpus store: published PartSpecs with their corpus.index.json, served over PartSpecService for `agni serve --params-url`, and the workbench's drafts under drafts/. It is indexed at start when the index is missing or stale. Empty serves neither, and the workbench cannot save")
	c.Flags().StringVar(&storeURL, "store", "", "agni serve's --store, as a URL such as file:///srv/agni/meta, sqlite:///srv/agni.db, postgres://agni@db/agni or datastore://project?namespace=agni; with --uploads, the workbench also serves the mounts agni serve made at runtime, such as a GitHub snapshot, and sees a new one within a second. It only reads them")
	c.Flags().StringVar(&uploadsURL, "uploads", "", "agni serve's --uploads, as a URL such as file:///srv/agni/files, where the runtime mounts' files are. Needs --store")
	c.Flags().StringVar(&auth, "auth", "", "ask every caller who they are: \""+authgate.Tokens+"\" accepts the tokens `agni token create` keeps in --store, the same ones agni serve accepts, and refuses a call without one (agni issue 1765). A browser signs in at /signin. Needs --store, and --tls-cert and --tls-key unless --addr is on localhost or --trusted-proxy names the proxy that ends TLS")
	c.Flags().StringVar(&access.connectionsKey, "connections-key", "", "agni serve's --connections-key, as env:NAME or file:/abs/path (agni issue 1778). With --auth, a caller sees a private mount when their own connection in --store reads its repo. agnids reads people's tokens and never refreshes or writes one, so a signed-in token that has expired reads as no until agni serve refreshes it. Without it, --auth hides every private mount. Needs --auth, --store and --uploads")
	c.Flags().StringVar(&access.githubAPI, "github-api", "", "the GitHub REST API a caller's access is checked through; a test or a GitHub Enterprise server names its own")
	_ = c.Flags().MarkHidden("github-api")
	c.Flags().StringVar(&tlsCert, "tls-cert", "", "a PEM certificate file to serve HTTPS with; with --tls-key")
	c.Flags().StringVar(&tlsKey, "tls-key", "", "the PEM private key file for --tls-cert")
	c.Flags().StringSliceVar(&trustedProxy, "trusted-proxy", nil, "the proxy that ends TLS in front of agnids, as an address or a CIDR range such as 10.0.0.0/8, comma-separated or repeated, as agni serve takes it (agni issue 1810). With it, --auth serves plain HTTP on any address, and a request from one of these that says X-Forwarded-Proto: https gets Secure cookies")
	c.Flags().StringVar(&pdf2doc, "pdf2doc", "", "command that derives a datasheet's doc-IR, e.g. \"python3 datasheet/tools/pdf2doc/pdf2doc.py\"; empty disables the workbench's Extract (first pass) action")
	return c
}

// newWorkbenchMux routes everything agnids serves: the DatasheetService API, the raw PDFs the
// workbench renders, its static bundle, the page itself, and a liveness probe. It is separate from
// serveCmd so a test can drive the real routes without a listener.
func newWorkbenchMux(ms *mountset.Table, access workbenchAccess, webDir string, pdf2doc []string, viewerURL string, corpusDir string, corpusStore param.Fetcher) *http.ServeMux {
	mux := http.NewServeMux()
	// Drafts live in the published corpus's store, so a server with no --corpus has none, and the
	// draft rpcs say to start it with one.
	var drafts dsservice.DraftStore
	if corpusDir != "" {
		drafts = newOSDraftStore(corpusDir)
	}
	// A datasheet's doc-IR and annotations live in the corpus store, keyed by the PDF's hash, so the
	// mounts are only read (agni issue 1734).
	guard := access.guard
	d := derived{mounts: ms, guard: guard, corpus: corpusDir}
	svc := dsservice.NewDatasheetService(&osDocLoader{derived: d}, drafts,
		&osDocExtractor{derived: d, cmd: pdf2doc}, &osAnnotationStore{derived: d})
	path, handler := dsapiconnect.NewDatasheetServiceHandler(dsserver.NewDatasheet(svc))
	mux.Handle(path, handler)
	// The workbench lists its folders through the engine's own WorkspaceService, over these mounts.
	workspace := service.NewWorkspaceService(mountset.NewWorkspace(ms)).WithGuard(guard)
	if access.hideRoots {
		// A caller on another machine has no business with the server's folder layout, as on agni
		// serve (agni issue 1627).
		workspace.WithHiddenRoots()
	}
	wpath, whandler := webapiconnect.NewWorkspaceServiceHandler(server.NewWorkspace(workspace))
	mux.Handle(wpath, whandler)
	// The published corpus's read side, for `agni serve --params-url`. Registered with no corpus too,
	// so a caller is told to start agnids with --corpus rather than getting a 404.
	ppath, phandler := paramconnect.NewPartSpecServiceHandler(dsserver.NewPartSpec(dsservice.NewPartSpecService(corpusStore)))
	mux.Handle(ppath, phandler)
	// The workbench renders the source PDF with pdf.js, so its raw bytes are served from the mounts.
	// The prefix is more specific than the page's, so ServeMux routes it here.
	mux.Handle("/datasheets/raw/", http.StripPrefix("/datasheets/raw/", rawDatasheetHandler(ms, guard)))
	// A workbench link from when datasheets lived under /datasheets/files/ redirects to the same path
	// under /datasheets/mounts/, query and all (agni issue 1734).
	mux.HandleFunc("/datasheets/files/", func(w http.ResponseWriter, r *http.Request) {
		target := "/datasheets/mounts/" + strings.TrimPrefix(r.URL.Path, "/datasheets/files/")
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710 -- target always starts /datasheets/mounts/, so the redirect stays on this site
	})
	// An empty webDir serves the API alone, with no page, no bundle, and no redirect to a missing
	// page.
	if webDir != "" {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))
		app := goal.NewApp(&dsApp{viewerURL: viewerURL}, goal.SetupTemplates(filepath.Join(webDir, "templates")))
		goal.Register[*DatasheetsPage](app, mux, "/datasheets/")
		mux.Handle("GET /{$}", http.RedirectHandler("/datasheets/", http.StatusFound))
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// openCorpus opens the published corpus at dir for PartSpecService. It refreshes the index first,
// and writes it when it is missing or no longer matches the files. The index is derived and can
// always be rebuilt. So a fresh corpus, such as a new volume or a new directory, needs no separate
// step, and start picks up a hand edit made while the server was down. A corpus that does not
// validate fails here, by name. While the server runs, a file edited behind the index is refused
// rather than re-indexed. Those errors are classified as dsservice.ErrCorpusNotReady, so a caller is
// told what to run rather than given a bad request.
func openCorpus(dir string, notes io.Writer) (param.Fetcher, error) {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	fsys := os.DirFS(dir)
	prev, err := readIndex(fsys)
	if err != nil {
		return nil, err
	}
	next, changed, err := corpus.Refresh(fsys, prev)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := writeIndex(dir, next); err != nil {
			return nil, err
		}
		fmt.Fprintf(notes, "note: indexed the corpus at start: %d specs, generation %d\n", len(next.Entries), next.Generation)
	}
	r, err := corpus.NewReader(fsys)
	if err != nil {
		return nil, err
	}
	return corpusStore{r}, nil
}

// corpusStore classifies a corpus.Reader's errors for the service.
type corpusStore struct{ r *corpus.Reader }

func (c corpusStore) BatchGet(ctx context.Context, mpns []string) ([]*parampb.PartSpec, uint64, error) {
	specs, gen, err := c.r.BatchGet(ctx, mpns)
	return specs, gen, classifyCorpusErr(err)
}

func (c corpusStore) Generation(ctx context.Context) (uint64, error) {
	gen, err := c.r.Generation(ctx)
	return gen, classifyCorpusErr(err)
}

func classifyCorpusErr(err error) error {
	if errors.Is(err, corpus.ErrStale) || errors.Is(err, corpus.ErrNoIndex) {
		return fmt.Errorf("%w: %w", dsservice.ErrCorpusNotReady, err)
	}
	return err
}

// serveMounts composes the mount table from --mount and --mount-root. The root is the image's
// zero-flag path, as on `agni serve`. Every subdirectory becomes a mount named after itself, so
// `-v ~/datasheets/ti:/datasheets/ti` needs no flag. An explicit --mount of the same name wins.
func serveMounts(specs []string, root string) ([]mounts.Mount, error) {
	explicit, err := mountset.Parse(specs)
	if err != nil {
		return nil, err
	}
	var discovered []mounts.Mount
	if root != "" {
		if discovered, err = mountset.Discover(root); err != nil {
			return nil, err
		}
	}
	return mountset.Merge(discovered, explicit), nil
}

// serveTable is the workbench's live mount table: the folders from --mount and --mount-root, and,
// with agni serve's --store and --uploads, the mounts agni serve made at runtime, read from their
// records (agni issue 1735). agnids only reads the records, and sees a new one within a second.
//
// Under --auth with no --connections-key it serves no mount that reads a private repo through a
// connection, since it can't tell who may read one (agni issue 1728). With --connections-key it
// opens agni serve's connections and people's sealed tokens for reading, and the caller's own
// connection decides (agni issue 1778).
func serveTable(specs []string, root string, st *storeurl.Store, uploadsDir string, access tableAccess) (*mountset.Table, error) {
	ms, err := serveMounts(specs, root)
	if err != nil {
		return nil, err
	}
	cfg := mountset.Config{Fixed: ms, ReadOnly: true, HidePrivate: access.auth && !access.checks(), GitHubAPI: access.githubAPI}
	if st != nil && uploadsDir != "" {
		cfg.Uploads = uploadsDir
		// A sqlite store keeps the mounts, connections and tokens in its database (agni issue 1733).
		stores, err := st.TableStores()
		if err != nil {
			return nil, err
		}
		if stores != nil && !access.checks() {
			stores.Connections, stores.UserTokens = nil, nil
		}
		cfg.Stores = stores
		if st.Dir != "" {
			cfg.Records = filepath.Join(st.Dir, mountset.RecordsDir)
			if access.checks() {
				cfg.Connections, cfg.UserTokens = filepath.Join(st.Dir, mountset.ConnectionsDir), filepath.Join(st.Dir, mountset.UserTokensDir)
			}
		}
		if access.checks() {
			cfg.ConnectionsKey = access.connectionsKey
		}
	}
	return mountset.Open(cfg)
}

// workbenchAccess is what newWorkbenchMux checks for each caller. guard refuses a private mount the
// caller's own connections can't read, and is nil when the server doesn't check (agni issue 1778).
// hideRoots leaves each mount's folder out of ListMounts, for a server under --auth.
type workbenchAccess struct {
	guard     *service.AccessGuard
	hideRoots bool
}

// tableAccess is how serveTable decides who sees a private mount: auth is --auth, connectionsKey
// is --connections-key, and githubAPI is the hidden --github-api.
type tableAccess struct {
	auth                      bool
	connectionsKey, githubAPI string
}

// checks reports whether the server checks each caller's own connections, rather than hiding every
// private mount from everyone.
func (a tableAccess) checks() bool { return a.auth && a.connectionsKey != "" }

// displayAddr turns a listen address into one a browser can open, since ":8090" listens on every
// interface and is not a host.
func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

// checkWorkbenchAssets verifies --web-dir holds the workbench before the listener opens: its page,
// its bundle, and the standalone pdf.js worker the page loads. A missing one fails at startup with
// the file named, rather than as a broken page on the first request.
func checkWorkbenchAssets(dir string) error {
	for _, f := range []string{"templates/DatasheetsPage.html", "static/datasheets.js", "static/pdf.worker.js"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			return fmt.Errorf("%q has no %s, so the datasheets workbench cannot be served: build it with `cd %s && pnpm build`, or point --web-dir at a built one", dir, f, dir)
		}
	}
	return nil
}

// dsApp is the goapplib application context for the workbench page. It holds the viewer's location
// for the heading's link home, since the workbench is hosted apart from the viewer.
type dsApp struct {
	viewerURL string
}

// DatasheetsPage is the server-rendered shell of the extraction workbench (WS13-006). Its template
// (DatasheetsPage.html) renders a datasheet-tree sidebar and the region viewer hole, and its own
// bundle (static/datasheets.js) loads pdf.js. goapplib maps this type to DatasheetsPage.html by name.
type DatasheetsPage struct {
	Title string
	// ViewerURL is the viewer's base URL (--viewer-url). The heading links home there, and is plain
	// text without it, since "/" on agnids is the workbench itself.
	ViewerURL string
}

// Load populates the workbench page before render. The shell is static, since a datasheet's doc-IR
// and source PDF arrive over the Connect API and the raw endpoint.
func (p *DatasheetsPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*dsApp]) (error, bool) {
	p.Title = "Agni datasheets"
	p.ViewerURL = app.Context.viewerURL
	return nil, false
}

// guard puts the token check in front of h, checking tokens against tokens (agni issue 1765). Nil
// tokens, for a server without --auth, checks nothing. proxies are the proxies that end TLS in front
// of agnids (agni issue 1810).
func guard(h http.Handler, tokens authgate.Keys, proxies skmw.TrustedProxies) http.Handler {
	if tokens == nil {
		return h
	}
	return authgate.New(tokens, proxies).Wrap(h)
}

// listen serves srv until a signal, over TLS when certFile names a certificate.
func listen(srv *http.Server, certFile, keyFile string) error {
	if certFile == "" {
		return skhttp.ListenAndServeGraceful(srv)
	}
	return skhttp.ListenAndServeGraceful(srv, skhttp.WithTLS(certFile, keyFile))
}
