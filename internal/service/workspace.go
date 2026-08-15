// Package service holds the importable, transport-neutral service implementations
// (CONSTRAINTS C13). Every method carries a plain protobuf signature
// ((ctx, *pb.XRequest) (*pb.XResponse, error)) and classifies its errors with this package's
// sentinels; the transports — Connect today (internal/server), grpc-gateway or a real gRPC
// server later — are thin translation layers over them. Each implementation depends on
// injected I/O ports (Workspace, Loader, NativeRenderer), never on os directly, so the same
// code runs on the server, in tests, and later in WASM. cmd/agni provides the OS-backed
// adapters and wires them.
package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"sort"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
)

// ErrInvalidPath marks a containment/argument violation an FS/Workspace adapter returns (e.g. a
// path escaping the mount); transports map it to their invalid-argument code. Adapters wrap it so
// the service can classify without importing os.
var ErrInvalidPath = errors.New("invalid path")

// MountInfo identifies one browseable root the workspace exposes.
type MountInfo struct {
	Name string
	Root string
}

// DirEntry is one raw filesystem entry, before the service labels a file with its design format.
type DirEntry struct {
	Name  string
	IsDir bool
}

// Workspace is the file-browsing port: the configured mounts, and one level of listing within a
// mount. Path containment is the adapter's responsibility. The server adapter is os-backed; there
// is deliberately no WASM adapter, since a seeded single-design instance has no folder to browse
// (WS9-011 discussion).
type Workspace interface {
	Mounts() []MountInfo
	ListDir(ctx context.Context, uri artifact.URI) ([]DirEntry, error)
}

// FormatForExt returns the design format label for a file name, or "" when the tree should
// show it disabled (no reader). It derives from the formats registry, so adding a reader
// there labels its extension here for free. Exported so an adapter can pre-filter if it
// wants; the service uses it.
func FormatForExt(name string) string {
	return formats.NameForExt(name)
}

// WorkspaceService serves the mounted folders and their contents over an injected Workspace
// port (CONSTRAINTS C13): it maps mounts and directory listings to the proto responses and does
// no file I/O itself.
type WorkspaceService struct {
	ws Workspace
}

// NewWorkspaceService returns a WorkspaceService backed by ws.
func NewWorkspaceService(ws Workspace) *WorkspaceService {
	return &WorkspaceService{ws: ws}
}

// ListMounts returns the configured mounts in configuration order. It never errors.
func (s *WorkspaceService) ListMounts(_ context.Context, _ *webapi.ListMountsRequest) (*webapi.ListMountsResponse, error) {
	resp := &webapi.ListMountsResponse{}
	for _, m := range s.ws.Mounts() {
		uri, err := artifact.New(m.Name, "")
		if err != nil {
			return nil, fmt.Errorf("%w: mount %q cannot be addressed: %s", ErrInternal, m.Name, err)
		}
		resp.Mounts = append(resp.Mounts, &webapi.Mount{Name: m.Name, Root: m.Root, Uri: uri.String()})
	}
	return resp, nil
}

// Bounds on the subtree walk prune_empty_dirs asks for. The walk is depth-first with an early
// exit on the first readable design, so a normal design folder settles in a listing or two; the
// bounds are there for the pathological case, a vendored tree or a source checkout under a mount,
// where proving a folder empty means reading all of it. pruneMaxDirs is a per-request budget
// shared by every subdirectory in the listing, so one huge sibling cannot make the whole call slow.
const (
	pruneMaxDepth = 8
	pruneMaxDirs  = 2000
)

// hasDesignFile reports whether u's subtree holds at least one file a reader understands. It
// reads through the Workspace port like everything else here (C13), never os, and stops at the
// first hit.
//
// It answers true when it cannot finish: a bound reached, a cancelled request, a directory the
// adapter refuses. A folder wrongly shown costs a click, a folder wrongly hidden costs a design,
// so the uncertain answer is the visible one.
func (s *WorkspaceService) hasDesignFile(ctx context.Context, u artifact.URI, depth int, budget *int) bool {
	if depth > pruneMaxDepth || *budget <= 0 || ctx.Err() != nil {
		return true
	}
	*budget--
	entries, err := s.ws.ListDir(ctx, u)
	if err != nil {
		return true
	}
	// This level's files first, subdirectories after: a design one level down is the common case,
	// and finding it costs one listing instead of a walk to the bottom of the first branch.
	var subdirs []artifact.URI
	for _, de := range entries {
		if strings.HasPrefix(de.Name, ".") {
			continue
		}
		child, joinErr := u.Join(de.Name)
		if joinErr != nil {
			continue
		}
		if de.IsDir {
			subdirs = append(subdirs, child)
			continue
		}
		if FormatForExt(de.Name) != "" {
			return true
		}
	}
	for _, d := range subdirs {
		if s.hasDesignFile(ctx, d, depth+1, budget) {
			return true
		}
	}
	return false
}

// ListDir lists one level of a mount: its subdirectories and the design files a reader
// understands, directories first and each group sorted by name. With PruneEmptyDirs set, a
// subdirectory with no readable design anywhere beneath it is left out, so the tree stops
// offering folders it can only ever show empty. Errors are classified for the transport: a
// containment violation keeps ErrInvalidPath; anything else from the adapter (unknown mount,
// missing directory) is wrapped as ErrNotFound.
func (s *WorkspaceService) ListDir(ctx context.Context, req *webapi.ListDirRequest) (*webapi.ListDirResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	entries, err := s.ws.ListDir(ctx, u)
	if err != nil {
		if errors.Is(err, ErrInvalidPath) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", ErrNotFound, err)
	}

	var dirs, files []*webapi.DirEntry
	pruneBudget := pruneMaxDirs
	for _, de := range entries {
		if strings.HasPrefix(de.Name, ".") {
			continue // skip dotfiles/dirs
		}
		entry, joinErr := u.Join(de.Name)
		if joinErr != nil {
			// A name the mount itself produced cannot escape it; if one ever did, skipping it is
			// better than serving a listing entry nothing can open.
			continue
		}
		if de.IsDir {
			if req.GetPruneEmptyDirs() && !s.hasDesignFile(ctx, entry, 1, &pruneBudget) {
				continue
			}
			dirs = append(dirs, &webapi.DirEntry{Name: de.Name, Uri: entry.String(), IsDir: true})
			continue
		}
		// Every non-dotfile is listed, with an unrecognized one carrying an empty format, so each
		// client decides what to show: the design tree hides them, the datasheets tree keeps the
		// PDFs among them.
		files = append(files, &webapi.DirEntry{Name: de.Name, Uri: entry.String(), Format: FormatForExt(de.Name)})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return &webapi.ListDirResponse{Entries: append(dirs, files...)}, nil
}
