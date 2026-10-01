package main

import (
	"context"
	"fmt"
	"os"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
)

// osWorkspace is the OS-backed service.Workspace. It resolves a URI to a host path with
// mounts.Resolve, which keeps it inside the mount root, and lists it with os.ReadDir.
type osWorkspace struct {
	mounts []mounts.Mount
}

// Mounts returns the configured mounts as the port's runtime-neutral MountInfo.
func (w *osWorkspace) Mounts() []service.MountInfo {
	out := make([]service.MountInfo, 0, len(w.mounts))
	for _, m := range w.mounts {
		out = append(out, service.MountInfo{Name: m.Name, Root: m.Root})
	}
	return out
}

// ListDir resolves the URI and reads one directory level. The service maps an error wrapping
// service.ErrInvalidPath to InvalidArgument and any other, including an unknown mount or a missing
// directory, to NotFound.
func (w *osWorkspace) ListDir(_ context.Context, uri artifact.URI) ([]service.DirEntry, error) {
	abs, err := mounts.Resolve(w.mounts, uri)
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
