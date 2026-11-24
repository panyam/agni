package mountset

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/archivefs"
	"github.com/panyam/agni/service"
)

// Workspace is the OS-backed service.Workspace over a mount table. It resolves a URI to a host path
// with Resolve, which keeps it inside the mount root, and lists it with archivefs.OSReadDir, which
// lists an archive as a folder. Both the engine's server and the datasheet service list folders
// through it (agni issue 744).
type Workspace struct {
	t *Table
}

// NewWorkspace lists folders inside the mounts t serves, including any made at runtime.
func NewWorkspace(t *Table) *Workspace { return &Workspace{t: t} }

// Mounts returns the served mounts as the port's runtime-neutral MountInfo.
func (w *Workspace) Mounts() []service.MountInfo {
	ms := w.t.List()
	out := make([]service.MountInfo, 0, len(ms))
	for _, m := range ms {
		out = append(out, w.t.Info(m))
	}
	return out
}

// Info is m as the service port describes it.
func (t *Table) Info(m mounts.Mount) service.MountInfo {
	kind := webapi.MountKind_MOUNT_KIND_FILE
	source := m.Source
	switch {
	case m.Name == t.upMount && t.Fixed(m.Name):
		kind, source = webapi.MountKind_MOUNT_KIND_UPLOADS, ""
	case m.Kind == mounts.KindGitHub:
		kind = webapi.MountKind_MOUNT_KIND_GITHUB
	case m.Kind == mounts.KindS3:
		kind = webapi.MountKind_MOUNT_KIND_S3
	case m.Kind == mounts.KindGCS:
		kind = webapi.MountKind_MOUNT_KIND_GCS
	case m.Kind == mounts.KindMSGraph:
		kind = webapi.MountKind_MOUNT_KIND_MSGRAPH
	case m.Kind == mounts.KindFolder:
		source = "file:///" + strings.TrimPrefix(filepath.ToSlash(m.Source), "/")
	}
	fixed := t.Fixed(m.Name)
	return service.MountInfo{Name: m.Name, Root: t.Root(m), Kind: kind, Source: source, Ref: m.Ref, Commit: m.Version, Fixed: fixed,
		Writable: kind == webapi.MountKind_MOUNT_KIND_FILE, Connection: m.Connection}
}

// ListDir resolves the URI and reads one directory level. The service maps an error wrapping
// service.ErrInvalidPath to InvalidArgument. It maps any other error to NotFound, including an
// unknown mount or a missing directory.
func (w *Workspace) ListDir(_ context.Context, uri mounts.URI) ([]service.DirEntry, error) {
	// A lazy mount lists from its tree, so browsing it downloads nothing. A path that runs into an
	// archive isn't in the tree, and resolves as on any other mount.
	if m, ok := w.t.lazyMount(uri); ok {
		if des, err := fs.ReadDir(w.t.FS(m), lazyPath(uri)); err == nil {
			out := make([]service.DirEntry, 0, len(des))
			for _, de := range des {
				out = append(out, service.DirEntry{Name: de.Name(), IsDir: de.IsDir()})
			}
			return out, nil
		}
	}
	abs, err := w.t.Resolve(uri)
	if err != nil {
		return nil, err
	}
	// An archive lists as the folder it was made from, and a path can run into one (agni issue 1591).
	dirents, err := archivefs.OSReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("mount %q: %w", uri.Mount, byURI(uri, err))
	}
	out := make([]service.DirEntry, 0, len(dirents))
	for _, de := range dirents {
		out = append(out, service.DirEntry{Name: de.Name(), IsDir: de.IsDir()})
	}
	return out, nil
}

// ReadFile reads one file inside its mount, for service.FileReader. A directory is an error.
func (w *Workspace) ReadFile(_ context.Context, uri mounts.URI) ([]byte, error) {
	// A lazy mount fetches the one file, and not the folder around it as Resolve would.
	if m, ok := w.t.lazyMount(uri); ok {
		if b, err := fs.ReadFile(w.t.FS(m), lazyPath(uri)); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return b, byURI(uri, err)
		}
	}
	abs, err := w.t.Resolve(uri)
	if err != nil {
		return nil, err
	}
	b, err := archivefs.OSReadFile(abs)
	return b, byURI(uri, err)
}

// byURI names the file a read failed on by its mount URI rather than the folder it sits in on the
// server, since a caller sees the error and has no business knowing where the mount lives (agni
// issue 1864). The error still matches what it wrapped, such as fs.ErrNotExist.
func byURI(uri mounts.URI, err error) error {
	var pe *fs.PathError
	if !errors.As(err, &pe) {
		return err
	}
	return &fs.PathError{Op: pe.Op, Path: uri.String(), Err: pe.Err}
}

// lazyPath is uri's path in a lazy mount's FS, where the root is ".".
func lazyPath(uri mounts.URI) string {
	if uri.Path == "" {
		return "."
	}
	return uri.Path
}
