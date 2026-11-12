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

// perArtifactHash answers a distinct digest per artifact, as a real loader does, so two
// different files hash to two different values. fakeLoader's canned hash is the same string whatever
// it is asked about, so under it a caller hashing the WRONG artifact is indistinguishable from one
// hashing the right artifact. So this bug survived a suite that already had a hash fixture.
type perArtifactHash struct{ fakeLoader }

func (perArtifactHash) DesignHash(_ context.Context, u artifact.URI) (string, error) {
	return "sha256:" + u.Path, nil
}

// The three spellings of one design must report ONE revision identity, for the same reason they must
// resolve to one set of tiers (agni issue 656, constraint C32), since they analyse the same bytes.
//
// TestSourcesAgreeAcrossEverySpelling is the tier half of this and the hash half was left behind.
// GetDesign resolved the netlist tier and then handed DesignHash the REQUEST instead. Asking by
// companion then reported the companion's digest, while a link the CLI minted holds the entry's. The
// viewer compared the two and drew a stale-link note on a design that was perfectly in sync.
// localLoader.DesignHash's own doc says it fixed that failure on the CLI side, and nobody had fixed
// it here.
func TestRevisionIdentityAgreesAcrossEverySpelling(t *testing.T) {
	ld := perArtifactHash{fakeLoader{geom: twoSheetGeom()}}
	svc := NewDesignService(ld, noNative{}, render.Style{}, &ProjectResolver{Store: declaringStore{design: withCompanion()}})

	got := map[string]string{}
	for _, spelling := range []string{"mount://m/d", "mount://m/d/board.edn", "mount://m/d/board.eds"} {
		resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), spelling)})
		if err != nil {
			t.Fatalf("GetDesign(%s): %v", spelling, err)
		}
		got[spelling] = resp.GetContentHash()
	}
	// A read of this design actually opens the declared ENTRY, so the revision identity names the
	// entry's bytes.
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
// hash at all in production, so the stored provenance fell into the empty third state on a run that
// read perfectly good bytes.
func TestReviewRecordsTheEntrysRevision(t *testing.T) {
	l := &hashRecordingLoader{boardRecordingLoader: &boardRecordingLoader{board: "mount://m/d/board.kicad_pcb"}}
	svc := NewReviewService(l, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "",
		&ProjectResolver{Store: declaringStore{design: withBoardCompanion()}})
	snap := snapOf(t, svc.Snapshots(), "mount://m/d")
	// Ingesting names the snapshot's own revision from every tier it resolved. The test asks what the
	// review records afterwards.
	l.mu.Lock()
	l.hashed = nil
	l.mu.Unlock()
	if _, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		Snapshot: snap,
		Manifest: fixtureManifest(t, "review/mini.yaml"),
	}); err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if len(l.hashed) != 1 || l.hashed[0] != "mount://m/d/board.edn" {
		t.Errorf("review recorded the revision of %v, want exactly the declared entry", l.hashed)
	}
}
