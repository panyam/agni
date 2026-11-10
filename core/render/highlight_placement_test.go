package render

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// twoR2 is a sheet that places R2 twice, far apart, as a design that repeats a part across two
// copies of a block can (agni issue 1477).
func twoR2() *geom.SchematicGeometry {
	g := highlightFixture()
	g.Sheets[0].Placements = append(g.Sheets[0].Placements,
		&geom.SymbolPlacement{RefDes: "R2", CellRef: "R", LibraryRef: "L", ViewRef: "v", Transform: &geom.Transform{Origin: &geom.Point{X: 0, Y: 0}}},
		&geom.SymbolPlacement{RefDes: "R2", CellRef: "R", LibraryRef: "L", ViewRef: "v", Transform: &geom.Transform{Origin: &geom.Point{X: 5000, Y: 5000}}},
	)
	return g
}

// Each placement's symbol and pin primitives carry that placement's index, so a viewer frames the
// two R2s apart. A primitive no placement drew carries none.
func TestPackSheetKeysEachPlacement(t *testing.T) {
	g := twoR2()
	ps := PackSheet(g, g.Sheets[0])
	byPlacement := map[uint32]int{}
	for _, k := range ps.GetKeys() {
		switch {
		case k.GetRefDes() == "R2":
			byPlacement[k.GetPlacement()]++
		case k.GetNet() != "" && k.GetRefDes() == "" && k.GetPlacement() != 0:
			t.Errorf("wire key on %s carries placement %d, want 0", k.GetNet(), k.GetPlacement())
		}
	}
	// R1 and U1 come first, so the two R2s are placements 3 and 4. Each draws one rect and two pins.
	if len(byPlacement) != 2 || byPlacement[3] != 3 || byPlacement[4] != 3 {
		t.Errorf("R2 keys by placement %v, want 3 on placement 3 and 3 on placement 4", byPlacement)
	}
}

// The SVG overlay frames each placement of R2 on its own, the behaviour the packed keys let WebGL
// match.
func TestHighlightSVGFramesEachPlacement(t *testing.T) {
	g := twoR2()
	doc := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{Components: []string{"R2"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT}})
	if n := strings.Count(doc, "<rect"); n != 2 {
		t.Errorf("bounding rects = %d, want 2, one per placement of R2", n)
	}
}
