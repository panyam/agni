package service

import (
	"context"
	"strings"
)

// linkMountPath is the mount path a saved report's links open, for the design the caller named in
// ReportLinks.design_uri, or "" when it named none. A link names the design's declared ENTRY whatever
// the caller named (agni issue 489), and the design's folder or a declared companion resolves to it
// here, once, for every client. Before agni issue 1120 the viewer sent its page's address, the
// design's folder, while the CLI sent the entry, so the same report saved from the two linked
// different addresses.
//
// A ref that belongs to no declared design keeps its own path, as ProjectResolver.Sources answers,
// and so does any ref when the resolution fails, since a report about a design already read should
// keep its links rather than drop them.
func linkMountPath(ctx context.Context, projects *ProjectResolver, designURI string) (string, error) {
	if designURI == "" {
		return "", nil
	}
	u, err := ParseArtifactURI(designURI)
	if err != nil {
		return "", err
	}
	target := u.String()
	if res, err := projects.Sources(ctx, u, false); err == nil && res.NetlistURI != "" {
		target = res.NetlistURI
	}
	return strings.TrimPrefix(target, "mount://"), nil
}
