package service

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// SourceAccess says which mounts read a private source, and whether the caller may read one (agni
// issue 1728). mountset.Table is the server's.
type SourceAccess interface {
	// Guarding returns the private source a mount reads, such as "github://acme/board", or "" when
	// every caller the server lets in may read what it serves.
	Guarding(mount string) string
	// CanRead returns nil when one of the caller's own connections can read source. It returns
	// ErrPermissionDenied when none can, or the caller has none, and ErrUnavailable when the source
	// couldn't say.
	CanRead(ctx context.Context, source string) error
}

// AccessGuard decides whether the caller may read a design, a file or a snapshot that came from a
// private source (agni issue 1728). A design is guarded by every private source its artifacts were
// read from, and a file in the uploads mount by the design it belongs to. A nil guard lets every
// caller read everything, as a server without --auth does.
type AccessGuard struct {
	src     SourceAccess
	uploads string
	// design checks the caller may open the design named "designs/{d}". The catalog sets it.
	design func(ctx context.Context, name string) error
	// pack checks the caller may open the pack with id. PackService.WithGuard sets it.
	pack func(ctx context.Context, id string) error
}

// NewAccessGuard returns a guard over src. uploads names the mount uploaded designs' files are
// served from, whose first path segment is a design id.
func NewAccessGuard(src SourceAccess, uploads string) *AccessGuard {
	return &AccessGuard{src: src, uploads: uploads}
}

// packViews is the folder under the uploads mount that holds packs' files rather than a design's.
const packViews = "@packs"

// Sources returns the private sources arts were read from, sorted, with none repeated. An artifact
// names its own when it was read, and one written before agni issue 1728 is guarded by its mount.
func (g *AccessGuard) Sources(arts []*webapi.Artifact) []string {
	if g == nil {
		return nil
	}
	var out []string
	for _, a := range arts {
		s := a.GetSource().GetAccessSource()
		if s == "" {
			s = g.src.Guarding(a.GetSource().GetMount())
		}
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// stampSources records on each write the private source its mount reads, so the files stay guarded
// after an unlink or the mount's deletion. It runs whether or not reads are guarded, so a design
// linked on a server without --auth is guarded once the server runs with it (agni issue 1778).
func stampSources(src SourceAccess, writes []ArtifactWrite) {
	if src == nil {
		return
	}
	for i := range writes {
		w := writes[i].Source
		if w == nil || w.GetAccessSource() != "" {
			continue
		}
		if s := src.Guarding(w.GetMount()); s != "" {
			c := proto.Clone(w).(*webapi.ArtifactSource)
			c.AccessSource = s
			writes[i].Source = c
		}
	}
}

// unguardedKey marks a context whose reads skip the guard. Only this package sets it, for a check
// that must see every design whoever asks, such as which designs link to a mount.
type unguardedKey struct{}

func unguarded(ctx context.Context) context.Context {
	return context.WithValue(ctx, unguardedKey{}, true)
}

// Check returns nil when the caller may read every one of sources.
func (g *AccessGuard) Check(ctx context.Context, sources []string) error {
	if g == nil {
		return nil
	}
	for _, s := range sources {
		if err := g.src.CanRead(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// Mount returns nil when the caller may read what mount serves.
func (g *AccessGuard) Mount(ctx context.Context, mount string) error {
	if g == nil {
		return nil
	}
	if s := g.src.Guarding(mount); s != "" {
		return g.src.CanRead(ctx, s)
	}
	return nil
}

// URI returns nil when the caller may read uri. A file in the uploads mount belongs to the design
// its first segment names, and a design that no longer exists guards nothing.
func (g *AccessGuard) URI(ctx context.Context, uri mounts.URI) error {
	if g == nil {
		return nil
	}
	if uri.Mount != g.uploads || g.uploads == "" {
		return g.Mount(ctx, uri.Mount)
	}
	id, rest, _ := strings.Cut(uri.Path, "/")
	if id == packViews {
		// A pack's files sit at @packs/<pack>/<rev>/, and a pack read from a private source
		// guards them (agni issue 1801).
		pid, _, _ := strings.Cut(rest, "/")
		if pid == "" || g.pack == nil {
			return nil
		}
		err := g.pack(ctx, pid)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidArgument) {
			return nil
		}
		return err
	}
	if id == "" || g.design == nil {
		return nil
	}
	err := g.design(ctx, StandaloneDesignName(id))
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidArgument) {
		return nil
	}
	return err
}

// URIString is URI for a "mount://<mount>/<path>" string. One that doesn't parse names no mount,
// so it guards nothing, and the read that follows refuses it.
func (g *AccessGuard) URIString(ctx context.Context, uri string) error {
	if g == nil || uri == "" {
		return nil
	}
	u, err := mounts.ParseURI(uri)
	if err != nil {
		return nil
	}
	return g.URI(ctx, u)
}

// Snapshot returns nil when the caller may read what def reads: its design, and each uri it names
// for the netlist, the board, the geometry and the project.
func (g *AccessGuard) Snapshot(ctx context.Context, def *webapi.Snapshot) error {
	if g == nil || def == nil {
		return nil
	}
	if d := def.GetDesign(); d != "" && g.design != nil {
		if err := g.design(ctx, d); err != nil {
			return err
		}
	}
	for _, u := range []string{def.GetUri(), def.GetBoardUri(), def.GetGeometryUri(), def.GetProject()} {
		if err := g.URIString(ctx, u); err != nil {
			return err
		}
	}
	return nil
}
