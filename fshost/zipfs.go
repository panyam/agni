package fshost

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// ExpandZips presents every `.zip` file in fsys as a directory of the same name holding the
// archive's contents, so a dropped zip reads exactly as the folder it was made from (agni issue
// 854). It nests: a zip inside a zip is a directory too. Everything else is fsys unchanged.
//
// An archive is read whole on first use and kept, which suits a browser mount whose bytes are in
// memory already. A file that ends in `.zip` but does not parse stays a plain file.
//
// A file at OverlayDir/<path> in fsys reads as <path>, over whatever is there, archives included, and
// OverlayDir itself is hidden. It is how the page writes a design.yaml beside a design that came
// out of a zip: the store under the browser mount refuses a path whose parent is a file, which is
// what an archive is until it is expanded.
func ExpandZips(fsys fs.FS) fs.FS {
	return &zipFS{base: fsys, open: map[string]*zip.Reader{}}
}

// OverlayDir holds the files that read over an expanded tree. web/src/wasm/drop.ts writes into it.
const OverlayDir = ".agni-overlay"

type zipFS struct {
	base fs.FS
	open map[string]*zip.Reader // archive path in base terms -> its reader
}

// resolve walks name one element at a time and returns the FS and the name inside it that name
// lands in, descending into each archive the walk passes through.
func (z *zipFS) resolve(op, name string) (fs.FS, string, error) {
	if !fs.ValidPath(name) {
		return nil, "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	cur, inner, prefix := fs.FS(z.base), "", ""
	if name == "." {
		return cur, ".", nil
	}
	for _, el := range strings.Split(name, "/") {
		next := el
		if inner != "" {
			next = inner + "/" + el
		}
		if strings.EqualFold(path.Ext(el), ".zip") {
			if zr := z.archive(cur, next, prefix); zr != nil {
				cur, inner = zr, ""
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

// archive returns the zip at name in cur, read once and cached under its full path, or nil when name
// is not a readable archive.
func (z *zipFS) archive(cur fs.FS, name, prefix string) *zip.Reader {
	key := path.Join(prefix, name)
	if zr, ok := z.open[key]; ok {
		return zr
	}
	b, err := fs.ReadFile(cur, name)
	if err != nil {
		return nil
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil
	}
	z.open[key] = zr
	return zr
}

// overlay returns the overlay's path for name, or "" when name is the overlay itself or the root.
func overlay(name string) string {
	if name == "." || name == OverlayDir || strings.HasPrefix(name, OverlayDir+"/") {
		return ""
	}
	return OverlayDir + "/" + name
}

func (z *zipFS) Open(name string) (fs.File, error) {
	if o := overlay(name); o != "" && fs.ValidPath(name) {
		if st, err := fs.Stat(z.base, o); err == nil && !st.IsDir() {
			return z.base.Open(o)
		}
	}
	fsys, inner, err := z.resolve("open", name)
	if err != nil {
		return nil, err
	}
	f, err := fsys.Open(inner)
	if err != nil {
		return nil, err
	}
	if inner == "." && name != "." {
		// An archive's root, which reports itself as "." inside the archive and as the zip's name
		// here, as its directory entry does.
		return &zipDirFile{ReadDirFile: mountRoot{File: f, name: path.Base(name)}, z: z, dir: name}, nil
	}
	if d, ok := f.(fs.ReadDirFile); ok {
		return &zipDirFile{ReadDirFile: d, z: z, dir: name}, nil
	}
	return f, nil
}

func (z *zipFS) ReadDir(name string) ([]fs.DirEntry, error) {
	fsys, inner, err := z.resolve("readdir", name)
	if err != nil {
		return nil, err
	}
	des, err := fs.ReadDir(fsys, inner)
	if err != nil {
		return nil, err
	}
	return z.merge(name, z.asDirs(name, des)), nil
}

// merge adds the overlay's entries for dir to a listing and hides the overlay itself, in name order.
func (z *zipFS) merge(dir string, des []fs.DirEntry) []fs.DirEntry {
	out := des[:0:0]
	have := map[string]bool{}
	for _, de := range des {
		if dir == "." && de.Name() == OverlayDir {
			continue
		}
		have[de.Name()] = true
		out = append(out, de)
	}
	o := OverlayDir
	if dir != "." {
		o = overlay(dir)
	}
	if extra, err := fs.ReadDir(z.base, o); err == nil {
		for _, de := range extra {
			if !have[de.Name()] {
				out = append(out, de)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	}
	return out
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
	if o := overlay(name); o != "" && fs.ValidPath(name) {
		if st, err := fs.Stat(z.base, o); err == nil && !st.IsDir() {
			return st, nil
		}
	}
	fsys, inner, err := z.resolve("stat", name)
	if err != nil {
		return nil, err
	}
	if inner == "." && name != "." {
		return dirInfo(path.Base(name)), nil
	}
	return fs.Stat(fsys, inner)
}

// asDirs reports each readable archive in a listing as a directory.
func (z *zipFS) asDirs(dir string, des []fs.DirEntry) []fs.DirEntry {
	out := make([]fs.DirEntry, len(des))
	for i, de := range des {
		out[i] = de
		if de.IsDir() || !strings.EqualFold(path.Ext(de.Name()), ".zip") {
			continue
		}
		full := de.Name()
		if dir != "." {
			full = dir + "/" + de.Name()
		}
		if fsys, inner, err := z.resolve("readdir", full); err == nil && inner == "." && fsys != z.base {
			out[i] = fs.FileInfoToDirEntry(dirInfo(de.Name()))
		}
	}
	return out
}

// zipDirFile is a directory opened through a zipFS, so reading it through the file (as fs.WalkDir
// does) reports archives as directories too.
type zipDirFile struct {
	fs.ReadDirFile
	z    *zipFS
	dir  string
	list []fs.DirEntry
	read bool
}

// ReadDir lists the whole directory once, merged with the overlay, and pages through it, so reading
// through the file agrees with ReadDir on the FS.
func (d *zipDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		des, err := d.ReadDirFile.ReadDir(-1)
		if err != nil {
			return nil, err
		}
		d.list, d.read = d.z.merge(d.dir, d.z.asDirs(d.dir, des)), true
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
