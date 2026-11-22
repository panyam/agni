package records

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Dir is a Store that keeps each record as a file under a folder, at its key's path. A write goes
// to a temp file beside the record and is renamed into place. Create hard-links the temp file to
// the key, which fails when the key exists, and falls back to an exclusive create on a filesystem
// without hard links.
type Dir struct {
	root string
}

// NewDir returns a Dir over root, created if absent.
func NewDir(root string) (*Dir, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Dir{root: abs}, nil
}

// Root is the folder d keeps its records in.
func (d *Dir) Root() string { return d.root }

var _ Store = (*Dir)(nil)

func (d *Dir) path(key string) string { return filepath.Join(d.root, filepath.FromSlash(key)) }

// Get reads the file at key through an os.Root over the store's folder, so a symlink there cannot
// lead a read outside it.
func (d *Dir) Get(_ context.Context, key string) ([]byte, error) {
	if err := CheckKey(key); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.ReadFile(filepath.FromSlash(key))
}

// temp writes body to a temp file in dst's folder, making the folder, and returns its path.
func temp(dst string, body []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".new-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// Put writes key through a temp file and a rename.
func (d *Dir) Put(_ context.Context, key string, body []byte) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	dst := d.path(key)
	tmp, err := temp(dst, body)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Create writes key unless it exists.
func (d *Dir) Create(_ context.Context, key string, body []byte) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	dst := d.path(key)
	tmp, err := temp(dst, body)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	err = os.Link(tmp, dst)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return err
	}
	// No hard links here. An exclusive create still refuses a taken key, though a reader may see the
	// record while it is written.
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a checked key under the store's own folder
	if err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Delete removes the file at key. Its folder stays, and List treats an empty folder as no folder.
func (d *Dir) Delete(_ context.Context, key string) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	return os.Remove(d.path(key))
}

// List reads dir's folder. It leaves out dot files, which are temp files or an operator's notes, and
// folders holding no record.
func (d *Dir) List(_ context.Context, dir string) ([]Entry, error) {
	if err := CheckDir(dir); err != nil {
		return nil, err
	}
	ents, err := os.ReadDir(d.path(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(ents))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") || (e.IsDir() && !holdsRecords(filepath.Join(d.path(dir), e.Name()))) {
			continue
		}
		out = append(out, Entry{Name: e.Name(), Dir: e.IsDir()})
	}
	sortEntries(out)
	return out, nil
}

// holdsRecords reports whether a file other than a dot file sits anywhere under dir. A folder
// holding none is left out of List, as SQL has no row to list it by.
func holdsRecords(dir string) bool {
	found := errors.New("found")
	err := filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(e.Name(), ".") && p != dir {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !e.IsDir() {
			return found
		}
		return nil
	})
	return errors.Is(err, found)
}

// DeleteDir removes dir's folder and everything in it.
func (d *Dir) DeleteDir(_ context.Context, dir string) error {
	if err := CheckDir(dir); err != nil {
		return err
	}
	if dir == "" {
		ents, err := os.ReadDir(d.root)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if err := os.RemoveAll(filepath.Join(d.root, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return os.RemoveAll(d.path(dir))
}
