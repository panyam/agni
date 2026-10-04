package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/mounts"
)

// rawFileHandler serves one file inside a mount at /raw/<mount>/<path>, for a page that analyses a
// design in the browser and fetches the files ListDesignFiles named (agni issue 853). It answers
// files only: a directory, an unknown mount and a path escaping its mount are all not found, since
// the listing is the only index a client gets.
func rawFileHandler(ms []mounts.Mount) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/raw/")
		mount, p, _ := strings.Cut(rest, "/")
		u, err := artifact.New(mount, p)
		if err != nil || p == "" {
			http.NotFound(w, r)
			return
		}
		abs, err := mounts.Resolve(ms, u)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(abs)
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
		http.ServeContent(w, r, st.Name(), st.ModTime(), f)
	})
}
