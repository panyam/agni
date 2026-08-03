// Package mounts is the containment boundary of the web tier: named root folders the
// server exposes, plus the join that keeps every client-supplied path inside its mount.
// It is pure path logic (validated at parse time against the filesystem), extracted from
// cmd/agni so any entrypoint hosting the serve services reuses the same boundary.
package mounts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/internal/service"
)

// Mount is one configured root folder the web API serves. Name is the handle a client
// passes back to address the mount; Root is the resolved absolute host path. Clients never
// send absolute paths: they name a mount and a mount-relative path, and the server joins
// them through SafeResolve, so the mount is the containment boundary. The OS-backed service
// adapters (osWorkspace, and the design loader in WS9-011) hold these; the service package
// itself is os-free (CONSTRAINTS C13).
type Mount struct {
	Name string
	Root string
}

// Parse turns repeated `name=path` flag values into validated mounts. It resolves
// each path to an absolute directory and rejects a value with an empty name or path, a
// duplicate name, or a path that is not an existing directory. Order is preserved so
// ListMounts reflects the command line.
func Parse(specs []string) ([]Mount, error) {
	var mounts []Mount
	seen := map[string]bool{}
	for _, spec := range specs {
		name, path, ok := strings.Cut(spec, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("mount %q must be in the form name=path", spec)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate mount name %q", name)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("mount %q: %w", name, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("mount %q: %w", name, err)
		}
		if !fi.IsDir() {
			return nil, fmt.Errorf("mount %q: %q is not a directory", name, abs)
		}
		seen[name] = true
		mounts = append(mounts, Mount{Name: name, Root: abs})
	}
	return mounts, nil
}

// SafeResolve joins a mount-relative path onto root and confirms the result stays inside
// root, returning the cleaned absolute path. It rejects absolute inputs and any path that
// escapes the mount via "..". This is the shared containment check every path-taking OS
// adapter funnels through (osWorkspace.ListDir, and the design loader in WS9-011).
func SafeResolve(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q must be relative to the mount", rel)
	}
	joined := filepath.Join(root, rel)
	within, err := filepath.Rel(root, joined)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the mount", rel)
	}
	return joined, nil
}

// Find returns the mount with the given name from a set of mounts.
func Find(mounts []Mount, name string) (Mount, bool) {
	for _, m := range mounts {
		if m.Name == name {
			return m, true
		}
	}
	return Mount{}, false
}

// Resolve maps (mount, mount-relative path) to an absolute host path inside the mount for
// the OS-backed service adapters. An unknown mount returns service.ErrNotFound (mapped to
// NotFound) and a path escaping the mount returns service.ErrInvalidPath (mapped to
// InvalidArgument), so the service classifies without importing os.
func Resolve(mounts []Mount, mountName, rel string) (string, error) {
	m, ok := Find(mounts, mountName)
	if !ok {
		return "", fmt.Errorf("no such mount %q: %w", mountName, service.ErrNotFound)
	}
	abs, err := SafeResolve(m.Root, rel)
	if err != nil {
		return "", fmt.Errorf("%w: %s", service.ErrInvalidPath, err)
	}
	return abs, nil
}
