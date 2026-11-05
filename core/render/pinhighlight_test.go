package render

import (
	"regexp"
	"strconv"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A pin is a point, so its framing rect is all pad. On a KiCad sheet, in nanometres, the old
// 8-unit floor drew a rect with no size, and a pin picked on the board lit nothing on the
// schematic (agni issue 1374). The rect must be at least the pixel floor across.
func TestAHighlightedPinDrawsAVisibleRect(t *testing.T) {
	g := readKiCadSheet(t, "orient-jetson-caps.kicad_sch")
	spec := &geom.HighlightSpec{
		Pins:  []*geom.PinRef{{RefDes: "C133", Pin: "1"}},
		Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT,
	}
	out := SheetSVGHighlighted(g, g.Sheets[0], []*geom.HighlightSpec{spec})
	rects := regexp.MustCompile(`<rect x="[^"]*" y="[^"]*" width="([^"]*)" height="([^"]*)" fill="#ed1cb8"`).FindAllStringSubmatch(out, -1)
	if len(rects) != 1 {
		t.Fatalf("the highlight drew %d rects for one pin, want 1", len(rects))
	}
	for i, side := range []string{"width", "height"} {
		v, err := strconv.ParseFloat(rects[0][i+1], 64)
		if err != nil || v < 2*pointPadPx {
			t.Errorf("the pin's rect is %s px %s, want at least %d", rects[0][i+1], side, 2*pointPadPx)
		}
	}
}
