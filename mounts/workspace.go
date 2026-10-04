package mounts

import (
	"context"
	"fmt"
	"os"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

// Workspace is the OS-backed service.Workspace over a mount table. It resolves a URI to a host path
// with Resolve, which keeps it inside the mount root, and lists it with os.ReadDir. Both the engine's
// server and the datasheet service list folders through it (agni issue 744).
type Workspace struct {
	ms []Mount
}

// NewWorkspace lists folders inside ms, the mount table a server was started with.
func NewWorkspace(ms []Mount) *Workspace { return &Workspace{ms: ms} }

// Mounts returns the configured mounts as the port's runtime-neutral MountInfo.
func (w *Workspace) Mounts() []service.MountInfo {
	out := make([]service.MountInfo, 0, len(w.ms))
	for _, m := range w.ms {
		out = append(out, service.MountInfo{Name: m.Name, Root: m.Root})
	}
	return out
}

// ListDir resolves the URI and reads one directory level. The service maps an error wrapping
// service.ErrInvalidPath to InvalidArgument and any other, including an unknown mount or a missing
// directory, to NotFound.
func (w *Workspace) ListDir(_ context.Context, uri artifact.URI) ([]service.DirEntry, error) {
	abs, err := Resolve(w.ms, uri)
	if err != nil {
		return nil, err
	}
	dirents, err := os.ReadDir(abs)
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
	abs, err := Resolve(w.ms, uri)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}
