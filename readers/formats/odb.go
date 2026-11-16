package formats

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/archivefs"
	"github.com/panyam/agni/readers/odb"
)

// odbJobExt is the registry key for an ODB++ job's entry. A job is a directory. The file that enters
// it is `<job>/matrix/matrix`, which has no extension, so lowerExt maps that path to this key. No
// real file has this extension, since `.odb` is an OpenDocument database.
const odbJobExt = ".odb++"

// registerODB registers the two ways an ODB++ job arrives. One is its matrix file inside a directory.
// The other is an archive named as a design, a .zip, .tar, .7z or compressed tar holding one job,
// usually wrapped in a folder. internal/archivefs decodes every archive kind alike and shows it as the
// folder it was made from (agni issue 1591), so this adapter reads that folder through the loader. A
// host that shows archives as folders reaches the job by its matrix path instead, as it would any
// folder.
func registerODB() {
	Register(&Format{Ext: odbJobExt, Name: "odb++", Design: readODBJob, Board: readODBJobBoard})
	for _, ext := range archiveExts {
		Register(&Format{Ext: ext, Name: "odb++", Design: readODBArchive, Board: readODBArchiveBoard})
	}
}

// archiveExts are the registry keys of an archive named as a design. A compressed tar's two-part
// suffix maps onto its one-part key in odbKey.
var archiveExts = []string{".zip", ".7z", ".tar", ".tgz", ".tbz2", ".txz", ".tzst"}

func readODBJob(l *Loader, p string) (*ir.Design, error) {
	return odb.Read(jobFS(l, p), p)
}

func readODBJobBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	return odb.ReadBoardGeometry(jobFS(l, p), p)
}

func readODBArchive(l *Loader, p string) (*ir.Design, error) {
	fsys, err := archiveJob(l, p)
	if err != nil {
		return nil, err
	}
	return odb.Read(fsys, p)
}

func readODBArchiveBoard(l *Loader, p string) (*geom.BoardGeometry, error) {
	fsys, err := archiveJob(l, p)
	if err != nil {
		return nil, err
	}
	return odb.ReadBoardGeometry(fsys, p)
}

// archiveJob presents the job inside an archive. A host that shows archives as folders, as the
// browser's drop mount does, reads the members through the loader, so every read is recorded. Any
// other host hands over the archive file, whose bytes archivefs decodes. On the host filesystem the
// archive file stands for its members in the record, since they have no stamp of their own.
func archiveJob(l *Loader, p string) (fs.FS, error) {
	var inside fs.FS
	switch {
	case l == nil || l.FS == nil:
		if l != nil {
			l.Touched.note(nil, p)
		}
		members, err := archivefs.OSMembers(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		inside = members
	case isDir(l.FS, p):
		inside = loaderFS{l: l, root: p}
	default:
		b, err := l.ReadFile(p)
		if err != nil {
			return nil, err
		}
		members, err := archivefs.Open(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		inside = members
	}
	root, err := odb.FindRoot(inside)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if root == "." {
		return inside, nil
	}
	return fs.Sub(inside, root)
}

func isDir(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && st.IsDir()
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
	low := strings.ToLower(name)
	for long, short := range compressedTars {
		if strings.HasSuffix(low, long) {
			return short
		}
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

// compressedTars maps a compressed tar's two-part suffix onto its registry key.
var compressedTars = map[string]string{".tar.gz": ".tgz", ".tar.bz2": ".tbz2", ".tar.xz": ".txz", ".tar.zst": ".tzst"}

// IsArchive reports whether a path names an archive, by its name alone.
func IsArchive(name string) bool { return archivefs.Candidate(name) }

// IsODBArchive reports whether an archive's bytes hold an ODB++ job.
func IsODBArchive(b []byte) bool {
	fsys, err := archivefs.Open(b)
	if err != nil {
		return false
	}
	_, err = odb.FindRoot(fsys)
	return err == nil
}
