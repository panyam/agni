// Package mountset is the set of mounts an agni server or CLI serves, over goapplib storage's mounts
// registry (agni issue 1759). It reads the --mount and --mount-root flags into fixed folder mounts,
// keeps the runtime mounts CreateMount makes in --store, resolves a mount:// URI to a host path for
// the OS-backed adapters, and maps the library's errors onto agni's service errors. The library
// keeps every path inside its mount. The package is outside internal/ so an embedder can serve the
// same mounts (C13), and so the datasheet service, a module of its own, can import it (agni issue
// 744).
package mountset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/service"
)

// Folder is the fixed mount that serves dir, an absolute path, as name.
func Folder(name, dir string) mounts.Mount {
	return mounts.Mount{Name: name, Kind: mounts.KindFolder, Source: dir, Mode: mounts.ModeLive}
}

// Parse turns repeated `name=path` flag values into fixed folder mounts, each path resolved to an
// absolute directory. It rejects an empty name or path, a duplicate name, or a path that is not an
// existing directory. Order is preserved so ListMounts reflects the command line.
func Parse(specs []string) ([]mounts.Mount, error) {
	var out []mounts.Mount
	seen := map[string]bool{}
	for _, spec := range specs {
		name, path, ok := strings.Cut(spec, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("mount %q must be in the form name=path", spec)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate mount name %q", name)
		}
		if _, err := mounts.NewURI(name, ""); err != nil {
			return nil, fmt.Errorf("mount %q: %w", name, err)
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
		out = append(out, Folder(name, abs))
	}
	return out, nil
}

// Discover turns each immediate SUBDIRECTORY of root into a mount named after it, so a container
// can expose design folders by bind-mounting them under one path with no flags
// (`-v ~/boards:/workspace/boards`). Parse is the explicit half, and Merge combines the two.
//
// A missing root is NOT an error and returns no mounts, because the container always passes
// --mount-root and one with nothing bind-mounted should still start. A root that is a file is an
// error. Order is os.ReadDir's (sorted by filename), so ListMounts is stable across restarts.
// Non-directories and dotfiles (.DS_Store, .git) are skipped.
func Discover(root string) ([]mounts.Mount, error) {
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
	var found []mounts.Mount
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Stat through the symlink, since a bind mount can appear as a link. DirEntry.IsDir
		// reports on the link itself, and that would skip a perfectly good design folder.
		sub := filepath.Join(abs, name)
		si, err := os.Stat(sub)
		if err != nil || !si.IsDir() {
			continue
		}
		found = append(found, Folder(name, sub))
	}
	return found, nil
}

// Merge combines discovered mounts with explicitly configured ones. Explicit wins a name collision.
// The operator typed --mount for this run, and a discovered mount is whatever sat under the mount
// root. Order is discovered then explicit, so ListMounts is stable.
func Merge(discovered, explicit []mounts.Mount) []mounts.Mount {
	byName := map[string]bool{}
	for _, m := range explicit {
		byName[m.Name] = true
	}
	merged := make([]mounts.Mount, 0, len(discovered)+len(explicit))
	for _, m := range discovered {
		if byName[m.Name] {
			continue
		}
		merged = append(merged, m)
	}
	return append(merged, explicit...)
}

// Find returns the mount with the given name from a set of mounts.
func Find(ms []mounts.Mount, name string) (mounts.Mount, bool) {
	for _, m := range ms {
		if m.Name == name {
			return m, true
		}
	}
	return mounts.Mount{}, false
}

// Err maps an error from goapplib's storage module onto agni's service errors, so the transport
// classifies it as it classifies agni's own. An error that wraps none of the library's sentinels
// passes through unchanged. The message is kept.
//
// A Git LFS object the driver couldn't read is FAILED_PRECONDITION, whatever reason ErrLFS wraps
// with it, since the mount and the file exist and the message names the file (agni issue 1809).
// ErrTooLarge names the serve flags that set the cap (agni issue 1877).
func Err(err error) error {
	if err == nil {
		return nil
	}
	for _, m := range []struct{ lib, svc error }{
		{mounts.ErrLFS, service.ErrFailedPrecondition},
		{mounts.ErrNotFound, service.ErrNotFound},
		{mounts.ErrExists, service.ErrAlreadyExists},
		{mounts.ErrInvalidArgument, service.ErrInvalidArgument},
		{mounts.ErrInvalidPath, service.ErrInvalidPath},
		{mounts.ErrFixed, service.ErrFailedPrecondition},
		{mounts.ErrInUse, service.ErrFailedPrecondition},
		{mounts.ErrReadOnly, service.ErrFailedPrecondition},
		{mounts.ErrNotLocal, service.ErrFailedPrecondition},
		{mounts.ErrConflict, service.ErrFailedPrecondition},
		{mounts.ErrTooLarge, service.ErrResourceExhausted},
		{mounts.ErrUnavailable, service.ErrUnavailable},
		{mounts.ErrPermissionDenied, service.ErrPermissionDenied},
	} {
		if errors.Is(err, m.lib) && m.lib == mounts.ErrTooLarge {
			return fmt.Errorf("%w: %w (agni serve's --mount-max-bytes and --mount-max-files set the cap)", m.svc, err)
		}
		if errors.Is(err, m.lib) {
			return fmt.Errorf("%w: %w", m.svc, err)
		}
	}
	return err
}
