package render

import (
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/ipc2581"
)

// TestBoardIPCPlacementLandsInFrame is the WS1-029 behavioral gate: an IPC-2581 placement pad,
// composed through the real renderer rule (padWorld), must land at its true board position.
// Before the fix, top parts rotated the wrong way and bottom parts were not mirrored, so an
// asymmetric rotated connector flew off the board (testcase1 CN11 sat ~0.8" past the edge). The
// fixture places one asymmetric 3-pin part on each side, both rotated 90 deg; expected world
// coordinates are hand-derived (top: R(+90)*pad; bottom: R(-90)*mirrorX(pad)) as an independent
// oracle, so this checks the composition end to end rather than the reader's internal convention.
func TestBoardIPCPlacementLandsInFrame(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "readers", "ipc2581", "testdata", "board_geom_rotation.xml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := ipc2581.ReadBoardGeometry(f, "board_geom_rotation.xml")
	if err != nil {
		t.Fatal(err)
	}

	const mm = 1_000_000 // nm per mm, the fixture's unit
	want := map[string]map[string][2]int64{
		"AT": { // top, rot 90 at (5,4): R(+90) applied to (1,0),(0,2),(0,0)
			"1": {5 * mm, 5 * mm},
			"2": {3 * mm, 4 * mm},
			"3": {5 * mm, 4 * mm},
		},
		"AB": { // bottom, rot 90 at (2,2): R(-90) applied to mirrorX of (1,0),(0,2),(0,0)
			"1": {2 * mm, 3 * mm},
			"2": {4 * mm, 2 * mm},
			"3": {2 * mm, 2 * mm},
		},
	}
	placement := func(ref string) *geom.ComponentPlacement {
		for _, p := range g.GetPlacements() {
			if p.GetRefDes() == ref {
				return p
			}
		}
		return nil
	}
	for ref, pins := range want {
		pl := placement(ref)
		if pl == nil {
			t.Fatalf("placement %q not found", ref)
		}
		for _, pad := range pl.GetPads() {
			exp, ok := pins[pad.GetNumber()]
			if !ok {
				continue
			}
			gx, gy := padWorld(pl, pad)
			if abs64(gx-exp[0]) > 2 || abs64(gy-exp[1]) > 2 {
				t.Errorf("%s pin %s world = (%d,%d), want (%d,%d)", ref, pad.GetNumber(), gx, gy, exp[0], exp[1])
			}
		}
	}
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
