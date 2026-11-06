package kicad

import (
	"os"
	"strconv"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestAKiCad9PadstackGivesEachLayerItsLand holds board_padstack.kicad_pcb's per-layer padstacks to
// what KiCad draws (agni 1407). The sizes below are kicad-cli 10.0.6's, read off
//
//	kicad-cli pcb export svg --layers <layer> --exclude-drawing-sheet --page-size-mode 2 board_padstack.kicad_pcb
//
// for each copper layer. Pad 1 is front_inner_back, with one inner shape for both inner layers.
// Pad 2 is custom, with a shape per layer, a square on In2.Cu among them.
func TestAKiCad9PadstackGivesEachLayerItsLand(t *testing.T) {
	f, err := os.Open("testdata/board_padstack.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := ReadBoardGeometry(f, "board_padstack.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	stacks := map[string]*geom.Padstack{}
	for _, ps := range g.GetPadstacks() {
		stacks[ps.GetId()] = ps
	}
	want := map[string]map[string]string{
		"1": {"F.Cu": "circle 2000000", "In1.Cu": "circle 1400000", "In2.Cu": "circle 1400000", "B.Cu": "circle 2400000"},
		"2": {"F.Cu": "circle 2000000", "In1.Cu": "circle 1200000", "In2.Cu": "rect 1600000", "B.Cu": "circle 2200000"},
	}
	pads := 0
	for _, pl := range g.GetPlacements() {
		for _, pad := range pl.GetPads() {
			w, ok := want[pad.GetNumber()]
			if !ok {
				continue
			}
			pads++
			ps := stacks[pad.GetPadstack()]
			if ps == nil {
				t.Errorf("pad %s names padstack %q, which the board lacks", pad.GetNumber(), pad.GetPadstack())
				continue
			}
			got := map[string]string{}
			for _, l := range ps.GetLayers() {
				got[l.GetLayer()] = l.GetPad().GetShape() + " " + strconv.FormatInt(l.GetPad().GetSize().GetX(), 10)
			}
			for layer, land := range w {
				if got[layer] != land {
					t.Errorf("pad %s on %s: %q, KiCad draws %q", pad.GetNumber(), layer, got[layer], land)
				}
			}
		}
	}
	if pads != 2 {
		t.Fatalf("found %d of the two pads", pads)
	}
}
