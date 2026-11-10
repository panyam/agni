package render

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A board's layer names come from the design, and KiCad names its through-board zone layer
// F&B.Cu. A raw & in a data-layer attribute made Glasgow revC3's board an invalid SVG, and a
// browser stopped drawing it at that zone. Every element that names a layer goes through here.
func TestBoardSVGIsWellFormedWithAnAmpersandInALayerName(t *testing.T) {
	const layer = "F&B.Cu"
	pt := func(x, y int64) *geom.Point { return &geom.Point{X: x, Y: y} }
	b := &geom.BoardGeometry{
		Zones:    []*geom.Zone{{Net: "GND", Layer: layer, Outline: &geom.Polyline{Points: []*geom.Point{pt(0, 0), pt(10_000_000, 0), pt(10_000_000, 10_000_000)}}}},
		Graphics: []*geom.BoardGraphic{{Layer: layer, Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{pt(0, 0), pt(5_000_000, 0)}}}},
		Texts:    []*geom.BoardText{{Text: "T", At: pt(5_000_000, 5_000_000), Layer: layer, Kind: "gr"}},
	}
	svg := BoardSVG(b)
	d := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := d.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("BoardSVG is not well-formed XML: %v", err)
		}
	}
	if got := strings.Count(svg, `data-layer="F&amp;B.Cu"`); got < 3 {
		t.Fatalf("found %d escaped data-layer attributes, want at least 3 (zone, graphic, text)", got)
	}
}
