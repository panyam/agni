package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckWebAssets asserts serve validates its assets dir upfront: a folder with no viewer
// template points at --mount (the misdirected-arg case), a template without the bundle hints to
// build it, and the repo's own web/ passes.
func TestCheckWebAssets(t *testing.T) {
	dir := t.TempDir()

	// No templates: the "you passed a design folder" case.
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "--mount") {
		t.Errorf("empty dir should error mentioning --mount, got %v", err)
	}

	// Template present but no built bundle: hint to build.
	touch(t, filepath.Join(dir, "templates", "ViewerPage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "pnpm build") {
		t.Errorf("missing bundle should hint pnpm build, got %v", err)
	}

	// Viewer template + bundle present, but the datasheets workbench page is missing (WS13-006).
	touch(t, filepath.Join(dir, "static", "app.js"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "DatasheetsPage.html") {
		t.Errorf("missing datasheets page should name DatasheetsPage.html, got %v", err)
	}

	// Datasheets template present but its bundle missing: hint to build.
	touch(t, filepath.Join(dir, "templates", "DatasheetsPage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "datasheets.js") {
		t.Errorf("missing datasheets bundle should name datasheets.js, got %v", err)
	}

	// Datasheets complete, but the browse page is missing (WS9-049 phase 2).
	touch(t, filepath.Join(dir, "static", "datasheets.js"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "BrowsePage.html") {
		t.Errorf("missing browse page should name BrowsePage.html, got %v", err)
	}

	// Browse template present but its bundle missing: hint to build.
	touch(t, filepath.Join(dir, "templates", "BrowsePage.html"))
	if err := checkWebAssets(dir); err == nil || !strings.Contains(err.Error(), "browse.js") {
		t.Errorf("missing browse bundle should name browse.js, got %v", err)
	}

	// All three templates and all three bundles present: valid.
	touch(t, filepath.Join(dir, "static", "browse.js"))
	if err := checkWebAssets(dir); err != nil {
		t.Errorf("a dir with every template and bundle should pass, got %v", err)
	}

	// The repo's web/ dir passes (asserts the marker paths match the real layout).
	if err := checkWebAssets("../../web"); err != nil {
		t.Errorf("repo web/ should pass checkWebAssets, got %v", err)
	}
}

// touch writes an empty file, creating parent directories.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestServeRejectsUnknownTheme pins the flag contract: an unknown --theme errors up front
// listing the valid palettes (it used to fall back to the default silently, unlike every
// other validated enum flag).
func TestServeRejectsUnknownTheme(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"serve", "--theme", "solarized", "definitely-missing-dir"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unknown --theme "solarized"`) || !strings.Contains(err.Error(), "default") {
		t.Fatalf("want an unknown-theme error naming the valid palettes, got %v", err)
	}
}

// TestHealthHandler asserts the probe answers 200 with a body, and that it is registered on the
// exact path rather than as a prefix — a "/healthz/" subtree would quietly swallow page routes.
func TestHealthHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthHandler())
	// Stand-in for the page space that registerPages puts at "/", so a prefix-shadowing
	// regression shows up as this handler never being reached.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "page")
	})

	t.Run("200 with a body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != "ok" {
			t.Fatalf("got body %q, want ok", rec.Body.String())
		}
	})

	t.Run("does not shadow the page space", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz/sub", nil))
		if rec.Body.String() != "page" {
			t.Fatalf("/healthz/sub reached the probe, so it is registered as a subtree: %q", rec.Body.String())
		}
	})
}
