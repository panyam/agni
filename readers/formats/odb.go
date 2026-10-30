package formats

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/odb"
)

// odbJobExt is the registry key an ODB++ job's entry is filed under. A job is a directory, and the
// file that enters it, `<job>/matrix/matrix`, has no extension, so lowerExt maps that path here. The
// key is spelled so no real file carries it (`.odb` is an OpenDocument database).
const odbJobExt = ".odb++"

// registerODB files the three ways an ODB++ job arrives: its matrix file inside a directory (and
// inside a zip a host expands into one), a .tgz, and a .zip read as an archive.
func registerODB() {
	Register(&Format{Ext: odbJobExt, Name: "odb++", Design: readODBJob, Board: readODBJobBoard})
	for _, ext := range []string{".tgz", ".zip"} {
		Register(&Format{Ext: ext, Name: "odb++", Design: readODBArchive, Board: readODBArchiveBoard})
	}
}

func readODBJob(l *Loader, p string) (*ir.Design, error) {
	return odb.Read(jobFS(l, p), p)
}

func readODBJobBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	return odb.ReadBoardGeometry(jobFS(l, p), p)
}

func readODBArchive(l *Loader, p string) (*ir.Design, error) {
	fsys, err := archiveFS(l, p)
	if err != nil {
		return nil, err
	}
	return odb.Read(fsys, p)
}

func readODBArchiveBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	fsys, err := archiveFS(l, p)
	if err != nil {
		return nil, err
	}
	return odb.ReadBoardGeometry(fsys, p)
}

func archiveFS(l *Loader, p string) (fs.FS, error) {
	b, err := l.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return odb.FromArchive(b)
}

// jobFS presents the job a matrix path belongs to as an fs.FS whose every open goes through the
// loader, so the reads are recorded (Touched) and resolve wherever the loader's names do.
func jobFS(l *Loader, matrixPath string) fs.FS {
	return loaderFS{l: l, root: l.dir(l.dir(matrixPath))}
}

type loaderFS struct {
	l    *Loader
	root string
}

func (f loaderFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if f.l == nil || f.l.FS == nil {
		return f.l.Open(filepath.Join(f.root, filepath.FromSlash(name)))
	}
	return f.l.Open(path.Join(f.root, name))
}

// odbKey is the registry key for a path that enters an ODB++ job, or "" for any other path.
func odbKey(name string) string {
	if odb.IsMatrixPath(filepath.ToSlash(name)) {
		return odbJobExt
	}
	if strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
		return ".tgz"
	}
	return ""
}

// DesignRoot is the directory a design entered by entry spans, when that is wider than the entry's
// own folder: an ODB++ job's matrix file sits two levels inside the job, and every layer the job
// reads is under the job. It answers "" for any other entry. Hosts that gather a design's files to
// send elsewhere (ListDesignFiles) walk this directory.
func DesignRoot(entry string) string {
	if odb.IsMatrixPath(filepath.ToSlash(entry)) {
		return odb.JobDir(filepath.ToSlash(entry))
	}
	return ""
}

// IsArchive reports whether a path names a .zip or .tgz, the archive extensions registered for
// ODB++ alone.
func IsArchive(name string) bool {
	switch lowerExt(name) {
	case ".zip", ".tgz":
		return true
	}
	return false
}

// IsODBArchive reports whether an archive's bytes hold an ODB++ job.
func IsODBArchive(b []byte) bool { return odb.IsArchiveJob(b) }
