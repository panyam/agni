package mcpserver

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// blockingIngester holds an ingest of slow until release closes, and answers any other at once.
type blockingIngester struct {
	slow    string
	release chan struct{}
	mu      sync.Mutex
	ingests map[string]int
}

func (b *blockingIngester) Ingest(_ context.Context, req *webapi.IngestRequest) (*webapi.Snapshot, error) {
	uri := req.GetSnapshot().GetUri()
	if uri == b.slow {
		<-b.release
	}
	b.mu.Lock()
	b.ingests[uri]++
	b.mu.Unlock()
	return &webapi.Snapshot{Name: "snapshots/" + uri, Uri: uri}, nil
}

func (b *blockingIngester) DeleteSnapshot(context.Context, *webapi.DeleteSnapshotRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// A design being read holds up questions about itself and no other, and two questions about one
// design still read it once (agni issue 1121).
func TestReadingOneDesignDoesNotHoldUpAnother(t *testing.T) {
	a, _ := artifact.Parse("mount://m/a.edn")
	b, _ := artifact.Parse("mount://m/b.edn")
	ing := &blockingIngester{slow: a.String(), release: make(chan struct{}), ingests: map[string]int{}}
	d := newDesigns(ing)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := d.snapshot(context.Background(), a); err != nil {
				t.Error(err)
			}
		}()
	}
	done := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond) // let both questions about a start reading it
		_, err := d.snapshot(context.Background(), b)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		close(ing.release)
		t.Fatal("a question about b waited for a's read")
	}
	close(ing.release)
	wg.Wait()
	if n := ing.ingests[a.String()]; n != 1 {
		t.Errorf("a was ingested %d times for two questions, want once", n)
	}
}
