package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// seedHandler serves /seed/<mount>/<path>, a design's store as a browser worker would write it
// (agni issue 1525). A page analysing the design in the browser puts it into its store before the
// first read, and restores the parsed board rather than fetching and parsing it.
//
// It builds the seed natively over exactly the files ListDesignFiles names, under this binary's
// wasmengine.StoreVersion. The page's worker takes it only under the same version, so a server built
// from another tree than the page's agni.wasm costs one wasted fetch and nothing else. A binary that
// cannot name its version answers 204 with no seed. A built seed is kept per version and file set.
func seedHandler(ws *service.WorkspaceService, ms []mounts.Mount, version func() string) http.Handler {
	var mu sync.Mutex
	kept := map[string][]byte{}
	var order []string
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := version()
		if v == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mount, p, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/seed/"), "/")
		u, err := artifact.New(mount, p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		listed, err := ws.ListDesignFiles(r.Context(), &webapi.ListDesignFilesRequest{Uri: u.String()})
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		id := []string{v, u.String()}
		for _, f := range listed.GetFiles() {
			id = append(id, f.GetPath(), f.GetSha256())
		}
		key := strings.Join(id, "\x00")
		mu.Lock()
		seed, ok := kept[key]
		mu.Unlock()
		if !ok {
			files := make([]wasmengine.SeedFile, 0, len(listed.GetFiles()))
			for _, f := range listed.GetFiles() {
				fu, err := artifact.New(mount, f.GetPath())
				if err != nil {
					http.NotFound(w, r)
					return
				}
				abs, err := mounts.Resolve(ms, fu)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				b, err := os.ReadFile(abs) // #nosec G304 G703 -- abs comes from mounts.Resolve, which refuses a path outside its mount
				if err != nil {
					http.NotFound(w, r)
					return
				}
				files = append(files, wasmengine.SeedFile{Path: f.GetPath(), Bytes: b})
			}
			seed, err = wasmengine.Seed(r.Context(), mount, u.String(), files, v)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			mu.Lock()
			kept[key] = seed
			order = append(order, key)
			for len(order) > 4 {
				delete(kept, order[0])
				order = order[1:]
			}
			mu.Unlock()
		}
		w.Header().Set(wasmengine.SeedVersionHeader, v)
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(seed))
	})
}
