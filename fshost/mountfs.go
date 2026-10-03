// Package fshost serves designs from a mount table of fs.FS values. It is the backend the wasm engine
// composes the services over (agni issue 178), and it implements the service ports (Loader,
// ReviewLoader, ConventionLoader, Workspace, ConfigResolver) without touching the host filesystem, so
// an in-memory map, an embedded tree, a zip and os.DirFS all serve the same way.
//
// Every mount is a top-level directory of ONE composed fs.FS (Root). A design names a sibling in
// another mount, such as a project's symbol library, as an ordinary path, which a separate fs.FS per
// mount could not express.
package fshost

import (
	"errors"
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// Mount is one named tree. Name is the handle a client addresses it by in an artifact.URI.
type Mount struct {
	Name string
	FS   fs.FS
}

// Root composes ms into one fs.FS whose top-level directories are the mounts, in name order. A name
// in no mount does not exist. A later mount with a repeated name is ignored, so the first wins.
func Root(ms []Mount) fs.FS {
	r := rootFS{byName: map[string]fs.FS{}}
	for _, m := range ms {
		if _, dup := r.byName[m.Name]; dup || m.Name == "" || strings.Contains(m.Name, "/") {
			continue
		}
		r.byName[m.Name] = m.FS
		r.names = append(r.names, m.Name)
	}
	sort.Strings(r.names)
	return r
}

type rootFS struct {
	byName map[string]fs.FS
	names  []string
}

// split takes "mount/rest" apart, with rest "." for the mount's own root.
func (r rootFS) split(op, name string) (fs.FS, string, error) {
	if !fs.ValidPath(name) {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	m, rest, _ := strings.Cut(name, "/")
	fsys, ok := r.byName[m]
	if !ok {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}
	if rest == "" {
		rest = "."
	}
	return fsys, rest, nil
}

func (r rootFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &rootDir{entries: r.entries()}, nil
	}
	fsys, rest, err := r.split("open", name)
	if err != nil {
		return nil, err
	}
	f, err := fsys.Open(rest)
	if err != nil || rest != "." {
		return f, err
	}
	// A mount's own root is "." inside its FS and the mount's name here, and fs.WalkDir and fs.Sub
	// expect a file's Stat to agree with the directory entry that led to it.
	return mountRoot{File: f, name: name}, nil
}

// mountRoot is a mount's root directory opened through the composed FS.
type mountRoot struct {
	fs.File
	name string
}

func (m mountRoot) Stat() (fs.FileInfo, error) { return dirInfo(m.name), nil }

func (m mountRoot) ReadDir(n int) ([]fs.DirEntry, error) {
	d, ok := m.File.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: m.name, Err: errors.New("not a directory")}
	}
	return d.ReadDir(n)
}

func (r rootFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return r.entries(), nil
	}
	fsys, rest, err := r.split("readdir", name)
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(fsys, rest)
}

func (r rootFS) Stat(name string) (fs.FileInfo, error) {
	if name == "." {
		return dirInfo("."), nil
	}
	fsys, rest, err := r.split("stat", name)
	if err != nil {
		return nil, err
	}
	if rest == "." {
		// The mount's own root reports the mount's name, as a directory entry for it would.
		return dirInfo(name), nil
	}
	return fs.Stat(fsys, rest)
}

func (r rootFS) entries() []fs.DirEntry {
	out := make([]fs.DirEntry, 0, len(r.names))
	for _, n := range r.names {
		out = append(out, fs.FileInfoToDirEntry(dirInfo(n)))
	}
	return out
}

// rootDir is the composed root opened as a directory.
type rootDir struct {
	entries []fs.DirEntry
	off     int
}

func (d *rootDir) Stat() (fs.FileInfo, error) { return dirInfo("."), nil }
func (d *rootDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: errors.New("is a directory")}
}
func (d *rootDir) Close() error { return nil }

func (d *rootDir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.off:]
	if n <= 0 {
		d.off = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.off += n
	return rest[:n], nil
}

// dirInfo describes a synthetic directory, the root or a mount's own root.
type dirInfo string

func (d dirInfo) Name() string       { return string(d) }
func (d dirInfo) Size() int64        { return 0 }
func (d dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (d dirInfo) ModTime() time.Time { return time.Time{} }
func (d dirInfo) IsDir() bool        { return true }
func (d dirInfo) Sys() any           { return nil }
