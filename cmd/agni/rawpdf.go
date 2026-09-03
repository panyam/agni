package main

import (
	"net/http"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
)

// rawDatasheetHandler streams a datasheet's source bytes (the PDF the browser renders under the
// region overlay on the /datasheets page, WS13-006) from a mount, so the page can load it into
// pdf.js. It is mounted under /datasheets/raw/<mount>/<path...>; mounts are the security boundary
// (mounts.Resolve contains the path), and only .pdf files are served — doc-IR is served
// structured over DatasheetService, never raw. Rendering stays in the browser, so nothing about
// the document leaves the deployment boundary beyond the local client (C16).
func rawDatasheetHandler(ms []mounts.Mount) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is already stripped of the /datasheets/raw/ prefix, leaving "<mount>/<path...>".
		mountName, rel, ok := strings.Cut(r.URL.Path, "/")
		if !ok || mountName == "" || rel == "" {
			http.Error(w, "raw datasheet path must be <mount>/<path>", http.StatusBadRequest)
			return
		}
		// The service owns what counts as a datasheet, so this endpoint and the browser trees cannot
		// drift on it. It used to test the suffix here, which was the second copy of that rule.
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
			// Unknown mount or a path escaping it: do not distinguish, do not echo the host path.
			http.Error(w, "no such datasheet", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		http.ServeFile(w, r, abs)
	})
}
