package render

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A label parked far off the board stays out of the frame, so the board keeps the drawing's width.
// A label just past the edge, such as a designator beside a connector, stays in.
func TestTheFrameLeavesOutAParkedLabel(t *testing.T) {
	mm := int64(1_000_000)
	board := func(texts ...*geom.BoardText) *geom.BoardGeometry {
		return &geom.BoardGeometry{
			UnitNm: 1,
			Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{
				{X: 0, Y: 0}, {X: 80 * mm, Y: 0}, {X: 80 * mm, Y: 50 * mm}, {X: 0, Y: 50 * mm}, {X: 0, Y: 0},
			}}}},
			Texts: texts,
		}
	}
	bare := frameBoard(board())
	parked := frameBoard(board(&geom.BoardText{Text: "C157", At: &geom.Point{X: 0, Y: 330 * mm}, Height: mm}))
	if parked.pxW != bare.pxW || parked.pxH != bare.pxH {
		t.Errorf("a label 280 mm off the board moved the frame from %gx%g to %gx%g", bare.pxW, bare.pxH, parked.pxW, parked.pxH)
	}
	beside := frameBoard(board(&geom.BoardText{Text: "J1", At: &geom.Point{X: 85 * mm, Y: 25 * mm}, Height: mm}))
	if beside.pxW == bare.pxW && beside.pxH == bare.pxH {
		t.Errorf("a label 5 mm past the edge left the frame at %gx%g, want it framed", bare.pxW, bare.pxH)
	}

	// With no outline there is nothing to be near, so every label is framed.
	open := &geom.BoardGeometry{UnitNm: 1, Texts: []*geom.BoardText{
		{Text: "A", At: &geom.Point{X: 0, Y: 0}}, {Text: "B", At: &geom.Point{X: 300 * mm, Y: 0}},
	}}
	if f := frameBoard(open); f.pxW <= f.pxH {
		t.Errorf("an outline-less board framed %gx%g, want both labels in a wide frame", f.pxW, f.pxH)
	}
}
