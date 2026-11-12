package formats

import (
	"os"
	"regexp"
	"testing"

	"github.com/panyam/agni/core/classify"
)

// The pinned sample board, fetched by `make samples` from github.com/panyam/agni-samples. It is a
// real 17-sheet carrier board (Antmicro's Jetson AGX Thor baseboard, Apache-2.0) rather than a
// fixture we authored. Every synthetic fixture in this repo was built to the reader's own
// assumptions, so none of them can catch an assumption that is wrong.
const sampleJetson = "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_sch"

// TestSampleBoardRead pins what the reader currently produces for a real hierarchical board.
//
// It is a CHARACTERIZATION test, so one of the three numbers below is knowingly wrong and is
// asserted anyway. KiCad resolves this board to 1387 nets and we produce 1355, because we still merge
// some nets that KiCad keeps apart (below). Asserting the wrong number makes the next fix VISIBLE.
// Whoever moves it changes the constant deliberately, rather than discovering months later that a
// number drifted.
//
// It has moved twice from 1729. Issue 577, the pin swap on mirrored symbols, took it to 1606. This
// board places a lot of mirrored symbols, and each swap moved one connection out of a net and another
// in. Issue 597 then taught the walk to follow a GROUP bus across a sheet boundary. This board spells
// its buses that way, as `CAM0{CSI}`. The members come from a `bus_alias` and are named
// `CAM0.CSI2_CLK+`. Issue 561's first half, the bus VECTOR (`AN[0..7]`), merged before either and
// did not move this number at all. So the two halves are separate issues.
//
// It is now BELOW KiCad's 1387. Falling further is the WRONG direction, and the obvious reading of
// this constant is backwards. Every remaining disagreement is an over-merge. There are ten of them,
// and all ten predate both fixes. KiCad gives each unconnected pin its own
// `unconnected-(U65-GL-Pad10)` net. We group a part's unconnected pins into one `N$nnn`. Ten of ours
// stand in for about thirty of KiCad's, and that accounts for the whole gap. Closing it pushes this
// number UP toward 1387.
//
// readers/kicad/oracle_corpus.baseline lists which nets are still wrong, not just how many. Read that
// file. It separates the two directions, and the split half is now empty.
//
// The other two numbers are correct today and guard against regression. The component count matches
// KiCad exactly. The datasheet tier joins on MPNs, so the MPN count guards that join.
func TestSampleBoardRead(t *testing.T) {
	if _, err := os.Stat(sampleJetson); err != nil {
		// Deliberately fatal rather than skipped. A missing corpus must not read as a pass.
		// docsite/content/build/the-gate.md catalogues that failure shape.
		t.Fatalf("sample corpus missing, run `make samples`: %v", err)
	}

	d, err := (&Loader{}).ReadDesign(sampleJetson)
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}

	if got, want := len(d.GetComponents()), 1123; got != want {
		t.Errorf("components = %d, want %d (KiCad resolves the same 1123 from the board file)", got, want)
	}

	if got, want := len(d.GetNets()), 1355; got != want {
		t.Errorf("nets = %d, want %d; KiCad resolves 1387 and we are UNDER it, because we group a "+
			"part's unconnected pins where KiCad names each one. Closing that raises this number", got, want)
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
// part numbers. That claim is stronger than either view's count. The schematic and the board state
// the MPN in different places, and a reader can satisfy a count while joining on nothing.
//
// The board file is in the oracle corpus rather than the tutorial tarball. So testall depends on
// `samples-oracle` rather than on `samples`.
func TestSampleBoardPartIdentityAgreesAcrossViews(t *testing.T) {
	if _, err := os.Stat(sampleJetsonBoard); err != nil {
		// Fatal rather than skipped, as its neighbour above. testall depends on `make
		// samples-oracle`, so an absent corpus is a broken checkout rather than a normal state. This
		// test once had a skip here, and spent its first hours not running at all (agni issue 591).
		t.Fatalf("oracle corpus missing, run `make samples-oracle`: %v", err)
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

// TestSampleBoardPartIdentitySurvivesEmit requires a board file's part numbers to reach an EDIF
// export. A design leaves for a toolchain that is not ours through an export. Such a tool joins on
// a part-number column among the first things it does. An export with correct connectivity and no
// part numbers hands the recipient a netlist that silently fails a BOM join.
//
// It reads the BOARD view. The schematic view exercised the same path already, and the board view
// lost the value. The reader recorded the part number on the component, and the writer emits a
// SECTION's attributes. So the value sat in the IR one field away from the half the writer reads
// (agni issue 584).
func TestSampleBoardPartIdentitySurvivesEmit(t *testing.T) {
	if _, err := os.Stat(sampleJetsonBoard); err != nil {
		t.Fatalf("oracle corpus missing, run `make samples-oracle`: %v", err)
	}
	src, err := (&Loader{}).ReadDesign(sampleJetsonBoard)
	if err != nil {
		t.Fatalf("ReadDesign: %v", err)
	}
	want := map[string]string{}
	for _, c := range src.GetComponents() {
		if m := c.GetMpn(); m != "" {
			want[c.GetRefDes()] = m
		}
	}
	if len(want) == 0 {
		t.Fatal("the board states an MPN on every footprint and the read resolved none")
	}

	out := roundTripEDIF(t, src, sampleJetsonBoard)
	classify.StampMPN(out)
	var lost int
	for _, c := range out.GetComponents() {
		if want[c.GetRefDes()] != "" && c.GetMpn() != want[c.GetRefDes()] {
			lost++
		}
	}
	if lost != 0 {
		t.Errorf("%d of %d part number(s) lost through the EDIF emit", lost, len(want))
	}
}

// TestSampleBoardArcTracksReadAsArcs holds each sample board's arc tracks to its file: every
// top-level (arc ...) is one track segment with a mid point, so a curved trace is never split into
// chords and never dropped (agni issue 1358). The counts come from the files themselves.
func TestSampleBoardArcTracksReadAsArcs(t *testing.T) {
	arcNode := regexp.MustCompile(`(?m)^\s*\(arc(\s*$|\s+\()`)
	for _, name := range []string{
		"jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb",
		"mnt-reform-motherboard30/reform2-motherboard30.kicad_pcb",
		"cynthion/cynthion.kicad_pcb",
	} {
		path := "../../tools/samples/boards/" + name
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("oracle corpus missing, run `make samples-oracle`: %v", err)
		}
		want := len(arcNode.FindAll(src, -1))
		g, err := (&Loader{}).BoardGeometry(path)
		if err != nil {
			t.Fatal(err)
		}
		got := 0
		for _, nc := range g.GetNets() {
			for _, s := range nc.GetSegments() {
				if s.GetMid() != nil {
					got++
				}
			}
		}
		if want < 100 || got != want {
			t.Errorf("%s: read %d arc tracks, the file has %d", name, got, want)
		}
	}
}
