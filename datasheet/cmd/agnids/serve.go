package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/datasheet/dsserver"
	"github.com/panyam/agni/datasheet/dsservice"
	"github.com/panyam/agni/gen/go/agni/v1/dsapi/dsapiconnect"
	"github.com/panyam/agni/mounts"
	goal "github.com/panyam/goapplib"
	skhttp "github.com/panyam/servicekit/http"
)

// serveCmd serves the extraction workbench: its page, the raw PDFs it renders, and DatasheetService,
// which carries the workbench's whole API including its folder tree, so agnids can be hosted apart
// from the engine's `agni serve` (agni issue 744).
func serveCmd() *cobra.Command {
	var addr, webDir, pdf2doc string
	var specs []string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the datasheets workbench and its API over HTTP",
		Long: "serve hosts the extraction workbench at /datasheets/, the source PDFs it renders, and the\n" +
			"DatasheetService API on one listener. Pass --mount name=path (repeatable) for the folders\n" +
			"holding datasheets, and --web-dir for the built workbench assets.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ms, err := mounts.Parse(specs)
			if err != nil {
				return err
			}
			if err := checkWorkbenchAssets(webDir); err != nil {
				return err
			}
			mux := newWorkbenchMux(ms, webDir, strings.Fields(pdf2doc))
			fmt.Fprintf(cmd.ErrOrStderr(), "serving the datasheets workbench from %s at http://%s/datasheets/ with %d mount(s) (Ctrl-C to stop)\n", webDir, displayAddr(addr), len(ms))
			return skhttp.ListenAndServeGraceful(&http.Server{Addr: addr, Handler: mux})
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8090", "address to listen on")
	c.Flags().StringArrayVar(&specs, "mount", nil, "expose a folder of datasheets as name=path (repeatable)")
	c.Flags().StringVar(&webDir, "web-dir", "web", "directory holding the workbench's built assets (templates/DatasheetsPage.html, static/datasheets.js, static/pdf.worker.js)")
	c.Flags().StringVar(&pdf2doc, "pdf2doc", "", "command that derives a datasheet's doc-IR, e.g. \"python3 datasheet/tools/pdf2doc/pdf2doc.py\"; empty disables the workbench's Extract (first pass) action")
	return c
}

// newWorkbenchMux routes everything agnids serves: the DatasheetService API, the raw PDFs the
// workbench renders, its static bundle, the page itself, and a liveness probe. It is separate from
// serveCmd so a test can drive the real routes without a listener.
func newWorkbenchMux(ms []mounts.Mount, webDir string, pdf2doc []string) *http.ServeMux {
	mux := http.NewServeMux()
	svc := dsservice.NewDatasheetService(&osDocLoader{mounts: ms}, &osPartSpecStore{mounts: ms},
		&osDocExtractor{mounts: ms, cmd: pdf2doc}, &osAnnotationStore{mounts: ms}, mounts.NewWorkspace(ms))
	path, handler := dsapiconnect.NewDatasheetServiceHandler(dsserver.NewDatasheet(svc))
	mux.Handle(path, handler)
	// The workbench renders the source PDF with pdf.js, so its raw bytes are served from the mounts.
	// The prefix is more specific than the page's, so ServeMux routes it here.
	mux.Handle("/datasheets/raw/", http.StripPrefix("/datasheets/raw/", rawDatasheetHandler(ms)))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(webDir, "static")))))
	app := goal.NewApp(&dsApp{}, goal.SetupTemplates(filepath.Join(webDir, "templates")))
	goal.Register[*DatasheetsPage](app, mux, "/datasheets/")
	mux.Handle("GET /{$}", http.RedirectHandler("/datasheets/", http.StatusFound))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})
	return mux
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

// dsApp is the goapplib application context for the workbench page. The page is a static shell, so
// it carries nothing yet.
type dsApp struct{}

// DatasheetsPage is the server-rendered shell of the extraction workbench (WS13-006). Its template
// (DatasheetsPage.html) renders a datasheet-tree sidebar and the region viewer hole, and its own
// bundle (static/datasheets.js) loads pdf.js. goapplib maps this type to DatasheetsPage.html by name.
type DatasheetsPage struct {
	Title string
}

// Load populates the workbench page before render. The shell is static, since a datasheet's doc-IR
// and source PDF arrive over the Connect API and the raw endpoint.
func (p *DatasheetsPage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*dsApp]) (error, bool) {
	p.Title = "Agni datasheets"
	return nil, false
}
