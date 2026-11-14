package service

import (
	"context"
	"sort"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// boardIndex says what a board draws, for locating a subject on the board sheet (agni issue 1325).
// It keys a part by ref-des and a net by NAME, since a board's copper carries no net id and an EDIF
// schematic shares none with an ODB++ job. It holds names only, so a large board's index is small
// next to the board it came from.
//
// It also keeps the side each part sits on and the copper layers each part and net use, so a board
// view can turn to a selection and show its layers (agni issue 1587).
type boardIndex struct {
	comps map[string]bool // ref-des with a footprint
	pins  map[string]bool // pinKey(ref, pad number) for every pad
	nets  map[string]bool // net name with copper or a pad

	side      map[string]string   // ref-des -> "front" or "back", absent when the board states neither
	compLayer map[string]string   // ref-des -> the copper layer it is placed on
	netLayers map[string][]string // net name -> its copper layers, in the board's layer order
}

func newBoardIndex(b *geom.BoardGeometry) *boardIndex {
	ix := &boardIndex{comps: map[string]bool{}, pins: map[string]bool{}, nets: map[string]bool{},
		side: map[string]string{}, compLayer: map[string]string{}}
	copper := copperLayers(b)
	netLayers := map[string]map[string]bool{}
	addLayer := func(net, layer string) {
		if net == "" || !copper[layer] {
			return
		}
		if netLayers[net] == nil {
			netLayers[net] = map[string]bool{}
		}
		netLayers[net][layer] = true
	}
	for _, pl := range b.GetPlacements() {
		if pl.GetRefDes() == "" {
			continue
		}
		ix.comps[pl.GetRefDes()] = true
		if side := placedSide(pl); side != "" {
			ix.side[pl.GetRefDes()] = side
		}
		if copper[pl.GetLayer()] {
			ix.compLayer[pl.GetRefDes()] = pl.GetLayer()
		}
		for _, pad := range pl.GetPads() {
			if pad.GetNumber() != "" {
				ix.pins[pinKey(pl.GetRefDes(), pad.GetNumber())] = true
			}
			if pad.GetNet() != "" {
				ix.nets[pad.GetNet()] = true
			}
			if singleLayerPad(pad) {
				for _, ly := range pad.GetLayers() {
					addLayer(pad.GetNet(), ly)
				}
			}
		}
	}
	for _, nc := range b.GetNets() {
		if nc.GetNet() != "" && (len(nc.GetSegments()) > 0 || len(nc.GetVias()) > 0 || len(nc.GetShapes()) > 0) {
			ix.nets[nc.GetNet()] = true
		}
		for _, s := range nc.GetSegments() {
			addLayer(nc.GetNet(), s.GetLayer())
		}
		for _, sh := range nc.GetShapes() {
			addLayer(nc.GetNet(), sh.GetLayer())
		}
	}
	for _, z := range b.GetZones() {
		if z.GetNet() != "" {
			ix.nets[z.GetNet()] = true
		}
		addLayer(z.GetNet(), z.GetLayer())
	}
	ix.netLayers = make(map[string][]string, len(netLayers))
	order := layerOrder(b)
	for net, set := range netLayers {
		ix.netLayers[net] = order(set)
	}
	return ix
}

// copperLayers names the board's copper layers. A layer is copper when a track, a copper shape, a
// zone or a placement names it, or when the layer table lists it as anything but a user layer. A
// pad also lists its mask and paste layers, and this set keeps those out of a net's layers.
func copperLayers(b *geom.BoardGeometry) map[string]bool {
	out := map[string]bool{}
	for _, l := range b.GetLayers() {
		if l.GetKind() != "" && l.GetKind() != "user" {
			out[l.GetName()] = true
		}
	}
	for _, nc := range b.GetNets() {
		for _, s := range nc.GetSegments() {
			out[s.GetLayer()] = true
		}
		for _, sh := range nc.GetShapes() {
			out[sh.GetLayer()] = true
		}
	}
	for _, z := range b.GetZones() {
		out[z.GetLayer()] = true
	}
	for _, pl := range b.GetPlacements() {
		out[pl.GetLayer()] = true
	}
	delete(out, "")
	return out
}

// layerOrder sorts a set of layer names by the board's layer table, and a name the table lacks
// after the rest, by name.
func layerOrder(b *geom.BoardGeometry) func(map[string]bool) []string {
	num := map[string]int{}
	for i, l := range b.GetLayers() {
		if _, seen := num[l.GetName()]; !seen {
			num[l.GetName()] = i
		}
	}
	return func(set map[string]bool) []string {
		out := make([]string, 0, len(set))
		for ly := range set {
			out = append(out, ly)
		}
		sort.Slice(out, func(i, j int) bool {
			ni, iOK := num[out[i]]
			nj, jOK := num[out[j]]
			if iOK != jOK {
				return iOK
			}
			if ni != nj {
				return ni < nj
			}
			return out[i] < out[j]
		})
		return out
	}
}

// placedSide is the side a part sits on. A mirrored part is on the back, as the renderer draws it.
// Otherwise its layer says, and a layer other than F.Cu or B.Cu states no side.
func placedSide(pl *geom.ComponentPlacement) string {
	switch {
	case pl.GetMirror() || pl.GetLayer() == "B.Cu":
		return "back"
	case pl.GetLayer() == "F.Cu":
		return "front"
	}
	return ""
}

// singleLayerPad reports whether a pad sits on its listed layers alone. A wildcard such as "*.Cu"
// spans the stack, as a through-hole pad does, and so does a pad on both outer layers.
func singleLayerPad(pad *geom.Pad) bool {
	front, back := false, false
	for _, l := range pad.GetLayers() {
		switch {
		case strings.HasPrefix(l, "*."):
			return false
		case l == "F.Cu":
			front = true
		case l == "B.Cu":
			back = true
		}
	}
	return !(front && back)
}

// sideOf is where the board places s, "front", "back" or "" for a net or a part with no side.
func (ix *boardIndex) sideOf(s *checkspb.Subject) string {
	if ix == nil || !ix.draws(s) {
		return ""
	}
	switch s.GetKind() {
	case check.KindPin, check.KindComponent:
		return ix.side[s.GetRef()]
	}
	return ""
}

// layersOf names the copper layers that carry s on the board.
func (ix *boardIndex) layersOf(s *checkspb.Subject) []string {
	if ix == nil || !ix.draws(s) {
		return nil
	}
	switch s.GetKind() {
	case check.KindNet:
		return append([]string(nil), ix.netLayers[s.GetRef()]...)
	case check.KindPin, check.KindComponent:
		if ly := ix.compLayer[s.GetRef()]; ly != "" {
			return []string{ly}
		}
	}
	return nil
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
