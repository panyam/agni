package archivefs

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Expand presents every archive in fsys as a directory of the same name holding the archive's
// members. It nests, so an archive inside an archive is a directory too. A file that has an archive's
// name and does not open as one stays a file. An archive is read whole on first use and kept.
func Expand(fsys fs.FS) fs.FS {
	return &expanded{base: fsys, open: map[string]fs.FS{}}
}

type expanded struct {
	base fs.FS
	mu   sync.Mutex
	open map[string]fs.FS // archive path in base terms -> its members
}

// Resolve walks name one element at a time and descends into each archive on the way. It returns
// the file system that holds name, and name's path inside it.
func (e *expanded) resolve(op, name string) (fs.FS, string, error) {
	if !fs.ValidPath(name) {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	cur, inner, prefix := e.base, "", ""
	if name == "." {
		return cur, ".", nil
	}
	for _, el := range strings.Split(name, "/") {
		next := el
		if inner != "" {
			next = inner + "/" + el
		}
		if Candidate(el) {
			if a := e.archive(cur, next, prefix); a != nil {
				cur, inner = a, ""
				prefix = path.Join(prefix, next)
				continue
			}
		}
		inner = next
	}
	if inner == "" {
		inner = "."
	}
	return cur, inner, nil
}

// archive returns the archive at name in cur, read once and kept under its full path, or nil when
// name is not a readable archive.
func (e *expanded) archive(cur fs.FS, name, prefix string) fs.FS {
	key := path.Join(prefix, name)
	e.mu.Lock()
	a, ok := e.open[key]
	e.mu.Unlock()
	if ok {
		return a
	}
	if st, err := fs.Stat(cur, name); err != nil || st.IsDir() {
		return nil
	}
	b, err := fs.ReadFile(cur, name)
	if err != nil {
		return nil
	}
	if a, err = Open(b); err != nil {
		a = nil
	}
	e.mu.Lock()
	e.open[key] = a
	e.mu.Unlock()
	return a
}

func (e *expanded) Open(name string) (fs.File, error) {
	fsys, inner, err := e.resolve("open", name)
	if err != nil {
		return nil, err
	}
	f, err := fsys.Open(inner)
	if err != nil {
		return nil, err
	}
	if inner == "." && name != "." {
		// An archive's root reports itself as "." inside the archive. Here it reports the
		// archive's name, as its directory entry does.
		d, _ := f.(fs.ReadDirFile)
		return &dirFile{ReadDirFile: d, e: e, dir: name, name: path.Base(name)}, nil
	}
	if d, ok := f.(fs.ReadDirFile); ok {
		return &dirFile{ReadDirFile: d, e: e, dir: name}, nil
	}
	return f, nil
}

func (e *expanded) ReadDir(name string) ([]fs.DirEntry, error) {
	fsys, inner, err := e.resolve("readdir", name)
	if err != nil {
		return nil, err
	}
	des, err := fs.ReadDir(fsys, inner)
	if err != nil {
		return nil, err
	}
	return e.asDirs(name, des), nil
}

func (e *expanded) Stat(name string) (fs.FileInfo, error) {
	fsys, inner, err := e.resolve("stat", name)
	if err != nil {
		return nil, err
	}
	if inner == "." && name != "." {
		return entry{name: path.Base(name), dir: true}, nil
	}
	return fs.Stat(fsys, inner)
}

// asDirs reports each readable archive in a listing as a directory.
func (e *expanded) asDirs(dir string, des []fs.DirEntry) []fs.DirEntry {
	out := make([]fs.DirEntry, len(des))
	for i, de := range des {
		out[i] = de
		if de.IsDir() || !Candidate(de.Name()) {
			continue
		}
		full := de.Name()
		if dir != "." {
			full = dir + "/" + de.Name()
		}
		if fsys, inner, err := e.resolve("readdir", full); err == nil && inner == "." && fsys != e.base {
			out[i] = entry{name: de.Name(), dir: true}
		}
	}
	return out
}

// dirFile is a directory opened through Expand. Reading it through the file, as fs.WalkDir does,
// also reports archives as directories. name, when set, is the archive's own name for its root.
type dirFile struct {
	fs.ReadDirFile
	e    *expanded
	dir  string
	name string
	list []fs.DirEntry
	read bool
}

func (d *dirFile) Stat() (fs.FileInfo, error) {
	if d.name != "" {
		return entry{name: d.name, dir: true}, nil
	}
	return d.ReadDirFile.Stat()
}

// ReadDir lists the whole directory once and pages through it, so reading through the file agrees
// with ReadDir on the file system.
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		des, err := d.ReadDirFile.ReadDir(-1)
		if err != nil {
			return nil, err
		}
		d.list, d.read = d.e.asDirs(d.dir, des), true
	}
	if n <= 0 {
		out := d.list
		d.list = nil
		return out, nil
	}
	if len(d.list) == 0 {
		return nil, io.EOF
	}
	if n > len(d.list) {
		n = len(d.list)
	}
	out := d.list[:n]
	d.list = d.list[n:]
	return out, nil
}

// The host filesystem. The CLI and `agni serve` read designs by host path. A path that runs through
// an archive, such as /boards/job.7z/odb/matrix/matrix, opens the archive's member. The archive is
// kept while its size and modification time hold, a few at a time.

// osKept is how many archives on the host filesystem stay open at once.
const osKept = 8

type osArchive struct {
	fsys fs.FS
	size int64
	mod  time.Time
	used time.Time
}

var (
	osMu   sync.Mutex
	osOpen = map[string]*osArchive{}
)

// Container returns the archive file a host path runs through, or "" when the path names a file or
// directory of its own. A record of what a read touched stamps the archive, since its members have no
// stamp of their own on the host filesystem.
func Container(name string) string {
	a, _, ok := split(name)
	if !ok {
		return ""
	}
	return a
}

// split finds the archive a host path runs through. It answers false when the path exists as it is,
// or no element before its last names an archive file.
func split(name string) (archive, inner string, ok bool) {
	if _, err := os.Lstat(name); err == nil {
		return "", "", false
	}
	clean := filepath.Clean(name)
	vol := filepath.VolumeName(clean)
	parts := strings.Split(strings.TrimPrefix(clean[len(vol):], string(filepath.Separator)), string(filepath.Separator))
	root := vol
	if strings.HasPrefix(clean[len(vol):], string(filepath.Separator)) {
		root += string(filepath.Separator)
	}
	for i := range parts {
		prefix := root + filepath.Join(parts[:i+1]...)
		if !Candidate(parts[i]) {
			continue
		}
		if st, err := os.Stat(prefix); err == nil && st.Mode().IsRegular() {
			return prefix, path.Join(parts[i+1:]...), true
		}
	}
	return "", "", false
}

// osFS returns the members of the archive file at archive, opening it once while it is unchanged.
func osFS(archive string) (fs.FS, error) {
	st, err := os.Stat(archive)
	if err != nil {
		return nil, err
	}
	osMu.Lock()
	a := osOpen[archive]
	if a != nil && a.size == st.Size() && a.mod.Equal(st.ModTime()) {
		a.used = time.Now()
		osMu.Unlock()
		return a.fsys, nil
	}
	osMu.Unlock()
	b, err := os.ReadFile(archive) // #nosec G304 -- a host path the caller resolved through its mounts
	if err != nil {
		return nil, err
	}
	members, err := Open(b)
	if err != nil {
		return nil, err
	}
	fsys := Expand(members)
	osMu.Lock()
	defer osMu.Unlock()
	osOpen[archive] = &osArchive{fsys: fsys, size: st.Size(), mod: st.ModTime(), used: time.Now()}
	for len(osOpen) > osKept {
		var oldest string
		for k, v := range osOpen {
			if oldest == "" || v.used.Before(osOpen[oldest].used) {
				oldest = k
			}
		}
		delete(osOpen, oldest)
	}
	return fsys, nil
}

// through opens a host path that runs through an archive, or answers the error os gave for it.
func through[T any](name string, osErr error, inside func(fs.FS, string) (T, error)) (T, error) {
	var zero T
	archive, inner, ok := split(name)
	if !ok {
		return zero, osErr
	}
	fsys, err := osFS(archive)
	if err != nil {
		return zero, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if inner == "" {
		inner = "."
	}
	return inside(fsys, inner)
}

// OSOpen opens a host path as os.Open does, and through an archive where the path runs into one. An
// archive file itself opens as a file, as os.Open opens it.
func OSOpen(name string) (fs.File, error) {
	f, err := os.Open(name) // #nosec G304 -- a host path the caller resolved through its mounts
	if err == nil {
		return f, nil
	}
	return through(name, err, func(fsys fs.FS, inner string) (fs.File, error) { return fsys.Open(inner) })
}

// OSReadFile reads a host path as os.ReadFile does, and through an archive where the path runs into one.
func OSReadFile(name string) ([]byte, error) {
	b, err := os.ReadFile(name) // #nosec G304 -- a host path the caller resolved through its mounts
	if err == nil {
		return b, nil
	}
	return through(name, err, fs.ReadFile)
}

// OSStat stats a host path as os.Stat does, and through an archive where the path runs into one. An
// archive file stats as the directory it reads as.
func OSStat(name string) (fs.FileInfo, error) {
	st, err := os.Stat(name)
	if err == nil {
		if st.Mode().IsRegular() && Candidate(st.Name()) && IsArchiveFile(name) {
			return entry{name: st.Name(), dir: true, mod: st.ModTime()}, nil
		}
		return st, nil
	}
	return through(name, err, fs.Stat)
}

// OSReadDir lists a host directory as os.ReadDir does, with each archive listed as a directory, and
// lists inside an archive where the path runs into or names one.
func OSReadDir(name string) ([]fs.DirEntry, error) {
	if st, err := os.Stat(name); err == nil && st.Mode().IsRegular() {
		fsys, err := osFS(name)
		if err != nil {
			return nil, err
		}
		return fs.ReadDir(fsys, ".")
	}
	des, err := os.ReadDir(name)
	if err != nil {
		return through(name, err, fs.ReadDir)
	}
	for i, de := range des {
		if !de.IsDir() && Candidate(de.Name()) && IsArchiveFile(filepath.Join(name, de.Name())) {
			des[i] = entry{name: de.Name(), dir: true}
		}
	}
	return des, nil
}

// IsArchiveFile reports whether the host file at name starts as an archive does. It reads the file's
// first bytes alone, so a listing of a folder of large archives opens none of them. A file that
// starts like one and does not open fails when it is read.
func IsArchiveFile(name string) bool {
	f, err := os.Open(name) // #nosec G304 -- a host path the caller resolved through its mounts
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	return Sniff(head[:n])
}

// OSMembers is the archive file at a host path as the folder it was made from. It is kept while the
// file is unchanged.
func OSMembers(name string) (fs.FS, error) { return osFS(name) }
