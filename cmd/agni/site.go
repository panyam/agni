package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/service"
	goal "github.com/panyam/goapplib"
	"github.com/spf13/cobra"
)

// siteCmd writes the public demo as plain files for a static host (agni issue 856): a landing page
// listing seeded example boards, the viewer shell at each seeded design's address and at the drop
// page, the built viewer and wasm engine, and each seed's files with a listing of them. Nothing in
// it needs an agni server; every page runs the engine in the visitor's browser.
func siteCmd() *cobra.Command {
	var base, webDir string
	var seeds []string
	c := &cobra.Command{
		Use:   "site <outdir>",
		Short: "Write the browser-only demo as static files for a static host",
		Long: "Write the public demo as plain files: a landing page listing the seeded boards, the viewer " +
			"at each seeded design and at the drop page, the built viewer and wasm engine (run `make ui " +
			"wasm` first), and each seed's files. Every page runs the engine in the visitor's browser, " +
			"so the output can be served by any static host under --base.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(seeds) == 0 {
				return fmt.Errorf("name at least one --seed name=folder")
			}
			return writeSite(args[0], webDir, base, seeds, cmd.ErrOrStderr())
		},
	}
	c.Flags().StringVar(&base, "base", "/", "the path the site is served under, such as /agni/demo/; every asset and link goes through it")
	c.Flags().StringVar(&webDir, "web-dir", defaultWebDir, "the viewer's own assets (templates/ and the built static/)")
	c.Flags().StringArrayVar(&seeds, "seed", nil, "an example board, as name=folder (repeatable); the folder becomes mount <name>")
	return c
}

// siteSeed is one example board as the landing page lists it.
type siteSeed struct {
	Title, Href, Format, Size, Licence, Note string
}

// SitePage is the static demo's landing page. goapplib maps it to SitePage.html by name.
type SitePage struct {
	Title string
	Base  string
	Seeds []siteSeed
}

// Load fills the page from the site being written.
func (p *SitePage) Load(r *http.Request, w http.ResponseWriter, app *goal.App[*serveApp]) (error, bool) {
	p.Title = "Agni: open a design in your browser"
	p.Base = app.Context.basePath()
	p.Seeds = app.Context.siteSeeds
	return nil, false
}

func writeSite(out, webDir, base string, seeds []string, notes interface{ Write([]byte) (int, error) }) error {
	if _, err := os.Stat(filepath.Join(webDir, "templates", "ViewerPage.html")); err != nil {
		return fmt.Errorf("--web-dir %q has no templates/ViewerPage.html", webDir)
	}
	for _, need := range []string{"app.js", "agni.wasm", "wasm_exec.js", "agni-worker.js"} {
		if _, err := os.Stat(filepath.Join(webDir, "static", need)); err != nil {
			return fmt.Errorf("--web-dir %q has no static/%s; run `make ui wasm` first", webDir, need)
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := copyTree(filepath.Join(webDir, "static"), filepath.Join(out, "static")); err != nil {
		return err
	}

	sa := &serveApp{engine: engineWasm, base: base, static: true}
	var pages []string // design addresses, relative to the base
	for _, spec := range seeds {
		name, dir, ok := strings.Cut(spec, "=")
		if !ok || name == "" || dir == "" || strings.ContainsAny(name, "/ ") {
			return fmt.Errorf("--seed %q must be name=folder, the name one path element", spec)
		}
		// A seed folder that declares a design at its own root goes one folder down in its mount, since a
		// design is addressed by its path within the mount and the mount root has none (agni 857).
		under := ""
		if _, err := os.Stat(filepath.Join(dir, "design.yaml")); err == nil {
			under = filepath.Base(filepath.Clean(dir))
		}
		root := filepath.Join(out, "raw", name)
		listing, err := copySeed(dir, filepath.Join(root, under), name, under)
		if err != nil {
			return fmt.Errorf("--seed %s: %w", name, err)
		}
		b, err := protoJSON(listing)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(out, "files"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, "files", name+".json"), []byte(b), 0o644); err != nil {
			return err
		}
		designs, err := seedDesigns(root, name)
		if err != nil {
			return fmt.Errorf("--seed %s: %w", name, err)
		}
		for _, d := range designs {
			pages = append(pages, d.addr)
			sa.siteSeeds = append(sa.siteSeeds, siteSeed{
				Title: d.title, Href: d.addr + "/", Format: d.format,
				Size: humanBytes(listing.GetTotalSize()), Licence: licenceOf(dir), Note: d.note,
			})
		}
	}
	pages = append(pages, "designs/"+wasmengine.BrowserMount+"/view")

	app := newPageApp(webDir, sa)
	viewer, err := renderPage(pageHandler[*ViewerPage](app))
	if err != nil {
		return err
	}
	for _, p := range pages {
		dst := filepath.Join(out, filepath.FromSlash(p), "index.html")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, viewer, 0o644); err != nil {
			return err
		}
	}
	landing, err := renderPage(goal.Register[*SitePage](app, nil, "/"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "index.html"), landing, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(notes, "wrote %s: %d example design(s) and the drop page, under %s\n", out, len(sa.siteSeeds), sa.basePath())
	return nil
}

// renderPage renders a page handler's output as the static file a host will serve.
func renderPage(h http.Handler) ([]byte, error) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		return nil, fmt.Errorf("rendering a page: %d %s", rec.Code, rec.Body.String())
	}
	return bytes.Clone(rec.Body.Bytes()), nil
}

// copySeed copies a seed folder's files, dotfiles skipped, and returns their listing in the shape
// ListDesignFiles answers, which is what the page reads on a static host. under is the folder dst
// sits at within the mount, "" for its root, and prefixes every listed path.
func copySeed(src, dst, mount, under string) (*webapi.ListDesignFilesResponse, error) {
	resp := &webapi.ListDesignFilesResponse{Mount: mount}
	fsys := os.DirFS(src)
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != "." {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return err
		}
		resp.Files = append(resp.Files, &webapi.DesignFile{Path: path.Join(under, p), Size: int64(len(b)), Sha256: fshost.ContentHash(fsys, p)})
		resp.TotalSize += int64(len(b))
		return nil
	})
	sort.Slice(resp.Files, func(i, j int) bool { return resp.Files[i].GetPath() < resp.Files[j].GetPath() })
	return resp, err
}

type seededDesign struct{ addr, title, format, note string }

// seedDesigns are the designs a seed folder makes, as ProposeDesigns groups them, each with the
// address its viewer page is written at.
func seedDesigns(dir, mount string) ([]seededDesign, error) {
	host := fshost.New(fshost.Mount{Name: mount, FS: os.DirFS(dir)})
	svc := service.NewWorkspaceService(host.Workspace()).WithDesignFiles(nil, host.Workspace())
	resp, err := svc.ProposeDesigns(context.Background(), &webapi.ProposeDesignsRequest{Uri: "mount://" + mount})
	if err != nil {
		return nil, err
	}
	var out []seededDesign
	// One example per folder, the design that folder's design.yaml would declare. A KiCad project
	// folder often holds sheets nothing references, which propose as designs of their own and would
	// crowd the list with fragments of the board.
	listed := map[string]bool{}
	for _, pd := range resp.GetDesigns() {
		if listed[pd.GetFolder()] {
			continue
		}
		listed[pd.GetFolder()] = true
		target := pd.GetDesign().GetEntryUri()
		if target == "" {
			target = pd.GetDesign().GetUri()
		}
		rel := strings.TrimPrefix(target, "mount://"+mount)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			continue
		}
		// The title a person gave the design, else the entry file's own name, rather than the id.
		title := yamlField(pd.GetDesignYaml(), "title")
		entry := yamlField(pd.GetDesignYaml(), "entry")
		if entry == "" {
			entry = path.Base(rel)
		}
		if title == "" {
			title = strings.TrimSuffix(path.Base(entry), path.Ext(entry))
		}
		format := service.FormatForExt(entry)
		if format == "" {
			format = "design folder"
		}
		out = append(out, seededDesign{
			addr:   path.Join("designs", mount, rel, "view"),
			title:  fmt.Sprintf("%s (%s)", title, mount),
			format: format,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s makes no design", dir)
	}
	return out, nil
}

// yamlField reads a top-level scalar out of a descriptor's text, enough for a title or an entry
// without binding the whole document.
func yamlField(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// licenceOf is the first line of a seed's LICENSE file, which names the licence its files carry.
func licenceOf(dir string) string {
	for _, n := range []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "LICENCE"} {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(line); t != "" {
				return t
			}
		}
	}
	return ""
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", (n+1023)/1024)
	}
	return fmt.Sprintf("%d bytes", n)
}

// copyTree copies a folder's files into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}
