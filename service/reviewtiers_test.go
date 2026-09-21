package service

import (
	"context"
	"sync"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// boardRecordingLoader reads a netlist that carries no copper, as a served netlist does, and records
// every artifact a board was asked of. Only the declared board file yields geometry.
type boardRecordingLoader struct {
	board string
	mu    sync.Mutex
	asked []string
}

func (l *boardRecordingLoader) Design(context.Context, artifact.URI, ...ReadOption) (*ir.Design, error) {
	return &ir.Design{}, nil
}

func (l *boardRecordingLoader) Board(_ context.Context, u artifact.URI) (*geom.BoardGeometry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, u.String())
	if u.String() == l.board {
		return thinBoard(), nil
	}
	return nil, nil
}

func (l *boardRecordingLoader) Manifest(context.Context, artifact.URI) (review.Manifest, error) {
	return review.Manifest{}, nil
}

func (l *boardRecordingLoader) DesignHash(context.Context, artifact.URI) (string, error) {
	return "", nil
}

func withBoardCompanion() *webapi.Design {
	return &webapi.Design{
		Uri:           "mount://m/d",
		EntryUri:      "mount://m/d/board.edn",
		CompanionUris: []string{"mount://m/d/board.kicad_pcb"},
	}
}

// A review scored in the browser read no board where the CLI read the declared one, so board-tier
// items came back not-applicable on one surface and fail on the other (agni issue 646). The review was
// the one analysis path that never resolved a design's tiers, which is C32 at one more call site.
func TestCreateReviewReadsTheDeclaredBoard(t *testing.T) {
	const board = "mount://m/d/board.kicad_pcb"
	for _, tc := range []struct {
		name    string
		asNamed bool
		want    string
	}{
		{"declared board attached", false, board},
		// The opt-out the CLI's --as-named sends: read exactly what was named, which carries no board.
		{"as named reads the entry alone", true, "mount://m/d/board.edn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := &boardRecordingLoader{board: board}
			svc := NewReviewService(l, NewMemReviewStore(), check.DefaultCatalog(), nil, nil, testReviewEnv, "",
				&ProjectResolver{Store: declaringStore{design: withBoardCompanion()}})
			_, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
				DesignUri: "mount://m/d/board.edn",
				Manifest:  fixtureManifest(t, "review/mini.yaml"),
				AsNamed:   tc.asNamed,
			})
			if err != nil {
				t.Fatalf("CreateReview: %v", err)
			}
			if len(l.asked) != 1 || l.asked[0] != tc.want {
				t.Errorf("board read from %v, want exactly %s", l.asked, tc.want)
			}
		})
	}
}
