package odb

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// maxArchiveBytes bounds what an archive may expand to. A job is text and a large board's runs to
// a few hundred megabytes, so a gigabyte leaves room while refusing a decompression bomb (C1's
// input-is-hostile rule).
const maxArchiveBytes = 1 << 30

// FromArchive presents a .tgz, .tar or .zip holding an ODB++ job as the job's own file system,
// rooted at the directory that holds matrix/matrix. The format is sniffed from the bytes rather
// than the name, since producers name these files loosely.
func FromArchive(b []byte) (fs.FS, error) {
	var files map[string][]byte
	var err error
	switch {
	case bytes.HasPrefix(b, []byte("PK\x03\x04")):
		files, err = unzip(b)
	case bytes.HasPrefix(b, []byte{0x1f, 0x8b}):
		var raw []byte
		if raw, err = gunzip(b); err == nil {
			files, err = untar(raw)
		}
	case len(b) > 262 && string(b[257:262]) == "ustar":
		files, err = untar(b)
	default:
		return nil, errors.New("odb: not a .tgz, .tar or .zip archive")
	}
	if err != nil {
		return nil, err
	}
	mfs := newMemFS(files)
	root, err := FindRoot(mfs)
	if err != nil {
		return nil, err
	}
	if root == "." {
		return mfs, nil
	}
	return fs.Sub(mfs, root)
}

// IsArchiveJob reports whether b is an archive holding an ODB++ job, reading no more than it
// must. It is what a registry adapter sniffs before claiming a .zip or .tgz for this reader.
func IsArchiveJob(b []byte) bool {
	_, err := FromArchive(b)
	return err == nil
}

func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return limitedRead(zr)
}

func limitedRead(r io.Reader) ([]byte, error) {
	out, err := readAll(io.LimitReader(r, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if len(out) > maxArchiveBytes {
		return nil, errors.New("odb: archive expands past 1 GiB")
	}
	return out, nil
}

func untar(b []byte) (map[string][]byte, error) {
	files := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(b))
	total := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, fmt.Errorf("odb: tar: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name, ok := cleanMember(h.Name)
		if !ok {
			continue
		}
		data, err := limitedRead(tr)
		if err != nil {
			return nil, err
		}
		if total += len(data); total > maxArchiveBytes {
			return nil, errors.New("odb: archive expands past 1 GiB")
		}
		files[name] = data
	}
}

func unzip(b []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("odb: zip: %w", err)
	}
	files := map[string][]byte{}
	total := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name, ok := cleanMember(f.Name)
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("odb: zip: %s: %w", f.Name, err)
		}
		data, err := limitedRead(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if total += len(data); total > maxArchiveBytes {
			return nil, errors.New("odb: archive expands past 1 GiB")
		}
		files[name] = data
	}
	return files, nil
}

// cleanMember turns an archive member's name into an fs.ValidPath, refusing one that climbs out of
// the archive and skipping macOS's AppleDouble shadows, which sort ahead of the files they shadow.
func cleanMember(name string) (string, bool) {
	name = strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(name, "\\", "/")), "/")
	if name == "" || !fs.ValidPath(name) {
		return "", false
	}
	for _, el := range strings.Split(name, "/") {
		if strings.HasPrefix(el, "._") || el == "__MACOSX" {
			return "", false
		}
	}
	return name, true
}

// memFS is a read-only file system over an archive's members, with the directories their paths
// imply.
type memFS struct {
	files map[string][]byte
	dirs  map[string][]string // dir -> sorted child names
}

func newMemFS(files map[string][]byte) *memFS {
	m := &memFS{files: files, dirs: map[string][]string{".": nil}}
	seen := map[string]bool{}
	for name := range files {
		child := name
		for {
			dir := path.Dir(child)
			key := dir + "\x00" + path.Base(child)
			if !seen[key] {
				seen[key] = true
				m.dirs[dir] = append(m.dirs[dir], path.Base(child))
			}
			if dir == "." {
				break
			}
			if _, ok := m.dirs[dir]; !ok {
				m.dirs[dir] = nil
			}
			child = dir
		}
	}
	for d := range m.dirs {
		sort.Strings(m.dirs[d])
	}
	return m
}

func (m *memFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if b, ok := m.files[name]; ok {
		return &memFile{name: path.Base(name), r: bytes.NewReader(b), size: int64(len(b))}, nil
	}
	if kids, ok := m.dirs[name]; ok {
		d := &memDir{name: path.Base(name)}
		for _, k := range kids {
			full := path.Join(name, k)
			_, isDir := m.dirs[full]
			d.entries = append(d.entries, memEntry{name: k, dir: isDir, size: int64(len(m.files[full]))})
		}
		return d, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

type memFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *memFile) Stat() (fs.FileInfo, error) { return memEntry{name: f.name, size: f.size}, nil }
func (f *memFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *memFile) Close() error               { return nil }

type memDir struct {
	name    string
	entries []memEntry
	pos     int
}

func (d *memDir) Stat() (fs.FileInfo, error) { return memEntry{name: d.name, dir: true}, nil }
func (d *memDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: fs.ErrInvalid}
}
func (d *memDir) Close() error { return nil }
func (d *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
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

// memEntry is both the FileInfo and the DirEntry of a member.
type memEntry struct {
	name string
	dir  bool
	size int64
}

func (e memEntry) Name() string { return e.name }
func (e memEntry) Size() int64  { return e.size }
func (e memEntry) Mode() fs.FileMode {
	if e.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (e memEntry) ModTime() time.Time         { return time.Time{} }
func (e memEntry) IsDir() bool                { return e.dir }
func (e memEntry) Sys() any                   { return nil }
func (e memEntry) Type() fs.FileMode          { return e.Mode().Type() }
func (e memEntry) Info() (fs.FileInfo, error) { return e, nil }
