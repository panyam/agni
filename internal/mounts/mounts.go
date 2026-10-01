// Package mounts is the containment boundary of the web tier, the named root folders the server
// exposes plus the join that keeps every client-supplied path inside its mount. Mounts are checked
// against the filesystem at parse time. It lives outside cmd/agni so any entrypoint hosting the
// serve services reuses the same boundary.
package mounts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

// Mount is one configured root folder the web API serves. Name is the handle a client passes back
// to address the mount, and Root is the resolved absolute host path. Clients never send absolute
// paths. They send an artifact.URI naming a mount and a mount-relative path, which Resolve joins.
// The OS-backed service adapters in cmd/agni hold these, and the service package stays os-free
// (CONSTRAINTS C13).
type Mount struct {
	Name string
	Root string
}

// Parse turns repeated `name=path` flag values into validated mounts, each path resolved to an
// absolute directory. It rejects an empty name or path, a duplicate name, or a path that is not an
// existing directory. Order is preserved so ListMounts reflects the command line.
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

// Discover turns each immediate SUBDIRECTORY of root into a mount named after it, so a container
// can expose design folders by bind-mounting them under one path with no flags
// (`-v ~/boards:/workspace/boards`). Parse is the explicit half, and Merge combines the two. The
// subdirectories are the mounts rather than root, so each folder keeps its own handle in ListMounts.
//
// A missing root is NOT an error and returns no mounts, because the container always passes
// --mount-root and one with nothing bind-mounted should still start. A root that is a file is an
// error. Order is os.ReadDir's (sorted by filename), so ListMounts is stable across restarts.
// Non-directories and dotfiles (.DS_Store, .git) are skipped.
func Discover(root string) ([]Mount, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("mount root %q: %w", root, err)
	}
	fi, err := os.Stat(abs)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mount root %q: %w", root, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("mount root %q: %q is not a directory", root, abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("mount root %q: %w", root, err)
	}
	var found []Mount
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Stat through the symlink, since a bind mount can land as a link and DirEntry.IsDir
		// reports on the link itself, which would skip a perfectly good design folder.
		sub := filepath.Join(abs, name)
		si, err := os.Stat(sub)
		if err != nil || !si.IsDir() {
			continue
		}
		found = append(found, Mount{Name: name, Root: sub})
	}
	return found, nil
}

// Merge combines discovered mounts with explicitly configured ones. Explicit wins a name collision,
// since --mount is what the operator typed for this run and a discovered mount is whatever sat under
// the mount root. Order is discovered then explicit, so ListMounts stays stable.
func Merge(discovered, explicit []Mount) []Mount {
	byName := map[string]bool{}
	for _, m := range explicit {
		byName[m.Name] = true
	}
	merged := make([]Mount, 0, len(discovered)+len(explicit))
	for _, m := range discovered {
		if byName[m.Name] {
			continue
		}
		merged = append(merged, m)
	}
	return append(merged, explicit...)
}

// SafeResolve joins a mount-relative path onto root and confirms the result stays inside root,
// returning the cleaned absolute path. It rejects absolute inputs and any path that escapes the
// mount via "..". Resolve is its only non-test caller.
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

// Resolve maps an artifact URI to an absolute host path inside its mount, for the OS-backed service
// adapters. An unknown mount returns service.ErrNotFound, so the service classifies without
// importing os. Containment is decided by artifact.Parse, and a parsed URI cannot escape its mount,
// so the SafeResolve call below is only an assertion.
func Resolve(mounts []Mount, uri artifact.URI) (string, error) {
	m, ok := Find(mounts, uri.Mount)
	if !ok {
		return "", fmt.Errorf("no such mount %q: %w", uri.Mount, service.ErrNotFound)
	}
	abs, err := SafeResolve(m.Root, uri.Path)
	if err != nil {
		// Unreachable for a parsed URI. If it fires, artifact.Parse and this join disagree, and
		// the alternative to failing here is a silent traversal.
		return "", fmt.Errorf("%w: %s", service.ErrInvalidPath, err)
	}
	return abs, nil
}
