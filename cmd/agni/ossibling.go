package main

import (
	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/internal/mounts"
)

// resolveSibling resolves a file DERIVED from another artifact's path — a datasheet's doc-IR beside
// its PDF, a PartSpec beside its document, an annotations directory — to an absolute host path.
//
// The derivation is a pure function of the mount-relative path (`docSibling`, `partSpecSibling`,
// `annotationsDir`), and the result is rebuilt through artifact.New rather than pasted onto a
// string, so a derivation that ever produced something escaping the mount is caught at the same
// boundary as a client-supplied URI instead of at whichever adapter forgot to look.
func resolveSibling(ms []mounts.Mount, uri artifact.URI, derive func(string) string) (string, error) {
	sib, err := artifact.New(uri.Mount, derive(uri.Path))
	if err != nil {
		return "", err
	}
	return mounts.Resolve(ms, sib)
}
