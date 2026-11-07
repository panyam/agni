package formats

import (
	"fmt"
	"sort"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// parasdrRead is one of the three files Allegro wrote for parasdr.
type parasdrRead struct{ name, file string }

var parasdrExports = []parasdrRead{
	{"IPC-2581", "parasdr_ipc2581.xml"},
	{"ODB++X", "parasdr_odb++.xml"},
}

// trackRows lists a read's track segments as "<layer> <net> w=<µm>" with a count of each, so two
// reads compare by how much track of each width every net has on every layer. A segment's ends are
// left out, since an export may split or join a route at a bend.
func trackRows(g *geom.BoardGeometry) []string {
	var out []string
	for _, nc := range g.GetNets() {
		for _, s := range nc.GetSegments() {
			out = append(out, fmt.Sprintf("%s %q w=%d", s.GetLayer(), nc.GetNet(), (s.GetWidth()+500)/1000))
		}
	}
	sort.Strings(out)
	return out
}

// viaRows lists a read's vias as "<net> at <µm>,<µm> drill=<µm> <from>-<to>".
func viaRows(g *geom.BoardGeometry) []string {
	um := func(v int64) int64 { return (v + 500) / 1000 }
	var out []string
	for _, nc := range g.GetNets() {
		for _, v := range nc.GetVias() {
			out = append(out, fmt.Sprintf("%q at %d,%d drill=%d %s-%s", nc.GetNet(), um(v.GetAt().GetX()), um(v.GetAt().GetY()), um(v.GetDrill()), v.GetLayerFrom(), v.GetLayerTo()))
		}
	}
	sort.Strings(out)
	return out
}

// pinCopperRows lists each pad as "<ref>.<pin> net=<net> drill=<µm>". The three reads number the
// mechanical pins three ways (mechPins, and none at all on the board), so those are listed by their
// part alone.
func pinCopperRows(g *geom.BoardGeometry) []string {
	mech := map[string]bool{}
	for k, v := range mechPins {
		mech[k], mech[v] = true, true
	}
	var out []string
	for _, pl := range g.GetPlacements() {
		for _, p := range pl.GetPads() {
			pin := pl.GetRefDes() + "." + p.GetNumber()
			if mech[pin] || p.GetNumber() == "" {
				pin = pl.GetRefDes() + ".mech"
			}
			out = append(out, fmt.Sprintf("%s net=%q drill=%d", pin, p.GetNet(), (p.GetDrill()+500)/1000))
		}
	}
	sort.Strings(out)
	return out
}

// TestParasdrsCopperAgreesAcrossItsReads holds the routed copper of parasdr's two exports to the
// board Allegro wrote them from (agni 1383). Every net carries the same track of the same width on
// the same layer, every via sits at the same point with the same drill and span, and every pad is
// on the same net with the same drill. Before 1383 the IPC-2581 read gave every track width 0, no
// via and no pad a net or a drill, and the ODB++X read dropped a third of the track.
func TestParasdrsCopperAgreesAcrossItsReads(t *testing.T) {
	l := &Loader{}
	board, err := l.BoardGeometry(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	tracks, vias, pads := trackRows(board), viaRows(board), pinCopperRows(board)
	// A count guard, so a board read that lost its copper cannot pass by comparing nothing.
	if len(tracks) != 4747 || len(vias) != 420 || len(pads) != 505 {
		t.Fatalf("the board reads %d segments, %d vias and %d pads, want 4747, 420 and 505", len(tracks), len(vias), len(pads))
	}
	for _, export := range parasdrExports {
		g, err := l.BoardGeometry(parasdr + export.file)
		if err != nil {
			t.Fatalf("%s: %v", export.name, err)
		}
		for _, c := range []struct {
			what       string
			board, got []string
		}{
			{"tracks", tracks, trackRows(g)},
			{"vias", vias, viaRows(g)},
			{"pads", pads, pinCopperRows(g)},
		} {
			if diffs := diffRows(c.what, c.board, c.got); len(diffs) > 0 {
				t.Errorf("%s %s: %d differences from the board:\n  %s", export.name, c.what, len(diffs), capLines(diffs))
			}
		}
	}
}
