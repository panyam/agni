package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/mounts"
)

// TestRawFileHandlerServesFilesInsideAMountOnly checks the read-only route's containment. A file in
// a mount is served. A directory, an unknown mount and a path climbing out of its mount are all not
// found.
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

// TestRawFileHandlerAsksTheBrowserToRevalidate checks that a file is served for revalidation (agni
// issue 1502). A browser engine fetches a design's files again on every page, and Jetson's board is
// 85 MB. With no-cache and a Last-Modified, a reload asks again and gets a 304 instead of the bytes.
func TestRawFileHandlerAsksTheBrowserToRevalidate(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(rawFileHandler([]mounts.Mount{{Name: "tut", Root: root}}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/raw/tut/project.yaml")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if got := res.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control %q, want no-cache", got)
	}
	modified := res.Header.Get("Last-Modified")
	if modified == "" {
		t.Fatal("no Last-Modified, so a browser has nothing to revalidate with")
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/raw/tut/project.yaml", nil)
	req.Header.Set("If-Modified-Since", modified)
	again, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	again.Body.Close()
	if again.StatusCode != http.StatusNotModified {
		t.Errorf("a revalidation answered %d, want 304", again.StatusCode)
	}
}
