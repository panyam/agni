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

// odbJobExt is the registry key for an ODB++ job's entry. A job is a directory. The file that enters
// it is `<job>/matrix/matrix`, which has no extension, so lowerExt maps that path to this key. No
// real file has this extension, since `.odb` is an OpenDocument database.
const odbJobExt = ".odb++"

// registerODB registers the three ways an ODB++ job arrives. One is its matrix file inside a
// directory, which also covers a zip a host expands into one. The others are a .tgz, and a .zip read
// as an archive.
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

// jobFS presents a matrix path's job as an fs.FS. Every open goes through the loader. So the reads
// are recorded (Touched), and they resolve wherever the loader's names do.
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

// DesignRoot returns the directory a design spans when that is wider than its entry's own folder.
// An ODB++ job's matrix file sits two levels inside the job, and every layer the job reads is under
// the job. DesignRoot answers "" for any other entry. A host that gathers a design's files to send
// elsewhere (ListDesignFiles) walks this directory.
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
