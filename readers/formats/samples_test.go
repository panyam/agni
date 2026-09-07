package formats

import (
	"os"
	"testing"
)

// The pinned sample board, fetched by `make samples` from github.com/panyam/agni-samples. It is a
// real 17-sheet carrier board (Antmicro's Jetson AGX Thor baseboard, Apache-2.0) rather than a
// fixture we authored, which is the point: every synthetic fixture in this repo was built to the
// reader's own assumptions, so none of them can catch an assumption that is wrong.
const sampleJetson = "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_sch"

// TestSampleBoardRead pins what the reader currently produces for a real hierarchical board.
//
// It is a CHARACTERIZATION test, so one of the three numbers below is knowingly wrong and is
// asserted anyway. KiCad resolves this board to 1387 nets and we produce 1729, because a net crossing
// a sheet boundary is read as two nets: `/inout_user/AN0` and `AN0` where KiCad has one `/AN0`.
// Asserting 1729 is what makes a fix VISIBLE, so whoever moves it changes the constant deliberately
// rather than discovering months later that a number drifted.
//
// The first half of issue 561 has landed and this number did not move, which is itself the finding.
// That fix follows a bus VECTOR (`AN[0..7]`) across a sheet boundary and clears the split entirely on
// the boards that use one. This board crosses with GROUP buses instead — `CAM0{CSI}`, whose members
// come from a `bus_alias` and are named `CAM0.CLK_N` — and those are still not followed. Which nets
// are still wrong, rather than how many, is in readers/kicad/testdata/oracle_corpus.baseline.
//
// The other two are correct today and guard against regression: the component count matches KiCad
// exactly, and the MPN count is what the datasheet tier joins on.
func TestSampleBoardRead(t *testing.T) {
	if _, err := os.Stat(sampleJetson); err != nil {
		// Deliberately fatal rather than skipped. A missing corpus must not read as a pass, which is
		// the failure shape docsite/content/build/the-gate.md exists to catalogue.
		t.Fatalf("sample corpus missing, run `make samples`: %v", err)
	}

	d, err := (&Loader{}).ReadDesign(sampleJetson)
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}

	if got, want := len(d.GetComponents()), 1123; got != want {
		t.Errorf("components = %d, want %d (KiCad resolves the same 1123 from the board file)", got, want)
	}

	if got, want := len(d.GetNets()), 1729; got != want {
		t.Errorf("nets = %d, want %d; KiCad resolves 1387, and the gap is the group-bus half of "+
			"issue 561. As that closes this constant should fall toward 1387", got, want)
	}

	var withMPN int
	for _, c := range d.GetComponents() {
		if c.GetMpn() != "" {
			withMPN++
		}
	}
	if got, want := withMPN, 1006; got != want {
		t.Errorf("components carrying an MPN = %d, want %d", got, want)
	}
}
