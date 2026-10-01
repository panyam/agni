package param

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Fetcher is a published PartSpec corpus reached over some transport, the shape of the contract's
// PartSpecService (agni issue 749). It is declared here, transport-neutral, so the caching below is
// the engine's and a Connect or in-process implementation is only an adapter.
type Fetcher interface {
	// BatchGet returns the current spec for each MPN that has one, and the corpus generation the
	// answer was read at. An MPN with no spec is absent, not an error.
	BatchGet(ctx context.Context, mpns []string) ([]*parampb.PartSpec, uint64, error)
	// Generation returns the corpus generation alone.
	Generation(ctx context.Context) (uint64, error)
}

// Prefetcher is a ParamProvider that has to fetch before it can answer. Lookup returns no error, so a
// provider that reaches its corpus over a network could only turn an unreachable service into a nil,
// which reads as "this part is not seeded" and makes every datasheet rule skip it silently, the very
// shrink LoadSet's all-or-nothing rule exists to prevent. Whoever builds a model calls Prefetch with
// every MPN the design names, where a context and an error exist, and Lookup then answers from what
// was fetched.
type Prefetcher interface {
	ParamProvider
	Prefetch(ctx context.Context, mpns []string) error
}

// Remote is a caching Prefetcher over a Fetcher. It keeps every spec it has fetched, and every MPN it
// learned has none, for as long as the corpus generation holds. It checks the generation at most once
// per maxAge, and a generation that moved empties the cache, so a spec published while a server runs
// reaches the next request without a restart.
type Remote struct {
	fetch  Fetcher
	maxAge time.Duration
	now    func() time.Time

	mu      sync.Mutex
	gen     uint64
	checked time.Time // when gen was last confirmed; zero before the first check
	specs   map[string]*parampb.PartSpec
}

// NewRemote returns a Remote over f that re-checks the corpus generation at most once per maxAge.
func NewRemote(f Fetcher, maxAge time.Duration) *Remote {
	return &Remote{fetch: f, maxAge: maxAge, now: time.Now, specs: map[string]*parampb.PartSpec{}}
}

// maxRefetch bounds how many times one Prefetch starts over because the generation moved under it.
const maxRefetch = 3

// Prefetch makes every named MPN answerable by Lookup, fetching what is not cached at the current
// generation in one batch. Any failure to reach the corpus is returned, never swallowed.
func (r *Remote) Prefetch(ctx context.Context, mpns []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checked.IsZero() || r.now().Sub(r.checked) >= r.maxAge {
		g, err := r.fetch.Generation(ctx)
		if err != nil {
			return fmt.Errorf("the PartSpec service: %w", err)
		}
		r.adopt(g)
	}
	for attempt := 0; attempt < maxRefetch; attempt++ {
		missing := r.missing(mpns)
		if len(missing) == 0 {
			return nil
		}
		specs, g, err := r.fetch.BatchGet(ctx, missing)
		if err != nil {
			return fmt.Errorf("the PartSpec service: %w", err)
		}
		if g != r.gen {
			// The corpus moved between the generation check and this answer, so everything cached
			// may be stale. Keep this answer, which is at g, and fetch the rest again.
			r.adopt(g)
		}
		for _, k := range missing {
			r.specs[k] = nil
		}
		for _, s := range specs {
			r.specs[strings.ToUpper(s.GetMpn())] = s
		}
	}
	return fmt.Errorf("the PartSpec service: the corpus changed %d times during one fetch", maxRefetch)
}

// Lookup answers from what Prefetch fetched. An MPN that was never prefetched answers nil, like an
// unseeded one, so a caller that skips Prefetch gets no datasheet tier rather than a wrong one.
func (r *Remote) Lookup(mpn string) *parampb.PartSpec {
	if mpn == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.specs[strings.ToUpper(mpn)]
}

// adopt records generation g as current, emptying the cache when it differs from the one held.
func (r *Remote) adopt(g uint64) {
	if !r.checked.IsZero() && g != r.gen {
		r.specs = map[string]*parampb.PartSpec{}
	}
	r.gen, r.checked = g, r.now()
}

// missing returns the upper-cased MPNs not yet cached, each once.
func (r *Remote) missing(mpns []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range mpns {
		k := strings.ToUpper(m)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		if _, ok := r.specs[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}
