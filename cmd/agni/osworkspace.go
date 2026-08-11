package main

import (
	"context"
	"fmt"
	"os"

	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
)

// osWorkspace is the OS-backed service.Workspace adapter: it resolves a (mount, relpath) to an
// absolute host path under the mount root (safeResolve containment) and lists it with os.ReadDir.
// All filesystem access stays at the cmd edge (CONSTRAINTS C1/C13); the service package is os-free.
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

// ListDir resolves the mount + relative path and reads one directory level. An unknown mount or a
// missing directory returns a plain error (the service maps it to NotFound); a path escaping the
// mount is wrapped with service.ErrInvalidPath (mapped to InvalidArgument).
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
