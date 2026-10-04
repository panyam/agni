package fshost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/review"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// MemFS is an in-memory tree of the given files, keyed by slash-separated path. It is how the wasm
// engine holds a design the browser handed it. Directories are implied by the paths.
func MemFS(files map[string][]byte) fs.FS {
	m := fstest.MapFS{}
	for p, b := range files {
		m[p] = &fstest.MapFile{Data: b, Mode: 0o444}
	}
	return m
}

// Host serves a mount table through the service ports. It is the fs.FS twin of the server's
// OS-backed adapters, and behaves as they do for the same tree, which the parity test in cmd/agni
// holds it to.
type Host struct {
	mounts []Mount
	root   fs.FS
	loader *formats.Loader
}

// New serves ms. Every read goes through one formats.Loader over the composed Root, and provenance
// records each file's path within its mount, as the server records it.
func New(ms ...Mount) *Host {
	h := &Host{mounts: ms, root: Root(ms)}
	h.loader = &formats.Loader{FS: h.root, SourceName: withinMount}
	return h
}

// Root is the composed tree, for a caller that needs the same name space the reads use.
func (h *Host) Root() fs.FS { return h.root }

// Mounts returns the mount table New was given.
func (h *Host) Mounts() []Mount { return h.mounts }

// withinMount drops a name's leading mount directory, so a locator names the file within its mount
// and never which mount served it (C28).
func withinMount(name string) string {
	if _, rest, ok := strings.Cut(name, "/"); ok {
		return rest
	}
	return name
}

// name maps a URI to its path in Root. An unknown mount is service.ErrNotFound.
func (h *Host) name(uri artifact.URI) (string, error) {
	for _, m := range h.mounts {
		if m.Name == uri.Mount {
			n := path.Join(uri.Mount, uri.Path)
			if !fs.ValidPath(n) || (n != uri.Mount && !strings.HasPrefix(n, uri.Mount+"/")) {
				return "", fmt.Errorf("%w: %q", service.ErrInvalidPath, uri.Path)
			}
			return n, nil
		}
	}
	return "", fmt.Errorf("no such mount %q: %w", uri.Mount, service.ErrNotFound)
}

// Design reads the netlist IR.
func (h *Host) Design(ctx context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	return service.LoaderIn(ctx, h.loader, opts...).ReadDesign(n)
}

// Geometry draws a netlist's companion schematic where it has one, else the design's own geometry or
// an auto-layout.
func (h *Host) Geometry(ctx context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...service.ReadOption) (*geom.SchematicGeometry, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	reader := service.LoaderIn(ctx, h.loader, opts...)
	if comp := reader.Companion(n); comp != "" {
		return reader.FaithfulGeometry(comp)
	}
	return reader.ResolveGeometry(n, layout, nil, formats.SymbolsFor(faithfulSymbols))
}

// Report classifies how an auto-layout draws each component.
func (h *Host) Report(ctx context.Context, uri artifact.URI, faithfulSymbols bool, opts ...service.ReadOption) (*graph.ConversionReport, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	return service.LoaderIn(ctx, h.loader, opts...).ConversionReport(n, formats.SymbolsFor(faithfulSymbols), nil)
}

// Expectations loads a `<design>.expect.yaml` sidecar, and (nil, nil) when there is none.
func (h *Host) Expectations(_ context.Context, uri artifact.URI) (*expect.Expectations, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	b, err := fs.ReadFile(h.root, n+".expect.yaml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return expect.Parse(b)
}

// Board reads the physical board sidecar for a format that carries one, and (nil, nil) otherwise.
func (h *Host) Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	return service.LoaderIn(ctx, h.loader).BoardGeometry(n)
}

// Manifest parses a review checklist. Absent is an error, since a manifest is a required input.
func (h *Host) Manifest(_ context.Context, uri artifact.URI) (review.Manifest, error) {
	n, err := h.name(uri)
	if err != nil {
		return review.Manifest{}, err
	}
	f, err := h.root.Open(n)
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

// Convention parses a naming-convention config. Absent is an error once named.
func (h *Host) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	n, err := h.name(uri)
	if err != nil {
		return nil, err
	}
	b, err := fs.ReadFile(h.root, n)
	if err != nil {
		return nil, err
	}
	return naming.Parse(b)
}

// DesignHash is "sha256:<hex>" over the design's entry file, or "" for an unreadable one inside the
// mount. The caller passes a resolved tier (see service.Loader).
func (h *Host) DesignHash(_ context.Context, uri artifact.URI) (string, error) {
	n, err := h.name(uri)
	if err != nil {
		return "", err
	}
	return ContentHash(h.root, n), nil
}

// ContentHash is "sha256:<hex>" over one file, or "" when it cannot be read. It is the revision
// identity a verdict link is checked against, so every host computes it here.
func ContentHash(fsys fs.FS, name string) string {
	f, err := fsys.Open(name)
	if err != nil {
		return ""
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return ""
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil))
}

// MountInfo lists the mounts for WorkspaceService. A mount held in memory has no host root, so Root
// carries its name.
func (h *Host) MountInfo() []service.MountInfo {
	out := make([]service.MountInfo, 0, len(h.mounts))
	for _, m := range h.mounts {
		out = append(out, service.MountInfo{Name: m.Name, Root: m.Name})
	}
	return out
}

// Workspace returns the service.Workspace over this host's mounts. It is also the host's
// service.FileReader.
func (h *Host) Workspace() interface {
	service.Workspace
	service.FileReader
} {
	return workspace{h}
}

type workspace struct{ h *Host }

func (w workspace) Mounts() []service.MountInfo { return w.h.MountInfo() }

// ListDir reads one directory level. An unknown mount or a missing directory is an error the service
// maps to NotFound.
func (w workspace) ListDir(_ context.Context, uri artifact.URI) ([]service.DirEntry, error) {
	n, err := w.h.name(uri)
	if err != nil {
		return nil, err
	}
	des, err := fs.ReadDir(w.h.root, n)
	if err != nil {
		return nil, fmt.Errorf("mount %q: %w", uri.Mount, err)
	}
	out := make([]service.DirEntry, 0, len(des))
	for _, de := range des {
		out = append(out, service.DirEntry{Name: de.Name(), IsDir: de.IsDir()})
	}
	return out, nil
}

// ReadFile reads one file inside its mount, for service.FileReader.
func (w workspace) ReadFile(_ context.Context, uri artifact.URI) ([]byte, error) {
	n, err := w.h.name(uri)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(w.h.root, n)
}
