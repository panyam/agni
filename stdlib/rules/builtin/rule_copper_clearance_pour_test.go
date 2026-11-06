package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pourCase is a GND pour 10 mm square on F.Cu with a 2 mm square hole at its centre, and one SIG
// track of 0.2 mm width on F.Cu.
func pourCase(track *geom.TrackSegment) []check.Verdict {
	sq := func(x0, y0, x1, y1 int64) *geom.Polyline {
		return &geom.Polyline{Points: []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}
	}
	g := &geom.BoardGeometry{UnitNm: 1, Nets: []*geom.NetCopper{
		{Net: "GND", Shapes: []*geom.CopperShape{{Layer: "F.Cu", Outline: sq(0, 0, 10_000_000, 10_000_000), Holes: []*geom.Polyline{sq(4_000_000, 4_000_000, 6_000_000, 6_000_000)}}}},
		{Net: "SIG", Segments: []*geom.TrackSegment{track}},
	}}
	return copperClearanceVerdicts(context.Background(), check.NewModel(&ir.Design{}, check.WithBoard(g)))
}

func hTrack(x0, x1, y int64) *geom.TrackSegment {
	return &geom.TrackSegment{A: &geom.Point{X: x0, Y: y}, B: &geom.Point{X: x1, Y: y}, Width: 200_000, Layer: "F.Cu"}
}

// A track measures against a pour's edges as against another track's, and copper buried inside
// another net's pour is a short (agni 1410).
func TestAPourIsCopperToTheClearanceRule(t *testing.T) {
	for _, c := range []struct {
		name  string
		track *geom.TrackSegment
		want  check.Outcome
		gap   string
	}{
		// The track runs across the hole's middle, 0.9 mm from its top and bottom edges.
		{"a track in a hole with room", hTrack(4_400_000, 5_600_000, 5_000_000), check.Pass, ""},
		// 0.1 mm from the hole's bottom edge, centre to edge, leaves no copper gap at all.
		{"a track too near a hole's edge", hTrack(4_400_000, 5_600_000, 4_100_000), check.Fail, "worst gap 0.000mm"},
		// Wholly inside the GND copper, crossing no edge.
		{"a track buried in another net's pour", hTrack(1_000_000, 2_000_000, 1_000_000), check.Fail, "worst gap -0.100mm"},
		// Outside the pour, 0.3 mm from its right edge.
		{"a track beside the pour with room", &geom.TrackSegment{A: &geom.Point{X: 10_400_000, Y: 1_000_000}, B: &geom.Point{X: 10_400_000, Y: 2_000_000}, Width: 200_000, Layer: "F.Cu"}, check.Pass, ""},
	} {
		vs := pourCase(c.track)
		if len(vs) != 1 {
			if c.want == check.Pass && len(vs) == 0 {
				continue // too far to be worth measuring is a pass too
			}
			t.Errorf("%s: %d verdicts, want 1", c.name, len(vs))
			continue
		}
		if vs[0].Outcome != c.want {
			t.Errorf("%s: %v, want %v (%s)", c.name, vs[0].Outcome, c.want, vs[0].Witness.Statement)
		}
		if c.gap != "" && !strings.Contains(vs[0].Witness.Statement, c.gap) {
			t.Errorf("%s: %q, want %s", c.name, vs[0].Witness.Statement, c.gap)
		}
	}
}

// A gap to a pour's edge may sit 5 microns under the floor, the arc error a file's pour edge carries,
// and no further (agni 1410). The track edge sits at floor - 4 um and then floor - 6 um from the
// pour's right edge.
func TestAPourEdgeGapHasTheArcTolerance(t *testing.T) {
	at := func(under int64) check.Outcome {
		x := int64(10_000_000) + 127_000 - under + 100_000 // the track's centre: the floor less under, plus its half width
		vs := pourCase(&geom.TrackSegment{A: &geom.Point{X: x, Y: 1_000_000}, B: &geom.Point{X: x, Y: 2_000_000}, Width: 200_000, Layer: "F.Cu"})
		if len(vs) != 1 {
			t.Fatalf("%d verdicts", len(vs))
		}
		return vs[0].Outcome
	}
	if got := at(4_000); got != check.Pass {
		t.Errorf("4 microns under the floor: %v, want a pass", got)
	}
	if got := at(6_000); got != check.Fail {
		t.Errorf("6 microns under the floor: %v, want a fail", got)
	}
}
