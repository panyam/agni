package mountset

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
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
		out = append(out, Info(m))
	}
	return out
}

// Info is m as the service port describes it.
func Info(m Mount) service.MountInfo {
	kind := webapi.MountKind_MOUNT_KIND_FILE
	switch m.Kind {
	case KindUploads:
		kind = webapi.MountKind_MOUNT_KIND_UPLOADS
	case KindGitHub:
		kind = webapi.MountKind_MOUNT_KIND_GITHUB
	}
	source := m.Source
	if source == "" && m.Kind == KindFile {
		source = "file:///" + strings.TrimPrefix(filepath.ToSlash(m.Root), "/")
	}
	return service.MountInfo{Name: m.Name, Root: m.Root, Kind: kind, Source: source, Ref: m.Ref, Commit: m.Commit, Fixed: !m.Runtime, Writable: m.Kind == KindFile}
}

// ListDir resolves the URI and reads one directory level. The service maps an error wrapping
// service.ErrInvalidPath to InvalidArgument. It maps any other error to NotFound, including an
// unknown mount or a missing directory.
func (w *Workspace) ListDir(_ context.Context, uri artifact.URI) ([]service.DirEntry, error) {
	abs, err := w.t.Resolve(uri)
	if err != nil {
		return nil, err
	}
	// An archive lists as the folder it was made from, and a path can run into one (agni issue 1591).
	dirents, err := archivefs.OSReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("mount %q: %w", uri.Mount, err)
	}
	out := make([]service.DirEntry, 0, len(dirents))
	for _, de := range dirents {
		out = append(out, service.DirEntry{Name: de.Name(), IsDir: de.IsDir()})
	}
	return out, nil
}

// ReadFile reads one file inside its mount, for service.FileReader. A directory is an error.
func (w *Workspace) ReadFile(_ context.Context, uri artifact.URI) ([]byte, error) {
	abs, err := w.t.Resolve(uri)
	if err != nil {
		return nil, err
	}
	return archivefs.OSReadFile(abs)
}
