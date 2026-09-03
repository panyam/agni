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
	"path"

	"github.com/panyam/agni/artifact"
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

// FormatForExt returns the design format label for a file name, or "" when no reader understands
// it. It derives from the formats registry, so adding a reader there labels its extension here for
// free. Exported so an adapter can pre-filter if it wants; the service uses it.
func FormatForExt(name string) string {
	return formats.NameForExt(name)
}

// datasheetExt is the one extension the extraction workbench opens. It sits here rather than in
// readers/formats because that registry means "a DESIGN reader exists for this", and registering
// PDF there would make `agni stats part.pdf` look supported. This is the single definition: the
// raw-PDF endpoint and both browser trees ask this package rather than testing the suffix again.
const datasheetExt = ".pdf"

// KindForName returns which client opens a file. A design wins over a datasheet if an extension is
// somehow both, since the registry is the extensible half and a reader claiming an extension is a
// stronger statement than this package's one constant.
func KindForName(name string) webapi.FileKind {
	if FormatForExt(name) != "" {
		return webapi.FileKind_FILE_KIND_DESIGN
	}
	if strings.EqualFold(path.Ext(name), datasheetExt) {
		return webapi.FileKind_FILE_KIND_DATASHEET
	}
	return webapi.FileKind_FILE_KIND_UNSPECIFIED
}

// opensSet indexes a request's declared kinds. UNSPECIFIED is dropped rather than honoured: a
// client asking to open "the files nothing opens" would prune nothing, which is the same as not
// asking, and reading it as a real kind would make every folder of lock files look openable.
func opensSet(kinds []webapi.FileKind) map[webapi.FileKind]bool {
	if len(kinds) == 0 {
		return nil
	}
	set := make(map[webapi.FileKind]bool, len(kinds))
	for _, k := range kinds {
		if k != webapi.FileKind_FILE_KIND_UNSPECIFIED {
			set[k] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
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

// ListMounts returns the configured mounts in configuration order. It never errors. With Opens set,
// a mount holding none of those kinds anywhere beneath it is left out and counted in PrunedMounts,
// which is the same rule ListDir applies to subdirectories, applied to the roots: a mount serving
// only datasheets is a root the DESIGN tree can only ever show empty, and the datasheets tree asks
// the same question with the other answer.
//
// The walk budget is per call, shared by every mount, so a configuration of many roots cannot turn
// one page load into many full walks. Sharing it means a later mount can inherit an exhausted
// budget and be kept on a bound rather than on its contents, which is the direction that shows too
// much rather than too little.
func (s *WorkspaceService) ListMounts(ctx context.Context, req *webapi.ListMountsRequest) (*webapi.ListMountsResponse, error) {
	resp := &webapi.ListMountsResponse{}
	opens := opensSet(req.GetOpens())
	pruneBudget := pruneMaxDirs
	for _, m := range s.ws.Mounts() {
		uri, err := artifact.New(m.Name, "")
		if err != nil {
			return nil, fmt.Errorf("%w: mount %q cannot be addressed: %s", ErrInternal, m.Name, err)
		}
		if opens != nil && !s.hasOpenableFile(ctx, uri, opens, 0, &pruneBudget) {
			resp.PrunedMounts++
			continue
		}
		resp.Mounts = append(resp.Mounts, &webapi.Mount{Name: m.Name, Root: m.Root, Uri: uri.String()})
	}
	return resp, nil
}

// Bounds on the subtree walk the two prune flags ask for. The walk is depth-first with an early
// exit on the first readable design, so a normal design folder settles in a listing or two; the
// bounds are there for the pathological case, a vendored tree or a source checkout under a mount,
// where proving a folder empty means reading all of it. pruneMaxDirs is a per-request budget
// shared by every subdirectory in the listing, so one huge sibling cannot make the whole call slow.
const (
	pruneMaxDepth = 8
	pruneMaxDirs  = 2000
)

// hasOpenableFile reports whether u's subtree holds at least one file of a kind the caller opens.
// It reads through the Workspace port like everything else here (C13), never os, and stops at the
// first hit.
//
// It answers true when it cannot finish: a bound reached, a cancelled request, a directory the
// adapter refuses. A folder wrongly shown costs a click, a folder wrongly hidden costs a file, so
// the uncertain answer is the visible one.
func (s *WorkspaceService) hasOpenableFile(ctx context.Context, u artifact.URI, opens map[webapi.FileKind]bool, depth int, budget *int) bool {
	if depth > pruneMaxDepth || *budget <= 0 || ctx.Err() != nil {
		return true
	}
	*budget--
	entries, err := s.ws.ListDir(ctx, u)
	if err != nil {
		return true
	}
	// This level's files first, subdirectories after: a match one level down is the common case, and
	// finding it costs one listing instead of a walk to the bottom of the first branch.
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
		if opens[KindForName(de.Name)] {
			return true
		}
	}
	for _, d := range subdirs {
		if s.hasOpenableFile(ctx, d, opens, depth+1, budget) {
			return true
		}
	}
	return false
}

// ListDir lists one level of a mount: its subdirectories and its files, directories first and each
// group sorted by name, every file labeled with its format and the kind of client that opens it.
// With Opens set, a subdirectory holding none of those kinds anywhere beneath it is left out, so a
// tree stops offering folders it can only ever show empty. Errors are classified for the transport: a
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
	opens := opensSet(req.GetOpens())
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
			if opens != nil && !s.hasOpenableFile(ctx, entry, opens, 1, &pruneBudget) {
				continue
			}
			dirs = append(dirs, &webapi.DirEntry{Name: de.Name, Uri: entry.String(), IsDir: true})
			continue
		}
		// Every non-dotfile is listed, labeled with the kind of client that opens it (UNSPECIFIED
		// for one nothing opens), so each tree hides what it cannot open by reading the label rather
		// than by re-deriving the rule from the extension.
		files = append(files, &webapi.DirEntry{
			Name:   de.Name,
			Uri:    entry.String(),
			Format: FormatForExt(de.Name),
			Kind:   KindForName(de.Name),
		})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return &webapi.ListDirResponse{Entries: append(dirs, files...)}, nil
}
