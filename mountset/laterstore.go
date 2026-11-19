package mountset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/fsstore"
)

// laterStore is a read-only reader's record store for a records folder that may not exist yet.
// agnids reads the store agni serve writes, often read-only, and agni serve makes the folder only
// when it makes its first mount. Until then the store holds no mounts. Once the folder appears it
// reads it through fsstore, so a mount made after agnids started shows within a second, as it did
// before agni moved onto goapplib storage.
type laterStore struct {
	dir  string
	mu   sync.Mutex
	open *fsstore.Store
}

func (s *laterStore) store() (*fsstore.Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.open != nil {
		return s.open, nil
	}
	if _, err := os.Stat(s.dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	st, err := fsstore.New(s.dir)
	if err != nil {
		return nil, err
	}
	s.open = st
	return st, nil
}

func (s *laterStore) ListMounts(ctx context.Context) ([]mounts.Mount, error) {
	st, err := s.store()
	if st == nil || err != nil {
		return nil, err
	}
	return st.ListMounts(ctx)
}

func (s *laterStore) GetMount(ctx context.Context, name string) (mounts.Mount, error) {
	st, err := s.store()
	if err != nil {
		return mounts.Mount{}, err
	}
	if st == nil {
		return mounts.Mount{}, fmt.Errorf("%w: no mount %q", mounts.ErrNotFound, name)
	}
	return st.GetMount(ctx, name)
}

func (s *laterStore) CreateMount(context.Context, mounts.Mount) error {
	return fmt.Errorf("%w: this reader keeps no mounts", mounts.ErrInvalidArgument)
}

func (s *laterStore) DeleteMount(context.Context, string) error {
	return fmt.Errorf("%w: this reader keeps no mounts", mounts.ErrInvalidArgument)
}

func (s *laterStore) Generation(ctx context.Context) (string, error) {
	st, err := s.store()
	if st == nil || err != nil {
		return "", err
	}
	g, err := st.Generation(ctx)
	return "open:" + g, err
}

// laterBlobs is a read-only reader's blob store for an --uploads folder that may not exist yet, and
// that the reader may not be able to make (agni issue 1767). agnids in its image runs as a user that
// can't write /var/lib/agni, and with no volume mounted there blobs.NewDir's MkdirAll stopped it
// from starting. Until agni serve makes the folder, the store holds nothing. Then it reads it
// through blobs.Dir. It never writes, since a read-only reader neither snapshots nor sweeps.
type laterBlobs struct {
	root string
	mu   sync.Mutex
	open *blobs.Dir
}

// errReadOnlyBlobs is why a read-only reader's blob store refuses a write.
var errReadOnlyBlobs = errors.New("this reader opens --uploads read-only")

func (b *laterBlobs) dir() (*blobs.Dir, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open != nil {
		return b.open, nil
	}
	if _, err := os.Stat(filepath.Join(b.root, "sha256")); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	d, err := blobs.NewDir(b.root)
	if err != nil {
		return nil, err
	}
	b.open = d
	return d, nil
}

func (b *laterBlobs) Put(context.Context, io.Reader) (blobs.Hash, error) {
	return "", errReadOnlyBlobs
}

func (b *laterBlobs) Open(ctx context.Context, h blobs.Hash) (io.ReadCloser, error) {
	d, err := b.dir()
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, blobs.ErrNotFound
	}
	return d.Open(ctx, h)
}

func (b *laterBlobs) Has(ctx context.Context, h blobs.Hash) (bool, error) {
	d, err := b.dir()
	if err != nil || d == nil {
		return false, err
	}
	return d.Has(ctx, h)
}

func (b *laterBlobs) Sweep(context.Context, time.Duration, ...blobs.Referrer) (int, error) {
	return 0, errReadOnlyBlobs
}

func (b *laterBlobs) Link(h blobs.Hash, to string) error {
	d, err := b.dir()
	if err != nil {
		return err
	}
	if d == nil {
		return blobs.ErrNotFound
	}
	return d.Link(h, to)
}
