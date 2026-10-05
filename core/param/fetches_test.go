package param

import (
	"context"
	"testing"
	"time"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

type noFetch struct{}

func (noFetch) BatchGet(context.Context, []string) ([]*parampb.PartSpec, uint64, error) {
	return nil, 0, nil
}

func (noFetch) Generation(context.Context) (uint64, error) { return 0, nil }

// TestFetchesLooksInsideLayered: Layered always implements Prefetch, so a cache that asked "is this
// a Prefetcher" refused to keep any model whose corpus was a local set behind a Layered, which is
// every project with params/ and every server started with --params (agni issue 895).
func TestFetchesLooksInsideLayered(t *testing.T) {
	remote := NewRemote(noFetch{}, time.Minute)
	for _, c := range []struct {
		name string
		p    ParamProvider
		want bool
	}{
		{"nil", nil, false},
		{"a local set", ParamSet{}, false},
		{"a layered local set over nothing", Layered{{Name: CorpusProject, Provider: ParamSet{}}, {Name: CorpusShared, Provider: nil}}, false},
		{"a remote corpus", remote, true},
		{"a local project over a remote shared corpus", Layered{{Name: CorpusProject, Provider: ParamSet{}}, {Name: CorpusShared, Provider: remote}}, true},
	} {
		if got := Fetches(c.p); got != c.want {
			t.Errorf("%s: Fetches = %v, want %v", c.name, got, c.want)
		}
	}
	if _, ok := ParamProvider(Layered(nil)).(Prefetcher); !ok {
		t.Fatal("Layered no longer implements Prefetcher, so the case this test guards against is gone")
	}
}
