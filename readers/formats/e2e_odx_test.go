package formats

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestXMLChoosesItsReaderByRoot reads three .xml files whose root elements differ. IPC-2581 and ODX
// each reach their own reader, for the netlist and the board alike, and any other root is refused
// with its name (agni issue 1372).
func TestXMLChoosesItsReaderByRoot(t *testing.T) {
	l := &Loader{}
	for path, want := range map[string]string{
		"../ipc2581/testdata/board.xml":                   "ipc-2581",
		"../odb/testdata/odx/two-sided.xml":               "odb++",
		"../../examples/common/designs/mixer.ipc2581.xml": "ipc-2581",
	} {
		d, err := l.ReadDesign(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if d.GetSourceFormat() != want {
			t.Errorf("%s read as %q, want %q", path, d.GetSourceFormat(), want)
		}
		if _, err := l.BoardGeometry(path); err != nil {
			t.Errorf("%s board: %v", path, err)
		}
	}

	prefixed := filepath.Join(t.TempDir(), "prefixed.xml")
	ipc, err := os.ReadFile("../ipc2581/testdata/board.xml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prefixed, []byte(strings.Replace(strings.Replace(string(ipc), "<IPC-2581", "<ns:IPC-2581", 1), "</IPC-2581>", "</ns:IPC-2581>", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sniffXML(bufio.NewReader(bytes.NewReader(mustRead(t, prefixed))), prefixed); err != nil {
		t.Errorf("a namespace-prefixed IPC-2581 root: %v", err)
	}

	other := filepath.Join(t.TempDir(), "drawing.xml")
	if err := os.WriteFile(other, []byte(`<?xml version="1.0"?>`+"\n<!-- not a board -->\n<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ReadDesign(other); err == nil || !strings.Contains(err.Error(), "root element <svg>") {
		t.Errorf("ReadDesign(<svg>) = %v, want a refusal naming the root", err)
	}
	if _, err := l.BoardGeometry(other); err == nil || !strings.Contains(err.Error(), "root element <svg>") {
		t.Errorf("BoardGeometry(<svg>) = %v, want a refusal naming the root", err)
	}
}

// parasdr is the Parallella parasdr board, which Allegro SPB 16.6 exported twice, as IPC-2581 and
// as ODX. The cadence-exports samples artifact carries both (CC BY-SA 3.0 US, agni-samples
// PROVENANCE.md).
const parasdr = "../../tools/samples/cadence/parallella-parasdr/"

// mechPins are the five mechanical pins the two exports spell differently. IPC-2581 calls them
// MECH1 and MECH2, and ODX numbers them 1, 15 and 16, the number its netlist and pads use. Both put
// each one on no net and at the same point.
var mechPins = map[string]string{
	"E97.MECH1": "E97.1", "E98.MECH1": "E98.1", "E99.MECH1": "E99.1",
	"X3.MECH1": "X3.15", "X3.MECH2": "X3.16",
}

// TestODXReadsAsItsIPC2581Twin holds parasdr's ODX read to its IPC-2581 read. Every pin is on the
// same net, and every pad sits at the same point on the same side. Each ODX pad is on its pin's net,
// and its copper is on nets the netlist has. TestParasdrsCopperAgreesAcrossItsReads holds the copper
// of both reads to the board itself.
//
// IPC-2581 writes an unconnected pin as a net of its own (`Unused_05CE8640`), and ODX puts it on
// `$NONE$`. Both readers read either as no net (agni 1383).
func TestODXReadsAsItsIPC2581Twin(t *testing.T) {
	ipcPath, odxPath := parasdr+"parasdr_ipc2581.xml", parasdr+"parasdr_odb++.xml"
	if _, err := os.Stat(odxPath); err != nil {
		t.Fatalf("the parasdr exports are missing; run `make samples-oracle`: %v", err)
	}
	l := &Loader{}
	id, err := l.ReadDesign(ipcPath)
	if err != nil {
		t.Fatal(err)
	}
	od, err := l.ReadDesign(odxPath)
	if err != nil {
		t.Fatal(err)
	}

	ipcPins, odxPins := pinNets(id), pinNets(od)
	// A count guard, so a read that lost the netlist cannot pass by comparing nothing with nothing.
	if len(ipcPins) != 432 || len(odxPins) != 432 {
		t.Fatalf("pins on nets: IPC-2581 %d, ODX %d, want 432 each", len(ipcPins), len(odxPins))
	}
	if diffs := diffRows("pin nets", ipcPins, odxPins); len(diffs) > 0 {
		t.Errorf("%d pin differences:\n  %s", len(diffs), capLines(diffs))
	}

	ig, err := l.BoardGeometry(ipcPath)
	if err != nil {
		t.Fatal(err)
	}
	og, err := l.BoardGeometry(odxPath)
	if err != nil {
		t.Fatal(err)
	}
	// A pad is compared by its pin, its side and where it composes to.
	padsAt := func(items []placed, rename map[string]string) []placed {
		var out []placed
		for _, p := range items {
			f := strings.Fields(p.key)
			pin := f[0]
			if r, ok := rename[pin]; ok {
				pin = r
			}
			out = append(out, placed{key: pin + " " + f[len(f)-1], pts: p.pts})
		}
		return out
	}
	ipcPads, odxPads := padsAt(padItems(ig), mechPins), padsAt(padItems(og), nil)
	if len(ipcPads) != 505 || len(odxPads) != 505 {
		t.Fatalf("pads: IPC-2581 %d, ODX %d, want 505 each", len(ipcPads), len(odxPads))
	}
	if diffs := diffPlaced("pads", ipcPads, odxPads); len(diffs) > 0 {
		t.Errorf("%d pad differences:\n  %s", len(diffs), capLines(diffs))
	}

	// Each pad's net is its pin's net in the netlist, and each piece of copper is on a net the
	// netlist has.
	pinNet := map[string]string{}
	nets := map[string]bool{}
	for _, row := range odxPins {
		pin, net, _ := strings.Cut(row, "=")
		pinNet[pin], nets[net] = net, true
	}
	var wrong []string
	for _, pl := range og.GetPlacements() {
		for _, p := range pl.GetPads() {
			if pin := pl.GetRefDes() + "." + p.GetNumber(); p.GetNet() != pinNet[pin] {
				wrong = append(wrong, pin+" pad on "+p.GetNet()+", pin on "+pinNet[pin])
			}
		}
	}
	vias, segs := 0, 0
	for _, nc := range og.GetNets() {
		// Unnamed copper is a pour on no net, which carries no track and no via.
		if nc.GetNet() == "" && len(nc.GetSegments()) == 0 && len(nc.GetVias()) == 0 {
			continue
		}
		if !nets[nc.GetNet()] {
			wrong = append(wrong, "copper on "+nc.GetNet()+", which the netlist lacks")
		}
		vias += len(nc.GetVias())
		segs += len(nc.GetSegments())
		for _, v := range nc.GetVias() {
			if v.GetDrill() == 0 || v.GetSize() <= v.GetDrill() {
				wrong = append(wrong, fmt.Sprintf("via on %s with land %d and drill %d", nc.GetNet(), v.GetSize(), v.GetDrill()))
			}
		}
	}
	sort.Strings(wrong)
	if len(wrong) > 0 {
		t.Errorf("%d board differences from the netlist:\n  %s", len(wrong), capLines(wrong))
	}
	if vias != 420 || segs == 0 {
		t.Errorf("ODX copper holds %d vias and %d segments, want 420 vias and some track", vias, segs)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
