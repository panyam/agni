package kicad

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/sexpr"
	"google.golang.org/protobuf/proto"
)

// TestBoardReadsIgnoreZoneFills holds the board reads to the same answer whether a zone's fill is
// parsed or skipped (agni issue 946), over the sample boards that carry fills and a small board whose
// fill holds a parenthesis inside a string.
func TestBoardReadsIgnoreZoneFills(t *testing.T) {
	boards := map[string][]byte{
		"inline": []byte(`(kicad_pcb (version 20240108) (generator "pcbnew")
  (net 0 "") (net 1 "GND")
  (footprint "R_0402" (layer "F.Cu") (at 10 10) (property "Reference" "R1")
    (pad "1" smd rect (at 0 0) (size 1 1) (layers "F.Cu") (net 1 "GND")))
  (zone (net 1) (net_name "GND") (layer "F.Cu")
    (polygon (pts (xy 0 0) (xy 20 0) (xy 20 20)))
    (filled_polygon (layer "F.Cu") (island) (pts (xy 1 1) (xy 19 1) (xy 19 19)) (note "a) b(")))
  (segment (start 0 0) (end 5 5) (width 0.2) (layer "F.Cu") (net 1)))`),
	}
	for _, p := range []string{
		"../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb",
		"../../tools/samples/boards/royalblue54L-feather/RoyalBlue54L-Feather.kicad_pcb",
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v (make samples-oracle fetches the boards)", p, err)
		}
		boards[p] = b
	}
	fills := 0
	for name, b := range boards {
		full, errFull := sexpr.Parse(bytes.NewReader(b), sexpr.KiCadStrings)
		skipped, errSkip := parseBoard(bytes.NewReader(b))
		if (errFull == nil) != (errSkip == nil) {
			t.Errorf("%s: parsing in full errs %v, skipping fills errs %v", name, errFull, errSkip)
			continue
		}
		if errFull != nil {
			continue // a board that does not parse fails both ways alike
		}
		var all []*node
		sexpr.Collect(full, "filled_polygon", &all)
		fills += len(all)
		if !proto.Equal(extractBoardGeometry(full, name), extractBoardGeometry(skipped, name)) {
			t.Errorf("%s: the board geometry differs when zone fills are skipped", name)
		}
		if !proto.Equal(extractPCB(full, name), extractPCB(skipped, name)) {
			t.Errorf("%s: the board read as a design differs when zone fills are skipped", name)
		}
		if strings.HasPrefix(name, "inline") && len(all) != 1 {
			t.Errorf("the inline board parses %d fills in full, want 1", len(all))
		}
	}
	// Positive control: the boards compared carry fills, so the equality is not over trees that never
	// held any.
	if fills < 100 {
		t.Fatalf("only %d zone fills across the boards compared, so skipping them was barely exercised", fills)
	}
}
