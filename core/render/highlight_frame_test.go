package render

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// elemAttr reads one numeric attribute off the first element named tag in an SVG string.
func elemAttr(t *testing.T, svg, tag, attr string) float64 {
	t.Helper()
	el := regexp.MustCompile(`<` + tag + `\b[^>]*>`).FindString(svg)
	if el == "" {
		t.Fatalf("no <%s> in %s", tag, svg)
	}
	m := regexp.MustCompile(`\b` + attr + `="([^"]*)"`).FindStringSubmatch(el)
	if m == nil {
		t.Fatalf("<%s> has no %s: %s", tag, attr, el)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// A frame is a translucent fill under an opaque stroke in the same color, and stroke_scale sizes
// the stroke (agni issue 1536).
func TestBoundingFrameStrokesOpaque(t *testing.T) {
	g := highlightFixture()
	for _, shape := range []geom.HighlightShape{geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT, geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE} {
		tag := "rect"
		if shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE {
			tag = "circle"
		}
		one := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{Components: []string{"R1"}, Color: "#00aaff", Shape: shape}})
		el := regexp.MustCompile(`<` + tag + `\b[^>]*>`).FindString(one)
		for _, want := range []string{`fill="#00aaff"`, `fill-opacity="0.3"`, `stroke="#00aaff"`} {
			if !strings.Contains(el, want) {
				t.Errorf("%v frame %s lacks %s", shape, el, want)
			}
		}
		if strings.Contains(el, "stroke-opacity") {
			t.Errorf("%v frame stroke must be opaque: %s", shape, el)
		}
		w1 := elemAttr(t, one, tag, "stroke-width")
		if w1 < 2 || w1 > 6 {
			t.Errorf("%v frame stroke = %v px, want a few pixels", shape, w1)
		}
		two := HighlightSVG(g, g.Sheets[0], []*geom.HighlightSpec{{Components: []string{"R1"}, Shape: shape, StrokeScale: 2}})
		if w2 := elemAttr(t, two, tag, "stroke-width"); math.Abs(w2-2*w1) > 0.15 {
			t.Errorf("%v stroke_scale 2 gave stroke %v, want %v", shape, w2, 2*w1)
		}
	}
}

// twoPadBoard is a 100 x 87 mm board holding one 0402-sized part, two 0.6 x 0.6 mm pads 1 mm apart.
func twoPadBoard() *geom.BoardGeometry {
	return &geom.BoardGeometry{UnitNm: 1,
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{
			{X: 0, Y: 0}, {X: 100_000_000, Y: 0}, {X: 100_000_000, Y: 87_000_000}, {X: 0, Y: 87_000_000}, {X: 0, Y: 0},
		}}}},
		Placements: []*geom.ComponentPlacement{
			{RefDes: "C1", At: &geom.Point{X: 50_000_000, Y: 40_000_000}, Layer: "F.Cu", Pads: []*geom.Pad{
				{Number: "1", At: &geom.Point{X: -500_000}, Size: &geom.Point{X: 600_000, Y: 600_000}, Shape: "rect", Layers: []string{"F.Cu"}},
				{Number: "2", At: &geom.Point{X: 500_000}, Size: &geom.Point{X: 600_000, Y: 600_000}, Shape: "rect", Layers: []string{"F.Cu"}},
			}},
		},
	}
}

// A selected part on the board frames as one rect, at about twice the span the 10% pad gave a small
// two-pad footprint, in place of the pad rings and origin dot (agni issue 1536).
func TestBoardFramesASmallPartAtTwiceItsOldSpan(t *testing.T) {
	b := twoPadBoard()
	fr := frameBoard(b)
	svg := HighlightBoardSVG(b, []*geom.HighlightSpec{{Components: []string{"C1"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT}})
	if n := strings.Count(svg, "<rect"); n != 1 {
		t.Fatalf("frames = %d, want 1: %s", n, svg)
	}
	if n := strings.Count(svg, "<circle"); n != 0 {
		t.Errorf("a framed part drew %d pad rings or dots, want none", n)
	}
	// The pads span 1.6 x 0.6 mm, and the 10% pad made that 1.92 mm wide.
	const raw, old = 1_600_000.0, 1_600_000.0 * 1.2
	got := elemAttr(t, svg, "rect", "width") / fr.scale
	if got < 1.8*old || got > 2.6*old {
		t.Errorf("frame width = %.0f nm, want about twice the old %.0f nm (raw %.0f)", got, old, raw)
	}
	if !strings.Contains(svg, `stroke="`+DefaultHighlightColor+`"`) {
		t.Error("board frame lacks the opaque stroke")
	}

	pin := HighlightBoardSVG(b, []*geom.HighlightSpec{{Pins: []*geom.PinRef{{RefDes: "C1", Pin: "2"}}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT}})
	if strings.Count(pin, "<rect") != 1 || strings.Contains(pin, "<circle") {
		t.Errorf("a framed pin should draw one rect and no ring: %s", pin)
	}
}

// A large part keeps its 10% pad, since the floor only lifts small parts.
func TestFrameFloorLeavesALargePartAlone(t *testing.T) {
	fr := frameBoard(twoPadBoard())
	e := &highlightEntity{pins: []*geom.Point{{X: 0, Y: 0}, {X: 30_000_000, Y: 30_000_000}}}
	fb, ok := framedBounds(e, fr.frameFloor())
	if !ok {
		t.Fatal("no bounds")
	}
	if got := fb.maxX - fb.minX; got != 36_000_000 {
		t.Errorf("30 mm part frames %v nm wide, want 36 mm", got)
	}
}

// The frame's fractions are twinned with web/src/highlights.ts, which asserts the same numbers.
func TestFrameFractionsTwin(t *testing.T) {
	if frameFloorFraction != 0.015 || frameStrokeFraction != 0.003 {
		t.Errorf("fractions = %v, %v; change web/src/highlights.ts with them", frameFloorFraction, frameStrokeFraction)
	}
}
