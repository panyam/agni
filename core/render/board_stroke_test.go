package render

import (
	"encoding/binary"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestPackBoardCarriesEachGraphicsWidth packs a 0.05 mm courtyard rectangle, a 0.12 mm silkscreen
// line and a silkscreen line with no width. Each record carries its graphic's width in micrometres,
// so WebGL can draw the outline as wide as BoardSVG strokes it (agni issue 1486).
func TestPackBoardCarriesEachGraphicsWidth(t *testing.T) {
	b := &geom.BoardGeometry{
		UnitNm: 1,
		Graphics: []*geom.BoardGraphic{
			{Layer: "F.CrtYd", RefDes: "U1", Width: 50_000, Shape: &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 5e6, Y: 3e6}}}},
			{Layer: "F.SilkS", RefDes: "U1", Width: 120_000, Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 0}, {X: 5e6, Y: 0}}}},
			{Layer: "F.SilkS", Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 1e6}, {X: 5e6, Y: 1e6}}}},
		},
	}
	recs := PackBoard(b).GetPrimitives()
	var got []uint16
	for i := 0; i+primRecordBytes <= len(recs); i += primRecordBytes {
		got = append(got, binary.LittleEndian.Uint16(recs[i+2:]))
	}
	want := []uint16{50, 120, 0}
	if len(got) != len(want) {
		t.Fatalf("packed %d primitives, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("primitive %d carries %d um, want %d", i, got[i], want[i])
		}
	}
}

func TestStrokeMicronsRoundsAndSaturates(t *testing.T) {
	for _, tc := range []struct {
		nm   int64
		want uint16
	}{{0, 0}, {-5, 0}, {499, 0}, {500, 1}, {150_000, 150}, {1e12, 65535}} {
		if got := strokeMicrons(tc.nm); got != tc.want {
			t.Errorf("strokeMicrons(%d) = %d, want %d", tc.nm, got, tc.want)
		}
	}
}
