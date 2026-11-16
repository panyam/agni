package fshost

import (
	"io"
	"io/fs"
	"sort"
	"strings"

	"github.com/panyam/agni/internal/archivefs"
)

// ExpandArchives presents every archive in fsys, a zip, a tar or a 7z, compressed or not, as a
// directory of the same name holding the archive's contents (agni issues 854 and 1591). So a dropped
// archive reads exactly as the folder it was made from, and every reader reads it as it reads any
// folder. It nests, so an archive inside an archive is a directory too. Everything else is fsys
// unchanged. internal/archivefs chooses each archive's decoder from its bytes and holds it to its
// bounds.
//
// An archive is read whole on first use and kept. That suits a browser mount, whose bytes are in
// memory already. A file that has an archive's name and does not parse is still a plain file.
//
// A file at OverlayDir/<path> in fsys reads as <path>, over whatever is there, archives included.
// OverlayDir itself is hidden. The page uses it to write a design.yaml beside a design that came
// out of an archive. Until it is expanded, an archive is a file. The store under the browser mount
// refuses a path whose parent is a file, so the page cannot write beside the design directly.
func ExpandArchives(fsys fs.FS) fs.FS {
	return &overlaid{base: fsys, exp: archivefs.Expand(fsys)}
}

// OverlayDir holds the files that read over an expanded tree. web/src/wasm/drop.ts writes into it.
const OverlayDir = ".agni-overlay"

type overlaid struct {
	base fs.FS // reads the overlay
	exp  fs.FS // reads everything else, with archives expanded
}

// overlay returns the overlay's path for name, or "" when name is the overlay itself or the root.
func overlay(name string) string {
	if name == "." || name == OverlayDir || strings.HasPrefix(name, OverlayDir+"/") {
		return ""
	}
	return OverlayDir + "/" + name
}

// overlaid reports the overlay's file for name, when there is one.
func (o *overlaid) overlaid(name string) (string, bool) {
	p := overlay(name)
	if p == "" || !fs.ValidPath(name) {
		return "", false
	}
	st, err := fs.Stat(o.base, p)
	return p, err == nil && !st.IsDir()
}

func (o *overlaid) Open(name string) (fs.File, error) {
	if p, ok := o.overlaid(name); ok {
		return o.base.Open(p)
	}
	f, err := o.exp.Open(name)
	if err != nil {
		return nil, err
	}
	if d, ok := f.(fs.ReadDirFile); ok {
		return &overlayDirFile{ReadDirFile: d, o: o, dir: name}, nil
	}
	return f, nil
}

func (o *overlaid) ReadDir(name string) ([]fs.DirEntry, error) {
	des, err := fs.ReadDir(o.exp, name)
	if err != nil {
		return nil, err
	}
	return o.merge(name, des), nil
}

func (o *overlaid) Stat(name string) (fs.FileInfo, error) {
	if p, ok := o.overlaid(name); ok {
		return fs.Stat(o.base, p)
	}
	return fs.Stat(o.exp, name)
}

// merge adds the overlay's entries for dir to a listing and hides the overlay itself, in name order.
func (o *overlaid) merge(dir string, des []fs.DirEntry) []fs.DirEntry {
	out := des[:0:0]
	have := map[string]bool{}
	for _, de := range des {
		if dir == "." && de.Name() == OverlayDir {
			continue
		}
		have[de.Name()] = true
		out = append(out, de)
	}
	over := OverlayDir
	if dir != "." {
		over = overlay(dir)
	}
	if extra, err := fs.ReadDir(o.base, over); err == nil {
		for _, de := range extra {
			if !have[de.Name()] {
				out = append(out, de)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	}
	return out
}

// overlayDirFile is a directory opened through ExpandArchives. Reading it through the file, as
// fs.WalkDir does, also merges the overlay.
type overlayDirFile struct {
	fs.ReadDirFile
	o    *overlaid
	dir  string
	list []fs.DirEntry
	read bool
}

// ReadDir lists the whole directory once, merged with the overlay, and pages through it, so reading
// through the file agrees with ReadDir on the FS.
func (d *overlayDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		des, err := d.ReadDirFile.ReadDir(-1)
		if err != nil {
			return nil, err
		}
		d.list, d.read = d.o.merge(d.dir, des), true
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
