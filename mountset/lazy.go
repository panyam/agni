package mountset

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"

	"github.com/panyam/goapplib/storage/mounts"
)

// Lazy reports whether m is a GitHub snapshot whose files are fetched the first time something
// reads them (agni issue 1877). A runtime GitHub mount whose commit passes the cap is made that way,
// so it can be mounted as long as each folder read from it fits. A commit under the cap is
// snapshotted whole, as before, and its view rebuilds from the blobs alone.
func Lazy(m mounts.Mount) bool {
	return m.Kind == mounts.KindGitHub && m.Mode == mounts.ModeSnapshot && m.Manifest == ""
}

// FS returns m's files as an fs.FS. For a lazy mount, listing a folder reads only the commit's tree
// and opening a file fetches that one file, so walking the mount costs no download. Any other mount
// is its host folder, Root.
func (t *Table) FS(m mounts.Mount) fs.FS {
	if Lazy(m) {
		sub, err := fs.Sub(t.reg.FS(context.Background()), m.Name)
		if err == nil {
			return sub
		}
	}
	return os.DirFS(t.Root(m))
}

// lazyMount returns the lazy mount uri names, if it is one.
func (t *Table) lazyMount(uri mounts.URI) (mounts.Mount, bool) {
	m, ok := t.Find(uri.Mount)
	return m, ok && Lazy(m)
}

// fetchNeighbours fetches the folder holding the file uri names, before the file itself, since a
// reader opens files beside the one it was given, as a KiCad schematic opens its sheets. A folder
// past the cap is left out, and only the file is fetched then, so a board packed in one archive
// still opens from a folder of nine. A file at the mount's root fetches only itself, since its
// folder is the whole repo.
func (t *Table) fetchNeighbours(ctx context.Context, m mounts.Mount, uri mounts.URI) error {
	dir := path.Dir(uri.Path)
	if uri.Path == "" || dir == "." {
		return nil
	}
	if fi, err := fs.Stat(t.FS(m), uri.Path); err != nil || fi.IsDir() {
		return nil
	}
	_, err := t.reg.HostPath(ctx, mounts.URI{Mount: uri.Mount, Path: dir})
	if errors.Is(err, mounts.ErrTooLarge) {
		return nil
	}
	return err
}
