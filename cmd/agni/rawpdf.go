package main

import (
	"net/http"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
)

// rawDatasheetHandler streams a datasheet's PDF from a mount so the /datasheets page can render it
// in pdf.js under the region overlay (WS13-006). It is mounted under /datasheets/raw/<mount>/<path...>,
// and mounts.Resolve keeps the path inside its mount. Only .pdf files are served, since doc-IR goes
// structured over DatasheetService. Rendering stays in the browser, so the document goes no
// further than the local client (C16).
func rawDatasheetHandler(ms []mounts.Mount) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is already stripped of the /datasheets/raw/ prefix, leaving "<mount>/<path...>".
		mountName, rel, ok := strings.Cut(r.URL.Path, "/")
		if !ok || mountName == "" || rel == "" {
			http.Error(w, "raw datasheet path must be <mount>/<path>", http.StatusBadRequest)
			return
		}
		// The service owns what counts as a datasheet, so this endpoint and the browser trees cannot
		// drift on it (#319).
		if service.KindForName(rel) != webapi.FileKind_FILE_KIND_DATASHEET {
			http.Error(w, "only .pdf datasheets are served raw", http.StatusBadRequest)
			return
		}
		uri, err := artifact.New(mountName, rel)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		abs, err := mounts.Resolve(ms, uri)
		if err != nil {
			// An unknown mount and an escaping path get the same answer, and the host path is never echoed.
			http.Error(w, "no such datasheet", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, abs)
	})
}
