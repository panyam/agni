package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/datasheet/corpus"
	"github.com/panyam/agni/datasheet/dsserver"
	"github.com/panyam/agni/datasheet/dsservice"
	"github.com/panyam/agni/datasheet/gen/go/agni/v1/dsapi/dsapiconnect"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/param/paramconnect"
	"github.com/panyam/agni/mounts"
	goal "github.com/panyam/goapplib"
	skhttp "github.com/panyam/servicekit/http"
)

// serveCmd serves the extraction workbench: its page, the raw PDFs it renders, and DatasheetService,
// which carries the workbench's whole API including its folder tree, so agnids can be hosted apart
// from the engine's `agni serve` (agni issue 744).
func serveCmd() *cobra.Command {
	var addr, webDir, pdf2doc, viewerURL, mountRoot, corpusDir string
	var specs []string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the datasheets workbench and its API over HTTP",
		Long: "serve hosts the extraction workbench at /datasheets/, the source PDFs it renders, and the\n" +
			"DatasheetService API on one listener. Pass --mount name=path (repeatable) for the folders\n" +
			"holding datasheets, and --web-dir for the built workbench assets.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := serveMounts(specs, mountRoot)
			if err != nil {
				return err
			}
			if err := checkWorkbenchAssets(webDir); err != nil {
				return err
			}
			var store param.Fetcher
			if corpusDir != "" {
				if store, err = openCorpus(corpusDir); err != nil {
					return fmt.Errorf("--corpus %q: %w", corpusDir, err)
				}
			}
			mux := newWorkbenchMux(ms, webDir, strings.Fields(pdf2doc), strings.TrimSuffix(viewerURL, "/"), store)
			fmt.Fprintf(cmd.ErrOrStderr(), "serving the datasheets workbench from %s at http://%s/datasheets/ with %d mount(s) (Ctrl-C to stop)\n", webDir, displayAddr(addr), len(ms))
			return skhttp.ListenAndServeGraceful(&http.Server{Addr: addr, Handler: mux})
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8090", "address to listen on")
	c.Flags().StringArrayVar(&specs, "mount", nil, "expose a folder of datasheets as name=path (repeatable)")
	c.Flags().StringVar(&mountRoot, "mount-root", "", "expose every subdirectory of this path as a mount named after it, so folders can be bind-mounted in without a --mount flag each; an explicit --mount of the same name wins, and a missing root yields no mounts rather than an error")
	c.Flags().StringVar(&webDir, "web-dir", "datasheet/web", "directory holding the workbench's built assets (templates/DatasheetsPage.html, static/datasheets.js, static/pdf.worker.js); the default is where a repo checkout keeps them, after `make ui`")
	c.Flags().StringVar(&viewerURL, "viewer-url", "", "where the viewer (`agni serve`) is served, e.g. http://host:8080; the workbench's heading links home there, and is plain text when this is empty")
	c.Flags().StringVar(&corpusDir, "corpus", "", "a published PartSpec corpus (a directory `agnids promote` writes into, with its corpus.index.json) to serve over PartSpecService, which `agni serve --params-url` reads; empty serves none")
	c.Flags().StringVar(&pdf2doc, "pdf2doc", "", "command that derives a datasheet's doc-IR, e.g. \"python3 datasheet/tools/pdf2doc/pdf2doc.py\"; empty disables the workbench's Extract (first pass) action")
	return c
}

// newWorkbenchMux routes everything agnids serves: the DatasheetService API, the raw PDFs the
// workbench renders, its static bundle, the page itself, and a liveness probe. It is separate from
// serveCmd so a test can drive the real routes without a listener.
func newWorkbenchMux(ms []mounts.Mount, webDir string, pdf2doc []string, viewerURL string, corpusStore param.Fetcher) *http.ServeMux {
	mux := http.NewServeMux()
	svc := dsservice.NewDatasheetService(&osDocLoader{mounts: ms}, &osPartSpecStore{mounts: ms},
		&osDocExtractor{mounts: ms, cmd: pdf2doc}, &osAnnotationStore{mounts: ms}, mounts.NewWorkspace(ms))
	path, handler := dsapiconnect.NewDatasheetServiceHandler(dsserver.NewDatasheet(svc))
	mux.Handle(path, handler)
	// The published corpus's read side, for `agni serve --params-url`. Registered with no corpus too,
	// so a caller is told to start agnids with --corpus rather than getting a 404.
	ppath, phandler := paramconnect.NewPartSpecServiceHandler(dsserver.NewPartSpec(dsservice.NewPartSpecService(corpusStore)))
	mux.Handle(ppath, phandler)
	// The workbench renders the source PDF with pdf.js, so its raw bytes are served from the mounts.
	// The prefix is more specific than the page's, so ServeMux routes it here.
	mux.Handle("/datasheets/raw/", http.StripPrefix("/datasheets/raw/", rawDatasheetHandler(ms)))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))
	app := goal.NewApp(&dsApp{viewerURL: viewerURL}, goal.SetupTemplates(filepath.Join(webDir, "templates")))
	goal.Register[*DatasheetsPage](app, mux, "/datasheets/")
	mux.Handle("GET /{$}", http.RedirectHandler("/datasheets/", http.StatusFound))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	return mux
}

// openCorpus opens the published corpus at dir for PartSpecService. Its errors that mean "this corpus
// cannot be served as it stands" (no index, an index that no longer matches its files) are classified
// as dsservice.ErrCorpusNotReady, so a caller is told what to run rather than given a bad request.
func openCorpus(dir string) (param.Fetcher, error) {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	r, err := corpus.NewReader(os.DirFS(dir))
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
// zero-flag path, as on `agni serve`: every subdirectory becomes a mount named after itself, so
// `-v ~/datasheets/ti:/datasheets/ti` needs no flag, and an explicit --mount of the same name wins.
func serveMounts(specs []string, root string) ([]mounts.Mount, error) {
	explicit, err := mounts.Parse(specs)
	if err != nil {
		return nil, err
	}
	var discovered []mounts.Mount
	if root != "" {
		if discovered, err = mounts.Discover(root); err != nil {
			return nil, err
		}
	}
	return mounts.Merge(discovered, explicit), nil
}

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

// dsApp is the goapplib application context for the workbench page. It carries where the viewer is,
// for the heading's link home, since the workbench is hosted apart from it.
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
