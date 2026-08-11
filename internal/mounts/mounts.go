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

	"github.com/panyam/agni/internal/artifact"
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

// Discover turns each immediate SUBDIRECTORY of root into a mount named after it, so a
// container can expose design folders by bind-mounting them under one path and passing no
// flags at all (`-v ~/boards:/workspace/boards`). It is the convention-over-configuration
// half of the mount model; Parse remains the explicit half, and the two compose.
//
// The subdirectories are the mounts, not root itself. Mounting root directly would make one
// bind mount per container the only option and would put every folder behind a single name,
// losing the per-folder handle that ListMounts roots the file tree on.
//
// A missing root is NOT an error: it returns no mounts. The container always passes
// --mount-root, and an operator who bind-mounts nothing should get an empty file tree and a
// server that still starts, not a startup failure. A root that exists but is a file is an
// error, because that is a misconfiguration rather than an absence.
//
// Mount order is os.ReadDir's, which is sorted by filename, so ListMounts is stable across
// restarts without this function sorting again. Non-directories and dotfiles are skipped, the
// latter so a stray .DS_Store or .git in a bind-mounted parent does not surface as a design
// folder.
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
		// Stat through the symlink: a bind mount can land as a link, and DirEntry.IsDir
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

// Merge combines discovered mounts with explicitly configured ones, with explicit winning on a
// name collision. That direction matters: --mount is what an operator typed for this run, while a
// discovered mount is whatever happened to be sitting under the mount root, so the typed one is
// the more specific intent. Order is discovered-then-explicit-extras, both already sorted or in
// command-line order, so ListMounts stays stable.
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

// Resolve maps an artifact URI to an absolute host path inside its mount, for the OS-backed service
// adapters. An unknown mount returns service.ErrNotFound (mapped to NotFound), so the service
// classifies without importing os.
//
// It no longer re-checks containment, and that is the point. A parsed artifact.URI cannot carry a
// path that escapes its mount, because artifact.Parse is where that is decided; re-checking here
// would be a second implementation of one rule, and the failure mode of two implementations is that
// they disagree. SafeResolve remains for the one caller that still joins a path it did not receive
// as a URI (mount discovery), and as the belt-and-braces assertion below.
func Resolve(mounts []Mount, uri artifact.URI) (string, error) {
	m, ok := Find(mounts, uri.Mount)
	if !ok {
		return "", fmt.Errorf("no such mount %q: %w", uri.Mount, service.ErrNotFound)
	}
	abs, err := SafeResolve(m.Root, uri.Path)
	if err != nil {
		// Unreachable for a parsed URI. Kept as an assertion rather than deleted: if it ever fires,
		// artifact.Parse and this join have come to disagree, and a silent traversal is the worst
		// possible way to find that out.
		return "", fmt.Errorf("%w: %s", service.ErrInvalidPath, err)
	}
	return abs, nil
}
