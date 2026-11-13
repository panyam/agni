package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
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

	versioned := newSeeds(ws, ms, func() string { return "v1" })
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
	if resp := get(newSeeds(ws, ms, func() string { return "" }), "/seed/tut/designs/gateway"); resp.StatusCode != http.StatusNoContent {
		t.Errorf("an unversioned binary answered %d, want 204 with no seed", resp.StatusCode)
	}
}

// A first browser-engine visit waited about 860 ms on the server building Jetson's seed (agni issue
// 1572). A request that names the design now starts the build, and the seed's own request waits on that
// build rather than starting a second.
func TestARequestNamingADesignWarmsItsSeedOnce(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: "tut", Root: root}}
	s := newSeeds(servedAPIFor(t, nil, ms).Workspace, ms, func() string { return "v1" })

	hook := warmSeeds(s)(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&webapi.ListDesignFilesResponse{}), nil
	})
	if _, err := hook(context.Background(), connect.NewRequest(&webapi.ListDesignFilesRequest{Uri: "mount://tut/designs/gateway"})); err != nil {
		t.Fatal(err)
	}
	// The seed builds with no request for it.
	built := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.kept) == 1
	}
	for deadline := time.Now().Add(30 * time.Second); !built(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("listing the gateway built no seed")
		}
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/seed/tut/designs/gateway")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("the warmed seed: %d, %d bytes", resp.StatusCode, len(body))
	}
	if n := s.builds.Load(); n != 1 {
		t.Errorf("the listing and the seed's request built %d seeds, want 1", n)
	}
}

// Only a listing or a server read of a design on a mount warms a seed. A design sent as files has none.
func TestOnlyAListingOrAReadOfAMountedDesignWarmsASeed(t *testing.T) {
	const gateway = "mount://tut/designs/gateway"
	cases := []struct {
		name string
		msg  any
		want bool
	}{
		{"a listing", &webapi.ListDesignFilesRequest{Uri: gateway}, true},
		{"a read", &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway}}, true},
		{"a design sent as files", &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: gateway, Files: []*webapi.SnapshotFile{{}}}}, false},
		{"another request", &webapi.GetSheetRequest{}, false},
		{"a listing of no design", &webapi.ListDesignFilesRequest{}, false},
	}
	for _, c := range cases {
		u, ok := designNamed(c.msg)
		if ok != c.want {
			t.Errorf("%s: names a design %v, want %v", c.name, ok, c.want)
		}
		if ok && (u.Mount != "tut" || u.Path != "designs/gateway") {
			t.Errorf("%s: names %+v", c.name, u)
		}
	}
}
