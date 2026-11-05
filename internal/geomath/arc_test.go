package geomath

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func pt(x, y int64) *geom.Point { return &geom.Point{X: x, Y: y} }

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestArcThroughTurnsTheWayItsMidPointSays(t *testing.T) {
	// A quarter circle of radius 1000 about the origin, from (1000, 0) to (0, 1000).
	ccw, ok := ArcThrough(pt(1000, 0), pt(707, 707), pt(0, 1000))
	if !ok || !near(ccw.R, 1000, 1) || !near(ccw.Sweep, math.Pi/2, 1e-3) || !near(ccw.Cx, 0, 1) || !near(ccw.Cy, 0, 1) {
		t.Errorf("counter-clockwise quarter = %+v, want r 1000 about the origin turning +pi/2", ccw)
	}
	// The same ends with the mid point on the far side: three quarters of a circle, clockwise.
	cw, ok := ArcThrough(pt(1000, 0), pt(-707, -707), pt(0, 1000))
	if !ok || !near(cw.Sweep, -3*math.Pi/2, 1e-3) {
		t.Errorf("clockwise three quarters = %+v, want a sweep of -3pi/2", cw)
	}
	if _, ok := ArcThrough(pt(0, 0), pt(1, 1), pt(2, 2)); ok {
		t.Error("three collinear points made an arc")
	}
}

func TestArcMidFollowsTheDirection(t *testing.T) {
	x, y := ArcMid(1000, 0, 0, 1000, 0, 0, false)
	if !near(x, 707.1, 0.1) || !near(y, 707.1, 0.1) {
		t.Errorf("counter-clockwise mid = (%.1f, %.1f), want (707.1, 707.1)", x, y)
	}
	x, y = ArcMid(1000, 0, 0, 1000, 0, 0, true)
	if !near(x, -707.1, 0.1) || !near(y, -707.1, 0.1) {
		t.Errorf("clockwise mid = (%.1f, %.1f), want (-707.1, -707.1)", x, y)
	}
}

func TestCenterArcSplitsAFullCircle(t *testing.T) {
	if got := CenterArc(1000, 0, 0, 1000, 0, 0, false); len(got) != 1 {
		t.Errorf("a quarter circle came back as %d pieces, want 1", len(got))
	}
	got := CenterArc(1000, 0, 1000, 0, 0, 0, true)
	if len(got) != 2 {
		t.Fatalf("a full circle came back as %d pieces, want 2", len(got))
	}
	if got[0].S != got[1].E || got[0].E != got[1].S || !near(got[0].E[0], -1000, 1e-6) {
		t.Errorf("halves = %+v, want (1000,0) to (-1000,0) and back", got)
	}
	if !near(got[0].M[1], -1000, 1e-6) {
		t.Errorf("clockwise first half passes %v, want (0,-1000)", got[0].M)
	}
}

func TestTrackPathChordsAnArcAndLeavesALineAlone(t *testing.T) {
	line := &geom.TrackSegment{A: pt(0, 0), B: pt(10, 0)}
	if p := TrackPath(line); len(p) != 2 {
		t.Errorf("straight run path = %v, want its two ends", p)
	}
	arc := &geom.TrackSegment{A: pt(1_000_000, 0), Mid: pt(707_107, 707_107), B: pt(0, 1_000_000)}
	p := TrackPath(arc)
	// pi/2 in steps of at most a tenth of a radian is 16 chords.
	if len(p) != 17 || p[0] != arc.A || p[len(p)-1] != arc.B {
		t.Fatalf("arc path has %d points, want 17 from a to b", len(p))
	}
	for _, q := range p {
		if r := math.Hypot(float64(q.X), float64(q.Y)); !near(r, 1_000_000, 2) {
			t.Errorf("chord point %v is %.0f from the center, want 1,000,000", q, r)
		}
	}
	if l := TrackLength(arc); !near(l, math.Pi/2*1_000_000, 2) {
		t.Errorf("arc length = %.0f, want pi/2 * 1e6", l)
	}
	if l := TrackLength(line); l != 10 {
		t.Errorf("line length = %v, want 10", l)
	}
}

func TestTurnToMeasuresInTheArcsDirection(t *testing.T) {
	cw, _ := ArcThrough(pt(1000, 0), pt(0, -1000), pt(-1000, 0))
	if got := cw.TurnTo(0, -1000); !near(got, -math.Pi/2, 1e-3) {
		t.Errorf("clockwise turn to the mid = %v, want -pi/2", got)
	}
}

// A consumer that ranges over a board's track segments and reads only a and b draws an arc as the
// chord across it, which is worse than the chords the readers wrote before agni 1358. So every
// non-test file that ranges over geom.NetCopper segments must read them through TrackPath or look
// at the mid point, or draw them through core/render's drawTrack. The model's BoardSeg list is chorded already, so ranging over it is fine.
var segmentRangeAllowed = map[string]string{
	"stdlib/rules/builtin/rule_netclass_conformance.go": "ranges over check.BoardNet's BoardSegs, chorded by the model",
	"stdlib/rules/builtin/rule_copper_clearance.go":     "ranges over check.BoardNet's BoardSegs, chorded by the model",
	"stdlib/relations/facts.go":                         "ranges over check.BoardNet's BoardSegs for their width and layer, chorded by the model",
}

func TestEveryTrackSegmentConsumerSeesArcs(t *testing.T) {
	root := filepath.Join("..", "..")
	rangeSegs := regexp.MustCompile(`range [\w.()]+\.(GetSegments\(\)|Segments)\s*\{`)
	found := 0
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch rel {
			case "gen", "node_modules", "web", "tools", "docsite", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !rangeSegs.Match(src) {
			return nil
		}
		found++
		if _, ok := segmentRangeAllowed[rel]; ok {
			seen[rel] = true
			return nil
		}
		if !strings.Contains(string(src), "TrackPath(") && !strings.Contains(string(src), "GetMid()") && !strings.Contains(string(src), "drawTrack(") {
			t.Errorf("%s ranges over track segments and reads neither geomath.TrackPath, a segment's mid nor render's drawTrack, so it draws an arc as its chord", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 7 {
		t.Fatalf("found %d files ranging over track segments, want the renderers, the model, the raster and the rules", found)
	}
	for f := range segmentRangeAllowed {
		if !seen[f] {
			t.Errorf("allowlist entry %s no longer ranges over segments; remove it", f)
		}
	}
}
