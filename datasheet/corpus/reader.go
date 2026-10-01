package corpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ErrStale is a corpus whose index no longer describes its files: an entry names a file that is gone,
// or one whose bytes no longer hash to what was indexed, the trace of a hand edit that skipped
// `agnids index`. It is reported rather than served around, because serving the file would hand out
// a spec nobody validated, and skipping it would make the part read as unseeded.
var ErrStale = errors.New("the corpus index is stale; run `agnids index` over the corpus")

// Reader answers the contract's PartSpecService from a published corpus on an fs.FS, through its
// index, and is a param.Fetcher. It reads only what is asked for: the index, then one file per part
// number requested. The index is re-read when its modification time or size changes, so a promotion
// reaches the next request, and a parsed spec is kept by content hash, so an unchanged file is parsed
// once.
type Reader struct {
	fsys fs.FS

	mu      sync.Mutex
	ix      *Index
	ixStamp stamp
	parsed  map[string]*parampb.PartSpec // by Entry.Hash
}

type stamp struct {
	mod  time.Time
	size int64
}

// NewReader returns a Reader over the corpus at fsys. It fails when the corpus has no index, because a
// corpus served without one would answer every part as unseeded.
func NewReader(fsys fs.FS) (*Reader, error) {
	r := &Reader{fsys: fsys, parsed: map[string]*parampb.PartSpec{}}
	if _, err := r.index(); err != nil {
		return nil, err
	}
	return r, nil
}

// Generation implements param.Fetcher.
func (r *Reader) Generation(context.Context) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ix, err := r.index()
	if err != nil {
		return 0, err
	}
	return ix.Generation, nil
}

// BatchGet implements param.Fetcher. Every spec it returns is the file the index names, verified
// against the hash the index recorded when it was validated.
func (r *Reader) BatchGet(_ context.Context, mpns []string) ([]*parampb.PartSpec, uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ix, err := r.index()
	if err != nil {
		return nil, 0, err
	}
	var out []*parampb.PartSpec
	seen := map[string]bool{}
	for _, m := range mpns {
		e, ok := ix.Lookup(m)
		if !ok || seen[e.File] {
			continue
		}
		seen[e.File] = true
		spec, err := r.spec(e)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, spec)
	}
	return out, ix.Generation, nil
}

// index returns the corpus's index, re-reading it when the file changed. The caller holds r.mu.
func (r *Reader) index() (*Index, error) {
	fi, err := fs.Stat(r.fsys, IndexFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w; run `agnids index` over the corpus first", ErrNoIndex)
	}
	if err != nil {
		return nil, err
	}
	st := stamp{fi.ModTime(), fi.Size()}
	if r.ix != nil && st == r.ixStamp {
		return r.ix, nil
	}
	ix, err := Read(r.fsys)
	if err != nil {
		return nil, err
	}
	r.ix, r.ixStamp = ix, st
	return ix, nil
}

// spec returns the parsed spec an entry names, reading and verifying the file unless a spec with that
// hash was already parsed. The caller holds r.mu.
func (r *Reader) spec(e Entry) (*parampb.PartSpec, error) {
	if s, ok := r.parsed[e.Hash]; ok {
		return s, nil
	}
	data, err := fs.ReadFile(r.fsys, e.File)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s, which seeds %s, is gone", ErrStale, e.File, e.MPN)
	}
	if err != nil {
		return nil, err
	}
	if hashOf(data) != e.Hash {
		return nil, fmt.Errorf("%w: %s, which seeds %s, changed since it was indexed", ErrStale, e.File, e.MPN)
	}
	s, err := param.Load(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", e.File, err)
	}
	r.parsed[e.Hash] = s
	return s, nil
}

var _ param.Fetcher = (*Reader)(nil)
