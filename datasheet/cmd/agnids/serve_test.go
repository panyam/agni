package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/mounts"
)

func writeEmpty(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// agnids refuses to start without the workbench's three files, naming the missing one and how to
// build it, so a wrong --web-dir fails at startup rather than as a broken page.
func TestCheckWorkbenchAssets(t *testing.T) {
	dir := t.TempDir()
	if err := checkWorkbenchAssets(dir); err == nil || !strings.Contains(err.Error(), "DatasheetsPage.html") {
		t.Errorf("missing datasheets page should name DatasheetsPage.html, got %v", err)
	}
	writeEmpty(t, filepath.Join(dir, "templates", "DatasheetsPage.html"))
	if err := checkWorkbenchAssets(dir); err == nil || !strings.Contains(err.Error(), "datasheets.js") || !strings.Contains(err.Error(), "pnpm build") {
		t.Errorf("missing datasheets bundle should name datasheets.js and hint pnpm build, got %v", err)
	}
	writeEmpty(t, filepath.Join(dir, "static", "datasheets.js"))
	if err := checkWorkbenchAssets(dir); err == nil || !strings.Contains(err.Error(), "pdf.worker.js") {
		t.Errorf("missing pdf.js worker should name pdf.worker.js, got %v", err)
	}
	writeEmpty(t, filepath.Join(dir, "static", "pdf.worker.js"))
	if err := checkWorkbenchAssets(dir); err != nil {
		t.Errorf("a complete workbench should pass, got %v", err)
	}
	if err := checkWorkbenchAssets("../../web"); err != nil {
		t.Errorf("datasheet/web should carry the workbench, got %v", err)
	}
}

// TestWorkbenchServesItsWholePage drives agnids' real routes: the page shell, the folder tree over
// its own API, a raw PDF, the root redirect and the liveness probe. The tree going through
// DatasheetService rather than the engine's WorkspaceService is what lets agnids be hosted alone.
func TestWorkbenchServesItsWholePage(t *testing.T) {
	root := t.TempDir()
	writeEmpty(t, filepath.Join(root, "ti", "LM1117.pdf"))
	ms, err := mounts.Parse([]string{"ds=" + root})
	if err != nil {
		t.Fatal(err)
	}
	mux := newWorkbenchMux(ms, filepath.Join("..", "..", "web"), nil, "", nil)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	page := do("GET", "/datasheets/", "")
	if page.Code != 200 {
		t.Fatalf("GET /datasheets/ = %d", page.Code)
	}
	for _, want := range []string{`id="ds-tree"`, `id="ds-view"`, `data-component="ds-tree"`, "/static/datasheets.js", "Agni datasheets"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("workbench page missing %q", want)
		}
	}
	tree := do("POST", "/agni.v1.dsapi.DatasheetService/ListDir", `{"uri":"mount://ds/ti"}`)
	if tree.Code != 200 || !strings.Contains(tree.Body.String(), "LM1117.pdf") {
		t.Errorf("ListDir over the workbench's own API = %d %s, want LM1117.pdf listed", tree.Code, tree.Body.String())
	}
	mountsResp := do("POST", "/agni.v1.dsapi.DatasheetService/ListMounts", `{}`)
	if mountsResp.Code != 200 || !strings.Contains(mountsResp.Body.String(), `"ds"`) {
		t.Errorf("ListMounts = %d %s, want the ds mount", mountsResp.Code, mountsResp.Body.String())
	}
	if raw := do("GET", "/datasheets/raw/ds/ti/LM1117.pdf", ""); raw.Code != 200 || raw.Body.String() != "x" {
		t.Errorf("raw PDF = %d %q", raw.Code, raw.Body.String())
	}
	if home := do("GET", "/", ""); home.Code != 302 || home.Header().Get("Location") != "/datasheets/" {
		t.Errorf("GET / = %d -> %q, want a redirect to the workbench", home.Code, home.Header().Get("Location"))
	}
	if h := do("GET", "/healthz", ""); h.Code != 200 {
		t.Errorf("/healthz = %d", h.Code)
	}
}

// The workbench is hosted apart from the viewer, and "/" on agnids is the workbench itself, so the
// heading links home only when --viewer-url says where the viewer is.
func TestWorkbenchHeadingLinksTheViewerOnlyWhenConfigured(t *testing.T) {
	page := func(viewer string) string {
		mux := newWorkbenchMux(nil, filepath.Join("..", "..", "web"), nil, viewer, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/datasheets/", nil))
		if rec.Code != 200 {
			t.Fatalf("GET /datasheets/ = %d", rec.Code)
		}
		return rec.Body.String()
	}
	if with := page("http://viewer:8080"); !strings.Contains(with, `<a class="ld-home" href="http://viewer:8080/">Agni</a>`) {
		t.Error("with --viewer-url, the heading does not link to the viewer")
	}
	if without := page(""); strings.Contains(without, `class="ld-home"`) || !strings.Contains(without, "Agni / Datasheets") {
		t.Error("without --viewer-url, the heading should be plain text, since / on agnids is the workbench")
	}
}

// TestMountRootExposesEachFolder pins the image's zero-flag path: every subdirectory of
// --mount-root is a mount named after itself, a plain file beside them is not, and an explicit
// --mount of the same name wins over the discovered one.
func TestMountRootExposesEachFolder(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"ti", "adi"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()

	ms, err := serveMounts([]string{"ti=" + elsewhere}, root)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, m := range ms {
		got[m.Name] = m.Root
	}
	if len(got) != 2 {
		t.Fatalf("mounts = %v, want exactly ti and adi", got)
	}
	if got["adi"] != filepath.Join(root, "adi") {
		t.Errorf("adi = %q, want the discovered %q", got["adi"], filepath.Join(root, "adi"))
	}
	if got["ti"] != elsewhere {
		t.Errorf("ti = %q, want the explicit --mount %q to win", got["ti"], elsewhere)
	}
}

// TestAgnidsCarriesTheImageCommands pins the two subcommands the agnids image leans on: the release
// workflow checks `version` against the tag, and HEALTHCHECK runs `healthcheck`.
func TestAgnidsCarriesTheImageCommands(t *testing.T) {
	root := rootCmd()
	for _, name := range []string{"version", "healthcheck"} {
		if c, _, err := root.Find([]string{name}); err != nil || c.Name() != name {
			t.Errorf("agnids has no %q subcommand", name)
		}
	}
	if root.Version == "" {
		t.Error("root Version is empty, so --version is not registered")
	}
}
