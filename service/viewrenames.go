package service

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/diff"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// viewRenames maps a board net's name to the name of the schematic net with exactly its pins, for
// each net the two views name differently (agni issue 1590). Imagine an Allegro board, which
// upper-cases every net name. A reader clicks "usb_dp" on the schematic, and the board draws it as
// "USB_DP". The cross-probe joins nets by name, so without this the board would light nothing.
//
// The pairing is diff.Views's, so the cross-probe and the agreement count agree on which nets are
// one net. A design whose netlist is its board, a design with no board, and a design whose netlist
// fails to read map nothing. b is the board when the caller has it in hand. Without it the board is read. The map
// is kept in the cache, keyed on the netlist, the board and the read options, and the caller must
// not change it.
func viewRenames(ctx context.Context, loader DesignLoader, snap Snap, b *geom.BoardGeometry) map[string]string {
	if !snap.twoViews() {
		return nil
	}
	opts := snap.Overlay.ReadOptions()
	build := func(ctx context.Context) (any, error) {
		d, err := loader.Design(ctx, snap.Netlist, opts...)
		if err != nil {
			return map[string]string(nil), nil
		}
		board := b
		if board == nil {
			bl, ok := loader.(boardReader)
			if !ok {
				return map[string]string(nil), nil
			}
			if board, err = bl.Board(ctx, snap.BoardRead()); err != nil || board == nil {
				return map[string]string(nil), nil
			}
		}
		return renamesOf(d, board), nil
	}
	var v any
	if l, ok := loader.(*CachingLoader); ok {
		if ok, key := l.key("viewrenames", cacheKey(snap.Netlist.String(), snap.BoardRead().String()), opts); ok {
			v, _ = l.cache.get(ctx, key, build)
		}
	}
	if v == nil {
		v, _ = build(ctx)
	}
	m, _ := v.(map[string]string)
	return m
}

// boardReader is a loader that reads boards. Every host's loader is one, and a test loader may not
// be.
type boardReader interface {
	Board(context.Context, artifact.URI) (*geom.BoardGeometry, error)
}

func renamesOf(d *ir.Design, b *geom.BoardGeometry) map[string]string {
	m := diff.Views(d, b).BoardRenames()
	if len(m) == 0 {
		return nil
	}
	return m
}

// renamedBoard is b with each net in renames under its schematic name. It copies the board only
// when there is a net to rename, since the cache shares b with every request.
func renamedBoard(b *geom.BoardGeometry, renames map[string]string) *geom.BoardGeometry {
	if len(renames) == 0 || b == nil {
		return b
	}
	out := proto.Clone(b).(*geom.BoardGeometry)
	name := func(n string) string {
		if to, ok := renames[n]; ok {
			return to
		}
		return n
	}
	for _, pl := range out.GetPlacements() {
		for _, pad := range pl.GetPads() {
			pad.Net = name(pad.GetNet())
		}
	}
	for _, nc := range out.GetNets() {
		nc.Net = name(nc.GetNet())
	}
	for _, z := range out.GetZones() {
		z.Net = name(z.GetNet())
	}
	return out
}

// schematicToBoard inverts renames, so a schematic net's name finds the board's.
func schematicToBoard(renames map[string]string) map[string]string {
	if len(renames) == 0 {
		return nil
	}
	m := make(map[string]string, len(renames))
	for board, sch := range renames {
		m[sch] = board
	}
	return m
}

// twoViews reports whether the snapshot reads its netlist and its board from two files, so there
// are two views to compare. A design with no board has one, and so does a board read as its own
// netlist, such as an ODB++ job opened alone.
func (s Snap) twoViews() bool {
	return !s.Board.IsZero() && s.Board.String() != s.Netlist.String() && formats.HasBoard(s.Board.Path)
}
