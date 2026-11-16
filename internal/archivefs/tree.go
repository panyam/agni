package archivefs

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"time"
)

// tree is a read-only file system over an archive's members, with the directories their paths
// imply. A member's bytes are either held already, for an archive decoded whole, or read on open,
// for a zip.
type tree struct {
	files map[string]leaf
	dirs  map[string][]string // dir -> sorted child names
}

type leaf struct {
	size int64
	mod  time.Time
	open func() (io.ReadCloser, error)
}

func newMemFS(files map[string][]byte) *tree {
	leaves := make(map[string]leaf, len(files))
	for name, b := range files {
		b := b
		leaves[name] = leaf{size: int64(len(b)), open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }}
	}
	return newTree(leaves)
}

func newZipFS(files map[string]*zip.File) *tree {
	leaves := make(map[string]leaf, len(files))
	for name, f := range files {
		leaves[name] = leaf{size: int64(f.UncompressedSize64), mod: f.Modified, open: f.Open} // #nosec G115 -- openZip bounds every size to MaxMemberBytes
	}
	return newTree(leaves)
}

func newTree(files map[string]leaf) *tree {
	t := &tree{files: files, dirs: map[string][]string{".": nil}}
	seen := map[string]bool{}
	for name := range files {
		child := name
		for {
			dir := path.Dir(child)
			key := dir + "\x00" + path.Base(child)
			if !seen[key] {
				seen[key] = true
				t.dirs[dir] = append(t.dirs[dir], path.Base(child))
			}
			if dir == "." {
				break
			}
			if _, ok := t.dirs[dir]; !ok {
				t.dirs[dir] = nil
			}
			child = dir
		}
	}
	for d := range t.dirs {
		sort.Strings(t.dirs[d])
	}
	return t
}

func (t *tree) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if l, ok := t.files[name]; ok {
		rc, err := l.open()
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		return &treeFile{info: entry{name: path.Base(name), size: l.size, mod: l.mod}, rc: rc}, nil
	}
	if kids, ok := t.dirs[name]; ok {
		d := &treeDir{name: path.Base(name)}
		for _, k := range kids {
			full := path.Join(name, k)
			_, isDir := t.dirs[full]
			l := t.files[full]
			d.entries = append(d.entries, entry{name: k, dir: isDir, size: l.size, mod: l.mod})
		}
		return d, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

type treeFile struct {
	info entry
	rc   io.ReadCloser
}

func (f *treeFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *treeFile) Read(p []byte) (int, error) { return f.rc.Read(p) }
func (f *treeFile) Close() error               { return f.rc.Close() }

type treeDir struct {
	name    string
	entries []entry
	pos     int
}

func (d *treeDir) Stat() (fs.FileInfo, error) { return entry{name: d.name, dir: true}, nil }
func (d *treeDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}
func (d *treeDir) Close() error { return nil }
func (d *treeDir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.pos:]
	if n > 0 && len(rest) == 0 {
		return nil, io.EOF
	}
	if n > 0 && n < len(rest) {
		rest = rest[:n]
	}
	d.pos += len(rest)
	out := make([]fs.DirEntry, len(rest))
	for i := range rest {
		out[i] = rest[i]
	}
	return out, nil
}

// entry is both the FileInfo and the DirEntry of a member or a directory.
type entry struct {
	name string
	dir  bool
	size int64
	mod  time.Time
}

func (e entry) Name() string { return e.name }
func (e entry) Size() int64  { return e.size }
func (e entry) Mode() fs.FileMode {
	if e.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (e entry) ModTime() time.Time         { return e.mod }
func (e entry) IsDir() bool                { return e.dir }
func (e entry) Sys() any                   { return nil }
func (e entry) Type() fs.FileMode          { return e.Mode().Type() }
func (e entry) Info() (fs.FileInfo, error) { return e, nil }
