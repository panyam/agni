package mountset

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

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
