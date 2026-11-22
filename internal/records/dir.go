package records

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Dir is a Store that keeps each record as a file under a folder, at its key's path. A write goes
// to a temp file beside the record and is renamed into place. Every write takes the record's lock
// file first, so Create's check for a taken key and Replace's check of the version hold across
// processes sharing the folder. A version is the hash of the record's contents.
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

// lockWait is how long a write waits for another writer's lock on the same record, and lockStale is
// how old a lock must be before it counts as left by a writer that died holding it.
const (
	lockWait  = 30 * time.Second
	lockStale = time.Minute
)

// lock takes key's lock, a file beside the record that only one writer can create, and returns its
// release. It serializes writes to one record across processes sharing the folder, so Replace's check
// and write happen as one step. A lock older than lockStale is taken over.
func (d *Dir) lock(ctx context.Context, key string) (func(), error) {
	dst := d.path(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return nil, err
	}
	lk := filepath.Join(filepath.Dir(dst), ".lock-"+filepath.Base(dst))
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(lk, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- beside a checked key under the store's own folder
		if err == nil {
			f.Close()
			return func() { os.Remove(lk) }, nil
		}
		// On Windows a lock file another writer just removed can't be created again until its last
		// handle closes, and that reads as a permission error. It's the same wait as a held lock.
		busy := errors.Is(err, fs.ErrExist) || (runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission))
		if !busy {
			return nil, err
		}
		if info, err := os.Stat(lk); err == nil && time.Since(info.ModTime()) > lockStale {
			os.Remove(lk)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("record %s: another writer has held its lock for %s", key, lockWait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// sharedWait is how long a rename or remove keeps trying while a reader has the file open.
const sharedWait = 2 * time.Second

// retryShared runs op again while it fails with a permission error on Windows. Reads take no lock,
// and Windows refuses to rename over or remove a file another handle has open, with "Access is
// denied". The reader closes the file within milliseconds, so a short retry gets through. Elsewhere
// op runs once, since a rename over an open file succeeds there.
func retryShared(op func() error) error {
	err := op()
	if runtime.GOOS != "windows" {
		return err
	}
	for deadline := time.Now().Add(sharedWait); err != nil && errors.Is(err, fs.ErrPermission) && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
		err = op()
	}
	return err
}

// write puts body at dst through a temp file and a rename, so a reader sees the old file or the new
// one.
func write(dst string, body []byte) error {
	tmp, err := temp(dst, body)
	if err != nil {
		return err
	}
	if err := retryShared(func() error { return os.Rename(tmp, dst) }); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Put writes key under its lock.
func (d *Dir) Put(ctx context.Context, key string, body []byte) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	unlock, err := d.lock(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	return write(d.path(key), body)
}

// Create writes key under its lock, unless the file exists.
func (d *Dir) Create(ctx context.Context, key string, body []byte) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	unlock, err := d.lock(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	if _, err := os.Lstat(d.path(key)); err == nil {
		return fmt.Errorf("record %s: %w", key, fs.ErrExist)
	}
	return write(d.path(key), body)
}

// Delete removes the file at key under its lock. Its folder stays, and List treats an empty folder
// as no folder.
func (d *Dir) Delete(ctx context.Context, key string) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	unlock, err := d.lock(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	return retryShared(func() error { return os.Remove(d.path(key)) })
}

// version is a record's contents' hash. Two writes of the same bytes share one, which Replace may
// treat as no change, since nothing about the record differs.
func version(body []byte) Version {
	h := sha256.Sum256(body)
	return Version(hex.EncodeToString(h[:16]))
}

// Read reads the file at key and hashes it for its version.
func (d *Dir) Read(ctx context.Context, key string) ([]byte, Version, error) {
	b, err := d.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return b, version(b), nil
}

// Replace checks the file's hash and writes it, both under the record's lock.
func (d *Dir) Replace(ctx context.Context, key string, body []byte, ver Version) error {
	if err := CheckKey(key); err != nil {
		return err
	}
	unlock, err := d.lock(ctx, key)
	if err != nil {
		return err
	}
	defer unlock()
	cur, err := d.Get(ctx, key)
	if err != nil {
		return err
	}
	if version(cur) != ver {
		return fmt.Errorf("record %s: %w", key, ErrConflict)
	}
	return write(d.path(key), body)
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
