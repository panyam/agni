package service

import (
	"context"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardIndex says what a board draws, for locating a subject on the board sheet (agni issue 1325).
// It keys a part by ref-des and a net by NAME, since a board's copper carries no net id and an EDIF
// schematic shares none with an ODB++ job. It holds names only, so a large board's index is small
// next to the board it came from.
type boardIndex struct {
	comps map[string]bool // ref-des with a footprint
	pins  map[string]bool // pinKey(ref, pad number) for every pad
	nets  map[string]bool // net name with copper or a pad
}

func newBoardIndex(b *geom.BoardGeometry) *boardIndex {
	ix := &boardIndex{comps: map[string]bool{}, pins: map[string]bool{}, nets: map[string]bool{}}
	for _, pl := range b.GetPlacements() {
		if pl.GetRefDes() == "" {
			continue
		}
		ix.comps[pl.GetRefDes()] = true
		for _, pad := range pl.GetPads() {
			if pad.GetNumber() != "" {
				ix.pins[pinKey(pl.GetRefDes(), pad.GetNumber())] = true
			}
			if pad.GetNet() != "" {
				ix.nets[pad.GetNet()] = true
			}
		}
	}
	for _, nc := range b.GetNets() {
		if nc.GetNet() != "" && (len(nc.GetSegments()) > 0 || len(nc.GetVias()) > 0 || len(nc.GetShapes()) > 0) {
			ix.nets[nc.GetNet()] = true
		}
	}
	for _, z := range b.GetZones() {
		if z.GetNet() != "" {
			ix.nets[z.GetNet()] = true
		}
	}
	return ix
}

// draws reports whether the board draws s. A bus is never on a board.
func (ix *boardIndex) draws(s *checkspb.Subject) bool {
	if ix == nil {
		return false
	}
	switch s.GetKind() {
	case check.KindNet:
		return ix.nets[s.GetRef()]
	case check.KindPin:
		return ix.pins[pinKey(s.GetRef(), s.GetPin())]
	case check.KindComponent:
		return ix.comps[s.GetRef()]
	}
	return false
}

// boardIndexFor returns the index of the board read from uri, or nil when there is no board. The
// cache keeps the index rather than the board, so a request after the first one copies no board
// geometry. Without a cache it reads the board each time.
func boardIndexFor(ctx context.Context, loader interface {
	Board(context.Context, artifact.URI) (*geom.BoardGeometry, error)
}, uri artifact.URI) (*boardIndex, error) {
	build := func(ctx context.Context) (any, error) {
		b, err := loader.Board(ctx, uri)
		if err != nil || b == nil {
			return (*boardIndex)(nil), err
		}
		return newBoardIndex(b), nil
	}
	var v any
	var err error
	if c := cacheOf(loader); c != nil {
		v, err = c.get(ctx, cacheKey("boardindex", uri.String()), build)
	} else {
		v, err = build(ctx)
	}
	if err != nil {
		return nil, ClassifyLoadErr(err)
	}
	ix, _ := v.(*boardIndex)
	return ix, nil
}

// pinKey joins a ref-des and a pin designator. NUL cannot occur in either.
func pinKey(ref, pin string) string { return ref + "\x00" + pin }

// locateBoard is the board index an answer locates on. A board-only design always gets one, since
// its board is the only drawing it lists (agni issue 1466). Any other design gets runBoardIndex's,
// so a run that never read the board does not read it to badge a finding.
func locateBoard(ctx context.Context, loader interface {
	Board(context.Context, artifact.URI) (*geom.BoardGeometry, error)
}, snap Snap, m check.Model) *boardIndex {
	if !snap.BoardOnly() {
		return runBoardIndex(ctx, loader, snap.BoardRead(), m)
	}
	ix, err := boardIndexFor(ctx, loader, snap.BoardRead())
	if err != nil {
		return nil
	}
	return ix
}

// traceBoard is the board a trace or a walk locates on. Only a board-only design has one, since
// those answers otherwise name the schematic's sheets alone, and reading a large board for them
// would cost seconds a reader asking about a schematic never sees.
func traceBoard(ctx context.Context, loader interface {
	Board(context.Context, artifact.URI) (*geom.BoardGeometry, error)
}, snap Snap) *boardIndex {
	if !snap.BoardOnly() {
		return nil
	}
	return locateBoard(ctx, loader, snap, nil)
}

// runBoardIndex is the board index for a run whose model read the board, so its findings badge the
// board sheet (#142's WS7-034). A model without a board gets none, and so does a board that fails
// to index, since the run already answered and a missing badge is all that is lost.
func runBoardIndex(ctx context.Context, loader interface {
	Board(context.Context, artifact.URI) (*geom.BoardGeometry, error)
}, uri artifact.URI, m check.Model) *boardIndex {
	if m == nil || !m.HasBoard() {
		return nil
	}
	ix, err := boardIndexFor(ctx, loader, uri)
	if err != nil {
		return nil
	}
	return ix
}
