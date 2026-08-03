package render

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestCollectLabels checks that every text source is gathered with the SVG-matching color and
// world position: placement fields, sheet labels, the title-block rows, and the zone ruler.
func TestCollectLabels(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1", Name: "S1",
			Size:       &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 1000, Y: 800}},
			TitleBlock: &geom.TitleBlock{Title: "T"},
			Placements: []*geom.SymbolPlacement{{
				RefDes: "R1",
				Fields: []*geom.Field{{Value: "R1", Origin: &geom.Point{X: 500, Y: 400}, Height: 20, Visible: true, Justify: "center center"}},
			}},
			Labels: []*geom.Label{{Text: "NET", Origin: &geom.Point{X: 100, Y: 100}, Height: 15}},
		}},
	}

	ls := collectLabels(g, g.Sheets[0], DefaultStyle)
	find := func(txt string) *placedLabel {
		for i := range ls {
			if ls[i].text == txt {
				return &ls[i]
			}
		}
		return nil
	}

	if f := find("R1"); f == nil || f.color != DefaultStyle.Field || f.x != 500 || f.y != 400 || f.height != 20 {
		t.Errorf("R1 field = %+v, want field color at (500,400) h=20", f)
	}
	if l := find("NET"); l == nil || l.color != DefaultStyle.Label || l.height != 15 {
		t.Errorf("NET label = %+v, want label color h=15", l)
	}
	if find("Title: T") == nil {
		t.Error("missing title-block Title row")
	}
	if find("Sheet: S1") == nil {
		t.Error("missing title-block Sheet row")
	}
	if r := find("1"); r == nil || r.color != DefaultStyle.Ruler {
		t.Errorf("ruler number 1 = %+v, want ruler color", r)
	}
	if find("A") == nil {
		t.Error("missing ruler letter A")
	}

	// A sheet with no page gets no worksheet text.
	g.Sheets[0].Size = nil
	for _, l := range collectLabels(g, g.Sheets[0], DefaultStyle) {
		if l.text == "1" || l.text == "A" {
			t.Fatalf("ruler text %q emitted for a sheet with no page", l.text)
		}
	}
}

// TestCollectLabelsUpright is the WebGL-side counterpart to the render layer's readable-text
// rule: the overlay applies rotationDeg/justify verbatim, so collectLabels must hand it
// already-upright text. A label whose source orientation is R180 must arrive flipped upright
// (no upside-down rotation) with its justify swapped, matching the SVG backend.
func TestCollectLabelsUpright(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Id: "P1", Name: "S1",
			Size:   &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 1000, Y: 800}},
			Labels: []*geom.Label{{Text: "NET_FLIP", Origin: &geom.Point{X: 100, Y: 100}, Height: 15, RotationDeg: 180, Justify: "left"}},
		}},
	}
	for _, l := range collectLabels(g, g.Sheets[0], DefaultStyle) {
		if l.text != "NET_FLIP" {
			continue
		}
		if normDeg(l.rotationDeg) == 180 {
			t.Errorf("R180 label reached the overlay upside down (rot=%d)", l.rotationDeg)
		}
		if l.justify != "right" {
			t.Errorf("flipped label justify = %q, want %q", l.justify, "right")
		}
	}
}

// TestCaptionWidth checks the caption-width budget: it is the symbol's drawn BOX rectangle
// (the box a label like "Net Splitter" is fit inside), not the full bounding box that pin
// stubs widen, and it falls back to the bounding box when the symbol has no BOX figure. This
// backs the condense-to-fit that stops the "Net Splitter" caption spilling past its rectangle.
func TestCaptionWidth(t *testing.T) {
	sym := &geom.SymbolDef{
		Bbox: &geom.BBox{Min: &geom.Point{X: 0}, Max: &geom.Point{X: 1778}}, // pin stubs widen the bbox
		Shapes: []*geom.Shape{
			{Kind: geom.Shape_KIND_RECT, FigureGroup: "BOX", Points: []*geom.Point{{X: 254}, {X: 1524}}}, // body: 1270 wide
			{Kind: geom.Shape_KIND_POLYLINE, FigureGroup: "PIN", Points: []*geom.Point{{X: 0}, {X: 254}}},
		},
	}
	if w := captionWidth(sym); w != 1270 {
		t.Errorf("captionWidth = %d, want 1270 (the BOX rect, not the 1778 bbox)", w)
	}
	// No BOX figure: fall back to the full bounding box.
	noBox := &geom.SymbolDef{Bbox: &geom.BBox{Min: &geom.Point{X: 10}, Max: &geom.Point{X: 90}}}
	if w := captionWidth(noBox); w != 80 {
		t.Errorf("captionWidth fallback = %d, want 80 (bbox width)", w)
	}
	if w := captionWidth(nil); w != 0 {
		t.Errorf("captionWidth(nil) = %d, want 0", w)
	}
}

// TestPackSheetLabelsRebased checks that PackSheet carries labels rebased to the same origin
// as the vertex blob, so the overlay works in the same sheet-local space as the geometry.
func TestPackSheetLabelsRebased(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id: "P1",
		Wires: []*geom.WireGeometry{{Net: "N", Polylines: []*geom.Polyline{
			{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}},
		}}},
		Labels: []*geom.Label{{Text: "NET", Origin: &geom.Point{X: 150, Y: 130}, Height: 10}},
	}}}

	ps := PackSheet(g, g.Sheets[0])
	if len(ps.Labels) != 1 {
		t.Fatalf("labels = %d, want 1", len(ps.Labels))
	}
	// Origin is the wire's min corner (100,100); the label at (150,130) rebases to (50,30).
	if l := ps.Labels[0]; l.X != 50 || l.Y != 30 || l.Text != "NET" {
		t.Errorf("rebased label = {x:%d y:%d text:%q}, want (50,30) NET", l.X, l.Y, l.Text)
	}
}
