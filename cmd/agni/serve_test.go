package main

import (
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

	// All four present: valid.
	touch(t, filepath.Join(dir, "static", "datasheets.js"))
	if err := checkWebAssets(dir); err != nil {
		t.Errorf("a dir with both templates and both bundles should pass, got %v", err)
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
