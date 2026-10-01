package param

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// fakeCorpus is a Fetcher over an in-memory corpus whose generation a test moves by hand. It counts
// calls, so a test can tell a cache hit from a fetch.
type fakeCorpus struct {
	specs      map[string]*parampb.PartSpec
	gen        uint64
	err        error
	batchCalls int
	genCalls   int
	asked      []string
	// moveOnBatch bumps the generation the first time BatchGet answers, as a publish landing between
	// the generation check and the fetch would.
	moveOnBatch bool
}

func (f *fakeCorpus) BatchGet(_ context.Context, mpns []string) ([]*parampb.PartSpec, uint64, error) {
	f.batchCalls++
	f.asked = append(f.asked, mpns...)
	if f.err != nil {
		return nil, 0, f.err
	}
	if f.moveOnBatch {
		f.moveOnBatch = false
		f.gen++
	}
	var out []*parampb.PartSpec
	for _, m := range mpns {
		if s, ok := f.specs[strings.ToUpper(m)]; ok {
			out = append(out, s)
		}
	}
	return out, f.gen, nil
}

func (f *fakeCorpus) Generation(context.Context) (uint64, error) {
	f.genCalls++
	return f.gen, f.err
}

func corpusOf(mpns ...string) *fakeCorpus {
	f := &fakeCorpus{specs: map[string]*parampb.PartSpec{}, gen: 1}
	for _, m := range mpns {
		f.specs[m] = &parampb.PartSpec{Mpn: m}
	}
	return f
}

// remoteAt returns a Remote whose clock the test advances.
func remoteAt(f Fetcher, maxAge time.Duration) (*Remote, *time.Time) {
	now := time.Unix(1000, 0)
	r := NewRemote(f, maxAge)
	r.now = func() time.Time { return now }
	return r, &now
}

// After a Prefetch, Lookup answers seeded parts, misses unseeded ones, and matches case-insensitively,
// with one batch for the whole design and nothing fetched again while the generation holds.
func TestRemoteAnswersFromOneBatch(t *testing.T) {
	f := corpusOf("LM1117", "TXB0104")
	r, _ := remoteAt(f, time.Minute)
	if err := r.Prefetch(context.Background(), []string{"lm1117", "TXB0104", "UNSEEDED", "LM1117", ""}); err != nil {
		t.Fatal(err)
	}
	if f.batchCalls != 1 || len(f.asked) != 3 {
		t.Errorf("batch calls %d asking %v; want one batch of the 3 distinct MPNs", f.batchCalls, f.asked)
	}
	if r.Lookup("LM1117") == nil || r.Lookup("txb0104") == nil {
		t.Error("a seeded part was not answered")
	}
	if r.Lookup("UNSEEDED") != nil {
		t.Error("an unseeded part was answered")
	}
	if err := r.Prefetch(context.Background(), []string{"LM1117", "UNSEEDED"}); err != nil {
		t.Fatal(err)
	}
	if f.batchCalls != 1 {
		t.Errorf("a second prefetch of cached parts fetched again (%d batches); a known miss must be cached too", f.batchCalls)
	}
}

// A publish moves the generation, and the next check after maxAge empties the cache, so the new spec
// reaches the next request without a restart. Within maxAge the generation is not asked at all.
func TestRemoteRefetchesWhenTheGenerationMoves(t *testing.T) {
	f := corpusOf("LM1117")
	r, now := remoteAt(f, time.Minute)
	ctx := context.Background()
	if err := r.Prefetch(ctx, []string{"LM1117", "NEWPART"}); err != nil {
		t.Fatal(err)
	}
	f.specs["NEWPART"] = &parampb.PartSpec{Mpn: "NEWPART"}
	f.gen = 2
	if err := r.Prefetch(ctx, []string{"NEWPART"}); err != nil {
		t.Fatal(err)
	}
	if r.Lookup("NEWPART") != nil || f.genCalls != 1 {
		t.Fatalf("within maxAge: NEWPART answered %v after %d generation checks; want the cached miss and no check", r.Lookup("NEWPART") != nil, f.genCalls)
	}
	*now = now.Add(time.Minute)
	if err := r.Prefetch(ctx, []string{"NEWPART"}); err != nil {
		t.Fatal(err)
	}
	if r.Lookup("NEWPART") == nil {
		t.Error("after the generation moved, the newly published part is still a cached miss")
	}
}

// A publish landing between the generation check and the fetch must not leave the cache holding
// answers from two generations.
func TestRemoteRefetchesWhatTheMovedGenerationInvalidated(t *testing.T) {
	f := corpusOf("LM1117", "TXB0104")
	r, _ := remoteAt(f, time.Minute)
	ctx := context.Background()
	if err := r.Prefetch(ctx, []string{"LM1117"}); err != nil {
		t.Fatal(err)
	}
	f.moveOnBatch = true
	if err := r.Prefetch(ctx, []string{"LM1117", "TXB0104"}); err != nil {
		t.Fatal(err)
	}
	if r.Lookup("LM1117") == nil || r.Lookup("TXB0104") == nil {
		t.Error("a part was lost when the generation moved mid-prefetch")
	}
	if f.batchCalls != 3 {
		t.Errorf("batch calls = %d; want 3 (the first, the one that saw the move, and the refetch of LM1117)", f.batchCalls)
	}
}

// An unreachable corpus is an error from Prefetch, never a nil from Lookup that reads as unseeded.
func TestRemoteReportsAnUnreachableCorpus(t *testing.T) {
	f := corpusOf("LM1117")
	f.err = errors.New("connection refused")
	r, _ := remoteAt(f, time.Minute)
	err := r.Prefetch(context.Background(), []string{"LM1117"})
	if err == nil || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("Prefetch = %v, want the transport error", err)
	}
}
