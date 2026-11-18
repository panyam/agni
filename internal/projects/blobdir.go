package projects

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// BlobDir keeps file contents by their SHA-256, one file each, under blobs/sha256/ab/cdef... in the
// uploads mount (agni issue 1739). A blob is written once and never changed, so a design's older
// revisions keep their bytes until the sweep finds nothing pointing at them. Writes go through a temp
// file and a rename, so a reader sees a whole blob or none.
type BlobDir struct{ root string }

// blobsDir is the folder under the uploads mount that holds the blobs.
const blobsDir = "blobs"

// NewBlobDir returns the blob folder under uploads, created if absent.
func NewBlobDir(uploads string) (*BlobDir, error) {
	root := filepath.Join(uploads, blobsDir, "sha256")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &BlobDir{root: root}, nil
}

// rel is a blob's path under the blob folder, "ab/cdef...", or false for anything that is not a
// "sha256:<64 hex>" hash, so no hash names a path outside the folder.
func (d *BlobDir) rel(hash string) (string, bool) {
	hex, ok := strings.CutPrefix(hash, "sha256:")
	if !ok || len(hex) != 64 || strings.Trim(hex, "0123456789abcdef") != "" {
		return "", false
	}
	return path.Join(hex[:2], hex[2:]), true
}

// Path is where the blob for hash lives, for a caller that links it into a design's view. It reports
// false for a malformed hash.
func (d *BlobDir) Path(hash string) (string, bool) {
	rel, ok := d.rel(hash)
	if !ok {
		return "", false
	}
	return filepath.Join(d.root, filepath.FromSlash(rel)), true
}

// Put stores b and returns its hash. Storing contents already held writes nothing.
func (d *BlobDir) Put(b []byte) (string, error) {
	h := hashOf(b)
	p, _ := d.Path(h)
	if _, err := os.Stat(p); err == nil {
		return h, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".put-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return h, nil
}

// Get returns the contents stored under hash. A hash the folder lacks, or a malformed one, reports
// false.
func (d *BlobDir) Get(hash string) ([]byte, bool) {
	rel, ok := d.rel(hash)
	if !ok {
		return nil, false
	}
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return nil, false
	}
	defer r.Close()
	b, err := r.ReadFile(rel)
	if err != nil || hashOf(b) != hash {
		return nil, false
	}
	return b, true
}

// Sweep removes every blob that keep does not hold and that is older than grace, and returns how
// many it removed. The grace period spares a blob a client uploaded a moment ago and has not yet
// named in a write.
func (d *BlobDir) Sweep(keep map[string]bool, grace time.Duration) (int, error) {
	cutoff := time.Now().Add(-grace)
	r, err := os.OpenRoot(d.root)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	// Every read and removal goes through r, so a symlink planted in the folder cannot lead the sweep
	// outside it.
	removed := 0
	err = fs.WalkDir(r.FS(), ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() || strings.HasPrefix(de.Name(), ".") {
			return err
		}
		h := "sha256:" + strings.ReplaceAll(p, "/", "")
		if keep[h] {
			return nil
		}
		fi, err := de.Info()
		if err != nil || fi.ModTime().After(cutoff) {
			return err
		}
		if err := r.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("sweeping %s: %w", p, err)
		}
		removed++
		return nil
	})
	return removed, err
}
