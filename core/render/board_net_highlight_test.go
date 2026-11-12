package render

import (
	"math"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// netBoard is twoPadBoard with C1.1 on net SIG, a track leaving it and a via at the track's end.
func netBoard() *geom.BoardGeometry {
	b := twoPadBoard()
	b.Placements[0].Pads[0].Net = "SIG"
	b.Nets = []*geom.NetCopper{{
		Net:      "SIG",
		Segments: []*geom.TrackSegment{{A: &geom.Point{X: 49_500_000, Y: 40_000_000}, B: &geom.Point{X: 49_500_000, Y: 30_000_000}, Width: 200_000, Layer: "F.Cu"}},
		Vias:     []*geom.Via{{At: &geom.Point{X: 49_500_000, Y: 30_000_000}, Size: 600_000}},
	}}
	return b
}

// A net lit on the board repaints its own copper (agni issue 1543): its track opaque at the default
// opacity, its pad filled over its land rather than ringed, and its via filled. The WebGL board does
// the same (web/src/canvas.ts, boardNetDraws).
func TestBoardNetRepaintsItsCopper(t *testing.T) {
	svg := HighlightBoardSVG(netBoard(), []*geom.HighlightSpec{{Nets: []string{"SIG"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH}})
	if !strings.Contains(svg, `stroke-opacity="1.0"`) {
		t.Errorf("the track is not opaque at the default opacity: %s", svg)
	}
	if strings.Contains(svg, `fill="none"`) {
		t.Errorf("a pad or via of the net is ringed rather than filled: %s", svg)
	}
	pads := strings.Count(svg, `<path d="M`)
	if pads != 1 {
		t.Errorf("filled pad lands = %d, want 1 (C1.1 alone; C1.2 is on no net): %s", pads, svg)
	}
	if !strings.Contains(svg, `<circle`) || !strings.Contains(svg, `fill="`+DefaultHighlightColor+`"`) {
		t.Errorf("the via is not filled in the highlight colour: %s", svg)
	}

	faint := HighlightBoardSVG(netBoard(), []*geom.HighlightSpec{{Nets: []string{"SIG"}, Shape: geom.HighlightShape_HIGHLIGHT_SHAPE_PATH, Alpha: 0.2}})
	if !strings.Contains(faint, `stroke-opacity="0.5"`) {
		t.Errorf("a path alpha of 0.2 should repaint the track at 0.5: %s", faint)
	}
}

// TestBoardNetAlphaTwin pins the opacity a board net repaints at. web/src/highlights.test.ts asserts
// the same numbers for boardNetAlpha.
func TestBoardNetAlphaTwin(t *testing.T) {
	for _, c := range []struct {
		in   float32
		want float64
	}{{0, 1}, {0.4, 1}, {0.2, 0.5}, {0.8, 1}} {
		// A spec's alpha is a float32, so 0.2 arrives as 0.2000000029.
		if got := boardNetAlpha(c.in); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("boardNetAlpha(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
