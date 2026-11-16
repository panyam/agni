package formats

import (
	"testing"
)

// heightTol is how far a part's height may differ between a .PcbDoc and its ODB++ export, in
// nanometres. Altium's export writes .comp_height in inches to four places, 2.54 µm.
const heightTol = 2_600

// TestAltiumHeightsMatchItsODBExport holds each foxBMS part's height, read from the .PcbDoc, to the
// .comp_height Altium wrote for it in the ODB++ job it exported from the same board (agni 1643). The
// export writes the component record's HEIGHT, and 0 for a part whose HEIGHT is 0. agni gives such
// a part its tallest 3D body's height where it has one, so a 0 in the job allows no height or a
// body's.
func TestAltiumHeightsMatchItsODBExport(t *testing.T) {
	held, fromBodies := 0, 0
	for _, pr := range altiumPairs(t) {
		t.Run(pr.name, func(t *testing.T) {
			_, pg := readBoth(t, pr.pcb)
			_, og := readBoth(t, pr.odb)
			want := map[string]int64{}
			for _, pl := range og.GetPlacements() {
				if pl.Height != nil {
					want[pl.GetRefDes()] = pl.GetHeight()
				}
			}
			compared := 0
			for _, pl := range pg.GetPlacements() {
				w, ok := want[pl.GetRefDes()]
				if !ok {
					continue
				}
				compared++
				switch {
				case w == 0 && pl.Height != nil:
					fromBodies++
				case w == 0:
				case pl.Height == nil || abs(pl.GetHeight()-w) > heightTol:
					t.Errorf("%s: %d nm tall (stated %v), the job says %d", pl.GetRefDes(), pl.GetHeight(), pl.Height != nil, w)
				default:
					held++
				}
			}
			if compared < len(want)*9/10 || compared < 50 {
				t.Errorf("compared %d parts of the job's %d with a height", compared, len(want))
			}
		})
	}
	if held < 1000 {
		t.Errorf("%d parts held to a height, want at least 1000 across foxBMS", held)
	}
	t.Logf("%d parts held to the job's height, %d the job gives 0 took a 3D body's", held, fromBodies)
}
