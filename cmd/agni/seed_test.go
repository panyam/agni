package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/mounts"
)

// TestTheSeedRouteServesADesignsStoreUnderItsVersion covers agni issue 1525's server half. A design
// gets its seed under this binary's version, and an unknown design does not. A binary that cannot
// name its version answers with no seed.
func TestTheSeedRouteServesADesignsStoreUnderItsVersion(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: "tut", Root: root}}
	ws := servedAPIFor(t, nil, ms).Workspace
	get := func(h http.Handler, p string) *http.Response {
		t.Helper()
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	versioned := seedHandler(ws, ms, func() string { return "v1" })
	resp := get(versioned, "/seed/tut/designs/gateway")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("the gateway's seed: %d, %d bytes", resp.StatusCode, len(body))
	}
	if got := resp.Header.Get(wasmengine.SeedVersionHeader); got != "v1" {
		t.Errorf("the seed names version %q, want v1", got)
	}
	if resp := get(versioned, "/seed/tut/designs/nothing-here"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("an unknown design answered %d, want 404", resp.StatusCode)
	}
	if resp := get(seedHandler(ws, ms, func() string { return "" }), "/seed/tut/designs/gateway"); resp.StatusCode != http.StatusNoContent {
		t.Errorf("an unversioned binary answered %d, want 204 with no seed", resp.StatusCode)
	}
}
