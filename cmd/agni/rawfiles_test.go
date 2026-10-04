package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/mounts"
)

// TestRawFileHandlerServesFilesInsideAMountOnly is the read-only route's containment: a file in a
// mount is served, and a directory, an unknown mount and a path climbing out of its mount are all
// not found.
func TestRawFileHandlerServesFilesInsideAMountOnly(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rawFileHandler([]mounts.Mount{{Name: "tut", Root: root}}))
	defer srv.Close()
	get := func(p string) (int, string) {
		t.Helper()
		res, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, body := get("/raw/tut/project.yaml"); code != http.StatusOK || len(body) == 0 {
		t.Errorf("project.yaml: %d, %d bytes", code, len(body))
	}
	for _, p := range []string{"/raw/tut/designs", "/raw/nope/project.yaml", "/raw/tut/../tutorial-project/project.yaml", "/raw/tut/%2e%2e/tutorial-project/project.yaml", "/raw/tut/"} {
		if code, _ := get(p); code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", p, code)
		}
	}
}
