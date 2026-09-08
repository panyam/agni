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
// are still wrong, rather than how many, is in readers/kicad/oracle_corpus.baseline.
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

const sampleJetsonBoard = "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb"

// TestSampleBoardPartIdentityAgreesAcrossViews asserts the two views of one design resolve the same
// part numbers. It is a stronger claim than either view's count, because the schematic and the board
// state the MPN in different places and a reader can satisfy a count while joining on nothing.
//
// The board file is in the oracle corpus rather than the tutorial tarball, so this needs
// `make samples-oracle`.
func TestSampleBoardPartIdentityAgreesAcrossViews(t *testing.T) {
	if _, err := os.Stat(sampleJetsonBoard); err != nil {
		t.Skipf("oracle corpus not fetched, run `make samples-oracle`: %v", err)
	}

	mpns := func(path string) map[string]string {
		t.Helper()
		d, err := (&Loader{}).ReadDesign(path)
		if err != nil {
			t.Fatalf("ReadDesign %s: %v", path, err)
		}
		got := map[string]string{}
		for _, c := range d.GetComponents() {
			if m := c.GetMpn(); m != "" {
				got[c.GetRefDes()] = m
			}
		}
		return got
	}

	fromSch, fromPCB := mpns(sampleJetson), mpns(sampleJetsonBoard)
	if len(fromPCB) == 0 {
		t.Fatalf("the board file states an MPN on every footprint and the read resolved none; "+
			"the schematic view of the same design resolves %d", len(fromSch))
	}

	var missing, differ int
	for ref, want := range fromSch {
		switch got, ok := fromPCB[ref]; {
		case !ok:
			missing++
		case got != want:
			differ++
		}
	}
	if missing != 0 || differ != 0 {
		t.Errorf("views disagree on part identity: %d ref_des present in the schematic and absent "+
			"from the board, %d carrying a different MPN (schematic %d, board %d)",
			missing, differ, len(fromSch), len(fromPCB))
	}
}
