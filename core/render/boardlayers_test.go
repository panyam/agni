package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/kicad"
)

func innerBoard(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "readers", "kicad", "testdata", "board_inner.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "board_inner.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func boardRow(t *testing.T, rows []*geom.BoardLayerRow, name string) *geom.BoardLayerRow {
	t.Helper()
	for _, r := range rows {
		if r.GetName() == name {
			return r
		}
	}
	t.Fatalf("no row %q in %v", name, rowNames(rows))
	return nil
}

func rowNames(rows []*geom.BoardLayerRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.GetName())
	}
	return out
}

// packedGroups counts a packed sheet's primitives per group.
func packedGroups(p *geom.PackedSheet) map[uint32]int {
	out := map[uint32]int{}
	recs := p.GetPrimitives()
	for i := 0; i+primRecordBytes <= len(recs); i += primRecordBytes {
		out[uint32(recs[i+1])]++
	}
	return out
}

func TestBoardLayersListTheStackFrontFirst(t *testing.T) {
	rows := BoardLayers(innerBoard(t))
	want := []string{"F.Cu", "In1.Cu", "In2.Cu", "B.Cu", "Edge", "Through", "Zones", "B.SilkS", "F.SilkS"}
	if got := strings.Join(rowNames(rows), " "); got != strings.Join(want, " ") {
		t.Fatalf("rows %q, want %q", got, strings.Join(want, " "))
	}
	sides := map[string]string{"F.Cu": "front", "In1.Cu": "inner", "In2.Cu": "inner", "B.Cu": "back", "Edge": "", "F.SilkS": "front", "B.SilkS": "back"}
	for name, side := range sides {
		if got := boardRow(t, rows, name).GetSide(); got != side {
			t.Errorf("%s side %q, want %q", name, got, side)
		}
	}
	// A layer the geometry never draws gets no row, so the old fixture lists no inner layer.
	for _, r := range BoardLayers(boardFixtureGeometry(t)) {
		if r.GetSide() == "inner" {
			t.Errorf("two-layer board lists %q", r.GetName())
		}
	}
}

func TestEveryPackedPrimitiveHidesWithARow(t *testing.T) {
	b := innerBoard(t)
	rows := BoardLayers(b)
	owned := map[uint32]bool{}
	for _, r := range rows {
		for _, g := range r.GetGroups() {
			owned[g] = true
		}
	}
	groups := packedGroups(PackBoard(b))
	if len(groups) < 8 {
		t.Fatalf("only %d groups packed: %v", len(groups), groups)
	}
	for g, n := range groups {
		if !owned[g] {
			t.Errorf("%d primitives in group %d, which no row hides", n, g)
		}
	}
	// Each inner layer packs into a group of its own, so hiding one leaves the other drawn.
	in1, in2 := boardRow(t, rows, "In1.Cu").GetGroups(), boardRow(t, rows, "In2.Cu").GetGroups()
	if in1[0] == in2[0] {
		t.Errorf("In1.Cu and In2.Cu share group %d", in1[0])
	}
	if groups[in1[0]] == 0 {
		t.Errorf("In1.Cu's group %d packs nothing", in1[0])
	}
	// The In2.Cu plane hides with its layer and with the Zones row.
	zones := boardRow(t, rows, "Zones").GetGroups()
	if len(in2) != 2 || len(zones) != 1 || in2[1] != zones[0] || groups[zones[0]] == 0 {
		t.Errorf("In2.Cu groups %v, Zones groups %v, want the zone group in both", in2, zones)
	}
	for _, l := range PackBoard(b).GetLabels() {
		if l.GetText() == "BOTTOM" && l.GetGroup() != boardRow(t, rows, "B.SilkS").GetGroups()[0] {
			t.Errorf("B.SilkS text packs in group %d", l.GetGroup())
		}
	}
}

func TestEverySVGRowSelectsWhatItDraws(t *testing.T) {
	b := innerBoard(t)
	doc := BoardSVG(b, WithPickTargets())
	for _, r := range BoardLayers(b) {
		for _, sel := range strings.Split(r.GetSvgSelector(), ", ") {
			var needle string
			switch {
			case strings.HasPrefix(sel, "[data-layer="):
				needle = strings.TrimSuffix(strings.TrimPrefix(sel, "["), "]")
			case strings.HasPrefix(sel, "."):
				needle = `class="` + strings.TrimPrefix(sel, ".") + `"`
			}
			if needle == "" || !strings.Contains(doc, needle) {
				// A zone class names all three sides, and a part pick class one side, and a board
				// draws only some of them.
				if r.GetKind() != "zones" && !strings.HasPrefix(sel, ".picks-") {
					t.Errorf("row %s selector %q matches nothing in the SVG", r.GetName(), sel)
				}
			}
		}
	}
	for _, want := range []string{`data-layer="In2.Cu" fill=`, `data-layer="B.SilkS">BOTTOM`} {
		if !strings.Contains(doc, want) {
			t.Errorf("SVG lacks %s", want)
		}
	}
}

func TestLayerSelectorQuotesTheName(t *testing.T) {
	if got := layerSelector(`My "odd" layer\`); got != `[data-layer="My \"odd\" layer\\"]` {
		t.Errorf("selector %s", got)
	}
}

// KiCad 9 numbers F.Cu 0, B.Cu 2 and the inner layers from 4, and lists them in stackup order. The
// rows follow the table's order, not the numbers (Jetson read F.Cu, B.Cu, In1.Cu at first).
func TestBoardLayersFollowTheTableNotTheNumbers(t *testing.T) {
	seg := func(layer string) *geom.TrackSegment {
		return &geom.TrackSegment{A: &geom.Point{X: 0, Y: 0}, B: &geom.Point{X: 1_000_000, Y: 0}, Width: 100_000, Layer: layer}
	}
	b := &geom.BoardGeometry{
		Layers: []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}, {Number: 4, Name: "In1.Cu", Kind: "signal"}, {Number: 6, Name: "In2.Cu", Kind: "power"}, {Number: 2, Name: "B.Cu", Kind: "signal"}, {Number: 5, Name: "F.SilkS", Kind: "user"}, {Number: 7, Name: "B.SilkS", Kind: "user"}},
		Nets:   []*geom.NetCopper{{Net: "N", Segments: []*geom.TrackSegment{seg("B.Cu"), seg("In2.Cu"), seg("F.Cu"), seg("In1.Cu")}}},
		Graphics: []*geom.BoardGraphic{
			{Layer: "B.SilkS", Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}}},
			{Layer: "F.SilkS", Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 0}, {X: 1, Y: 1}}}},
		},
	}
	want := "F.Cu In1.Cu In2.Cu B.Cu F.SilkS B.SilkS"
	if got := strings.Join(rowNames(BoardLayers(b)), " "); got != want {
		t.Errorf("rows %q, want %q", got, want)
	}
}
