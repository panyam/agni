package fshost

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"path"
	"strings"
)

// ExpandZips presents every `.zip` file in fsys as a directory of the same name holding the
// archive's contents, so a dropped zip reads exactly as the folder it was made from (agni issue
// 854). It nests: a zip inside a zip is a directory too. Everything else is fsys unchanged.
//
// An archive is read whole on first use and kept, which suits a browser mount whose bytes are in
// memory already. A file that ends in `.zip` but does not parse stays a plain file.
func ExpandZips(fsys fs.FS) fs.FS {
	return &zipFS{base: fsys, open: map[string]*zip.Reader{}}
}

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

func (z *zipFS) Open(name string) (fs.File, error) {
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
	return z.asDirs(name, des), nil
}

func (z *zipFS) Stat(name string) (fs.FileInfo, error) {
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
	z   *zipFS
	dir string
}

func (d *zipDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	des, err := d.ReadDirFile.ReadDir(n)
	return d.z.asDirs(d.dir, des), err
}
