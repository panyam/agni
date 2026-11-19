// Package service holds the importable, transport-neutral service implementations
// (CONSTRAINTS C13). Every method has a plain protobuf signature
// ((ctx, *pb.XRequest) (*pb.XResponse, error)) and classifies its errors with this package's
// sentinels. The transports (Connect, in internal/server) are thin translation layers over them.
// Each implementation depends on injected I/O ports (Workspace, Loader, NativeRenderer) and never
// on os, so the same code runs on the server, in tests, and in WASM (C1). All file
// I/O lives in cmd/agni's OS-backed adapters, which also wire the services.
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

// ErrInvalidPath marks a containment or argument violation an adapter returns, such as a path
// escaping its mount. Transports map it to their invalid-argument code. Adapters wrap it so the
// service can classify without importing os.
var ErrInvalidPath = errors.New("invalid path")

// MountInfo identifies one browseable root the workspace exposes.
type MountInfo struct {
	Name string
	Root string
	// Kind, Source, Ref and Commit are copied into webapi.Mount as they are.
	Kind   webapi.MountKind
	Source string
	Ref    string
	Commit string
	// Fixed marks a mount from the server's flags, and Writable one the server writes into.
	Fixed    bool
	Writable bool
}

// DirEntry is one raw filesystem entry, before the service labels a file with its design format.
type DirEntry struct {
	Name  string
	IsDir bool
}

// Workspace is the file-browsing port, serving the configured mounts and one level of listing
// within a mount. Path containment is the adapter's responsibility. The server adapter is
// os-backed (mounts.Workspace) and the in-browser engine's is fshost's.
type Workspace interface {
	Mounts() []MountInfo
	ListDir(ctx context.Context, uri artifact.URI) ([]DirEntry, error)
}

// FormatForExt returns the design format label for a file name, or "" when no reader understands
// it. It derives from the formats registry, so adding a reader there labels its extension here.
// Exported so an adapter can pre-filter.
func FormatForExt(name string) string {
	return formats.NameForExt(name)
}

// datasheetExt is the one extension the extraction workbench opens. It is not in readers/formats
// because that registry means "a DESIGN reader exists for this", and registering PDF there would
// make `agni stats part.pdf` look supported. The raw-PDF endpoint and both browser trees ask
// KindForName rather than testing the suffix themselves.
const datasheetExt = ".pdf"

// KindForName returns which client opens a file. A design wins over a datasheet if an extension is
// both, since a reader registered for an extension outranks this package's one constant.
func KindForName(name string) webapi.FileKind {
	if FormatForExt(name) != "" {
		return webapi.FileKind_FILE_KIND_DESIGN
	}
	if strings.EqualFold(path.Ext(name), datasheetExt) {
		return webapi.FileKind_FILE_KIND_DATASHEET
	}
	return webapi.FileKind_FILE_KIND_UNSPECIFIED
}

// opensSet indexes a request's declared kinds, or returns nil for "no filter". UNSPECIFIED is
// dropped, because reading it as a real kind would make every folder of lock files look openable.
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
// port (CONSTRAINTS C13). It maps mounts and directory listings to the proto responses and does no
// file I/O itself.
type WorkspaceService struct {
	ws Workspace
	// projects and files serve ListDesignFiles, and are set by WithDesignFiles.
	projects *ProjectResolver
	files    FileReader
	// maker makes and deletes runtime mounts, and is set by WithMountMaker.
	maker MountMaker
	// linked names the artifacts linked to a mount, and is set by WithLinkedArtifacts.
	linked func(ctx context.Context, mount string) ([]string, error)
}

// MountMaker is the port that makes and deletes runtime mounts (agni issue 1735). The server's
// adapter snapshots the source into --uploads and keeps a record in --store. It classifies its
// errors with this package's sentinels: ErrInvalidArgument for a bad name or source, ErrNotFound
// for a source or ref that does not exist, ErrResourceExhausted for a source over the size cap, and
// ErrFailedPrecondition for a mount it may not delete.
type MountMaker interface {
	CreateMount(ctx context.Context, name, source, ref string) (MountInfo, error)
	DeleteMount(ctx context.Context, name string) error
}

// WithMountMaker lets the service make and delete runtime mounts through m. Without it, both RPCs
// are ErrFailedPrecondition.
func (s *WorkspaceService) WithMountMaker(m MountMaker) *WorkspaceService {
	s.maker = m
	return s
}

// WithLinkedArtifacts lets DeleteMount refuse a mount that artifacts are linked to (agni issue
// 1723). linked names the artifacts whose source is the mount.
func (s *WorkspaceService) WithLinkedArtifacts(linked func(ctx context.Context, mount string) ([]string, error)) *WorkspaceService {
	s.linked = linked
	return s
}

// listSome joins up to n names, and says how many more there are.
func listSome(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:n], ", "), len(names)-n)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// noMaker is why a server without a MountMaker refuses CreateMount and DeleteMount.
const noMaker = "this server makes no mounts at runtime; start it with --store and --uploads"

// CreateMount makes a runtime mount through the MountMaker. A name the server already serves is
// ErrAlreadyExists, checked here as well as in the adapter so the answer does not depend on it.
func (s *WorkspaceService) CreateMount(ctx context.Context, req *webapi.CreateMountRequest) (*webapi.Mount, error) {
	if s.maker == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noMaker)
	}
	for _, m := range s.ws.Mounts() {
		if m.Name == req.GetName() {
			return nil, fmt.Errorf("%w: a mount named %q is already served", ErrAlreadyExists, m.Name)
		}
	}
	info, err := s.maker.CreateMount(ctx, req.GetName(), req.GetSource(), req.GetRef())
	if err != nil {
		return nil, err
	}
	return mountProto(info)
}

// DeleteMount deletes a runtime mount through the MountMaker. A mount from the flags is
// ErrFailedPrecondition and an unknown name ErrNotFound.
func (s *WorkspaceService) DeleteMount(ctx context.Context, req *webapi.DeleteMountRequest) (*webapi.DeleteMountResponse, error) {
	if s.maker == nil {
		return nil, fmt.Errorf("%w: %s", ErrFailedPrecondition, noMaker)
	}
	for _, m := range s.ws.Mounts() {
		if m.Name != req.GetName() {
			continue
		}
		if m.Fixed {
			return nil, fmt.Errorf("%w: the mount %q comes from the server's flags", ErrFailedPrecondition, m.Name)
		}
		if s.linked != nil {
			names, err := s.linked(ctx, m.Name)
			if err != nil {
				return nil, err
			}
			if len(names) > 0 {
				return nil, fmt.Errorf("%w: %s %s linked to the mount %q; unlink or delete %s first", ErrFailedPrecondition, listSome(names, 3), plural(len(names), "is", "are"), m.Name, plural(len(names), "it", "them"))
			}
		}
		if err := s.maker.DeleteMount(ctx, m.Name); err != nil {
			return nil, err
		}
		return &webapi.DeleteMountResponse{}, nil
	}
	return nil, fmt.Errorf("%w: no mount named %q", ErrNotFound, req.GetName())
}

// mountProto is m as the wire carries it.
func mountProto(m MountInfo) (*webapi.Mount, error) {
	uri, err := artifact.New(m.Name, "")
	if err != nil {
		return nil, fmt.Errorf("%w: mount %q cannot be addressed: %s", ErrInternal, m.Name, err)
	}
	return &webapi.Mount{Name: m.Name, Root: m.Root, Uri: uri.String(), Kind: m.Kind, Source: m.Source, Ref: m.Ref, Commit: m.Commit, Fixed: m.Fixed, Writable: m.Writable}, nil
}

// NewWorkspaceService returns a WorkspaceService backed by ws.
func NewWorkspaceService(ws Workspace) *WorkspaceService {
	return &WorkspaceService{ws: ws}
}

// ListMounts returns the configured mounts in configuration order. It never errors. With Opens set,
// a mount holding none of those kinds anywhere beneath it is left out and counted in PrunedMounts.
// ListDir applies the same rule to subdirectories. A mount serving only datasheets is thus hidden
// from the DESIGN tree and shown in the datasheets tree.
//
// The walk budget is per call and shared by every mount, so many roots cannot turn one page load
// into many full walks. A later mount can inherit an exhausted budget and be kept without being
// examined, which errs toward showing too much.
func (s *WorkspaceService) ListMounts(ctx context.Context, req *webapi.ListMountsRequest) (*webapi.ListMountsResponse, error) {
	resp := &webapi.ListMountsResponse{CanCreateMounts: s.maker != nil}
	opens := opensSet(req.GetOpens())
	pruneBudget := pruneMaxDirs
	for _, m := range s.ws.Mounts() {
		mp, err := mountProto(m)
		if err != nil {
			return nil, err
		}
		uri, _ := artifact.New(m.Name, "")
		if opens != nil && !s.hasOpenableFile(ctx, uri, opens, 0, &pruneBudget) {
			resp.PrunedMounts++
			continue
		}
		resp.Mounts = append(resp.Mounts, mp)
	}
	return resp, nil
}

// GetMount returns the mount named req.Name, as ListMounts would list it with no pruning. A name
// the server does not serve, the empty name included, is ErrNotFound.
func (s *WorkspaceService) GetMount(_ context.Context, req *webapi.GetMountRequest) (*webapi.Mount, error) {
	for _, m := range s.ws.Mounts() {
		if m.Name != req.GetName() {
			continue
		}
		return mountProto(m)
	}
	return nil, fmt.Errorf("%w: no mount named %q", ErrNotFound, req.GetName())
}

// The two prune flags ask for a subtree walk, and these bound it. The walk is depth-first and exits
// on the first openable file, so a normal design folder settles in a listing or two. The bounds are
// for a vendored tree or a source checkout under a mount, where proving a folder empty means reading
// all of it. pruneMaxDirs is a per-request budget shared by every subdirectory in the listing, so one
// huge sibling cannot make the whole call slow.
const (
	pruneMaxDepth = 8
	pruneMaxDirs  = 2000
)

// hasOpenableFile reports whether u's subtree holds at least one file of a kind the caller opens.
// It reads through the Workspace port (C13) and stops at the first hit.
//
// It answers true when it cannot finish (a bound reached, a cancelled request, a directory the
// adapter refuses). A folder wrongly shown costs a click and a folder wrongly hidden costs a file,
// so the uncertain answer is the visible one.
// isODBJob reports whether a directory holds an ODB++ job, a matrix/matrix file. A board reader
// enters the job by that file, and the directory names the job (agni issue 1250).
func (s *WorkspaceService) isODBJob(ctx context.Context, dir artifact.URI) bool {
	m, err := dir.Join("matrix")
	if err != nil {
		return false
	}
	entries, err := s.ws.ListDir(ctx, m)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name == "matrix" && !e.IsDir {
			return true
		}
	}
	return false
}

func (s *WorkspaceService) hasOpenableFile(ctx context.Context, u artifact.URI, opens map[webapi.FileKind]bool, depth int, budget *int) bool {
	if depth > pruneMaxDepth || *budget <= 0 || ctx.Err() != nil {
		return true
	}
	*budget--
	entries, err := s.ws.ListDir(ctx, u)
	if err != nil {
		return true
	}
	// This level's files first, subdirectories after, since a match one level down is the common
	// case and costs one listing instead of a walk to the bottom of the first branch.
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
// tree stops offering folders it can only ever show empty. A containment violation keeps
// ErrInvalidPath, and anything else from the adapter (unknown mount, missing directory) is wrapped
// as ErrNotFound.
func (s *WorkspaceService) ListDir(ctx context.Context, req *webapi.ListDirRequest) (*webapi.ListDirResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
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
			continue
		}
		entry, joinErr := u.Join(de.Name)
		if joinErr != nil {
			// A name the mount produced cannot escape it. If one ever did, skip it rather than serve
			// an entry nothing can open.
			continue
		}
		if de.IsDir {
			// An ODB++ job is a design entered by its matrix/matrix file, whose name gives no kind, so
			// the subtree walk alone would prune every job folder (agni issue 1367).
			job := (req.GetMarkBoards() || opens[webapi.FileKind_FILE_KIND_DESIGN]) && s.isODBJob(ctx, entry)
			if opens != nil && !(job && opens[webapi.FileKind_FILE_KIND_DESIGN]) && !s.hasOpenableFile(ctx, entry, opens, 1, &pruneBudget) {
				continue
			}
			d := &webapi.DirEntry{Name: de.Name, Uri: entry.String(), IsDir: true}
			if req.GetMarkBoards() && job {
				if m, err := entry.Join("matrix", "matrix"); err == nil {
					d.BoardUri = m.String()
				}
			}
			dirs = append(dirs, d)
			continue
		}
		// Every non-dotfile is listed with the kind of client that opens it (UNSPECIFIED when
		// nothing does), so each tree filters on the label rather than on the extension.
		files = append(files, &webapi.DirEntry{
			Name:   de.Name,
			Uri:    entry.String(),
			Format: FormatForExt(de.Name),
			Kind:   KindForName(de.Name),
		})
		if req.GetMarkBoards() && formats.HasBoard(de.Name) {
			files[len(files)-1].BoardUri = entry.String()
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	return &webapi.ListDirResponse{Entries: append(dirs, files...)}, nil
}
