package kicad

import (
	"bytes"
	"os"
	"testing"

	"github.com/panyam/agni/internal/sexpr"
)

// The parse leaves out a footprint graphic on a layer that no reader draws, and keeps one on a drawn
// layer (agni issue 1005). TestBoardReadsIgnoreZoneFills checks that the board reads give the same
// answer with and without the skip. This test checks that the skip actually drops lines.
func TestBoardParseDropsUndrawnFootprintGraphics(t *testing.T) {
	const p = "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb"
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%s: %v (make samples-oracle fetches the boards)", p, err)
	}
	full, err := sexpr.Parse(bytes.NewReader(b), sexpr.KiCadStrings)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := parseBoard(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	count := func(root *node) (drawn, undrawn int) {
		for _, fp := range root.Children("footprint") {
			for _, base := range graphicHeads {
				for _, n := range fp.Children("fp_" + base) {
					if footprintLayerDrawn(atomOf(n.Child("layer").Arg(1))) {
						drawn++
					} else {
						undrawn++
					}
				}
			}
		}
		return drawn, undrawn
	}
	fullDrawn, fullUndrawn := count(full)
	skipDrawn, skipUndrawn := count(skipped)
	if fullUndrawn < 100000 {
		t.Fatalf("the full parse holds %d undrawn footprint graphics, so the skip was barely exercised", fullUndrawn)
	}
	if skipUndrawn != 0 {
		t.Errorf("the board parse kept %d footprint graphics on undrawn layers, want 0", skipUndrawn)
	}
	if skipDrawn != fullDrawn {
		t.Errorf("the board parse kept %d drawn footprint graphics, the full parse %d", skipDrawn, fullDrawn)
	}
}
