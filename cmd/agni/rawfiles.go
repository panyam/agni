package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// rawFileHandler serves one file inside a mount at /raw/<mount>/<path>, for a page that analyses a
// design in the browser and fetches the files ListDesignFiles named (agni issue 853). It answers
// files only. A directory, an unknown mount and a path escaping its mount are all not found, since
// the listing is the only index a client gets.
func rawFileHandler(ms *mountset.Table, guard *service.AccessGuard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/raw/")
		mount, p, _ := strings.Cut(rest, "/")
		u, err := mounts.NewURI(mount, p)
		if err != nil || p == "" {
			http.NotFound(w, r)
			return
		}
		if err := guard.URI(r.Context(), u); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		abs, err := ms.Resolve(u)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(abs) // #nosec G304 G703 -- abs comes from mountset.Resolve, which refuses a path outside its mount
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		// A browser engine fetches the design's files on every page (agni issue 1502). no-cache keeps
		// a copy but asks first, so a reload of an unchanged 85 MB board gets a 304 from ServeContent's
		// Last-Modified.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
	})
}
