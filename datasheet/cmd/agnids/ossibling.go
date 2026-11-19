package main

import (
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/mountset"
)

// resolveSibling resolves a file DERIVED from another artifact's path (a datasheet's doc-IR beside
// its PDF, an annotations directory) to an absolute host path.
//
// The derivation is a pure function of the mount-relative path (`docSibling`, `annotationsDir`).
// The result is rebuilt through mounts.NewURI rather than pasted onto a string. So a derivation that
// escapes the mount is caught at the same boundary as a client-supplied URI.
func resolveSibling(ms *mountset.Table, uri mounts.URI, derive func(string) string) (string, error) {
	sib, err := mounts.NewURI(uri.Mount, derive(uri.Path))
	if err != nil {
		return "", err
	}
	return ms.Resolve(sib)
}
