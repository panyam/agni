package service

import (
	"context"
	"sync"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// perArtifactHash answers a distinct digest per artifact, which is what a real loader does: two
// different files hash to two different values. fakeLoader's canned hash is the same string whatever
// it is asked about, so under it a caller hashing the WRONG artifact is indistinguishable from one
// hashing the right artifact. That is why this bug survived a suite that already had a hash fixture.
type perArtifactHash struct{ fakeLoader }

func (perArtifactHash) DesignHash(_ context.Context, u artifact.URI) (string, error) {
	return "sha256:" + u.Path, nil
}

// The three spellings of one design must report ONE revision identity, for the same reason they must
// resolve to one set of tiers (agni issue 656, constraint C32): they analyse the same bytes.
//
// TestSourcesAgreeAcrossEverySpelling is the tier half of this and the hash half was left behind.
// GetDesign resolves the netlist tier and then hands DesignHash the REQUEST instead, so asking by
// companion reports the companion's digest while a link the CLI minted carries the entry's. The
// viewer compares the two and draws a stale-link note on a design that is perfectly in sync, which is
// the failure localLoader.DesignHash's own doc says it fixed on the CLI side and nobody fixed here.
func TestRevisionIdentityAgreesAcrossEverySpelling(t *testing.T) {
	ld := perArtifactHash{fakeLoader{geom: twoSheetGeom()}}
	svc := NewDesignService(ld, noNative{}, render.Style{}, &ProjectResolver{Store: declaringStore{design: withCompanion()}})

	got := map[string]string{}
	for _, spelling := range []string{"mount://m/d", "mount://m/d/board.edn", "mount://m/d/board.eds"} {
		resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: spelling})
		if err != nil {
			t.Fatalf("GetDesign(%s): %v", spelling, err)
		}
		got[spelling] = resp.GetContentHash()
	}
	// The declared ENTRY is the artifact a read of this design actually opens, so it is the one whose
	// bytes the revision identity names.
	const want = "sha256:d/board.edn"
	for spelling, h := range got {
		if h != want {
			t.Errorf("GetDesign(%s) content_hash = %q, want the declared entry's %q", spelling, h, want)
		}
	}
}

// hashRecordingLoader records every artifact a revision identity was asked of, the way
// boardRecordingLoader records every artifact a board was asked of.
type hashRecordingLoader struct {
	*boardRecordingLoader
	mu     sync.Mutex
	hashed []string
}

func (l *hashRecordingLoader) DesignHash(_ context.Context, u artifact.URI) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hashed = append(l.hashed, u.String())
	return "sha256:" + u.Path, nil
}

// A review's revision identity is PERSISTED, so getting it wrong outlives the request. CreateReview
// resolves the netlist tier, scores against it, and then hashed the request instead, so a review
// created by naming the design folder recorded the folder as the thing it analysed. A folder does not
// hash at all in production, which turns the stored provenance into the empty third state on a run
// that read perfectly good bytes.
func TestReviewRecordsTheEntrysRevision(t *testing.T) {
	l := &hashRecordingLoader{boardRecordingLoader: &boardRecordingLoader{board: "mount://m/d/board.kicad_pcb"}}
	svc := NewReviewService(l, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "",
		&ProjectResolver{Store: declaringStore{design: withBoardCompanion()}})
	if _, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		DesignUri: "mount://m/d",
		Manifest:  fixtureManifest(t, "review/mini.yaml"),
	}); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if len(l.hashed) != 1 || l.hashed[0] != "mount://m/d/board.edn" {
		t.Errorf("review recorded the revision of %v, want exactly the declared entry", l.hashed)
	}
}
