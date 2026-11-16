package formats

import (
	"math"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// outlineStats is the box and the shoelace area of the largest closed ring an outline's paths chain
// into, as the stack view chains them, in millimetres.
func outlineStats(o *geom.BoardOutline) (x0, y0, x1, y1, area float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range o.GetPaths() {
		pts := p.GetPoints()
		a := 0.0
		for i := range pts {
			q, r := pts[i], pts[(i+1)%len(pts)]
			x0, y0 = math.Min(x0, float64(q.GetX())/1e6), math.Min(y0, float64(q.GetY())/1e6)
			x1, y1 = math.Max(x1, float64(q.GetX())/1e6), math.Max(y1, float64(q.GetY())/1e6)
			a += float64(q.GetX())*float64(r.GetY()) - float64(r.GetX())*float64(q.GetY())
		}
		area += a / 2 / 1e12
	}
	return x0, y0, x1, y1, math.Abs(area)
}

// TestAllegro166OutlineMatchesItsODX holds parasdr's board outline, read from its 16.6 board, to the
// outline the ODX Allegro exported from it draws (agni 1669). The board draws its outline as 16
// pieces inside a package definition, and the ODX as one closed path, so the box must agree to a
// micrometre and the stack view's filled board to half a percent, as arcs flatten differently.
func TestAllegro166OutlineMatchesItsODX(t *testing.T) {
	l := &Loader{}
	bg, err := l.BoardGeometry(parasdr + "parasdr_layout.brd")
	if err != nil {
		t.Fatal(err)
	}
	og, err := l.BoardGeometry(parasdr + "parasdr_odb++.xml")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(bg.GetOutline().GetPaths()); n == 0 {
		t.Fatal("the board reads with no outline")
	}
	bx0, by0, bx1, by1, _ := outlineStats(bg.GetOutline())
	ox0, oy0, ox1, oy1, oarea := outlineStats(og.GetOutline())
	for _, d := range []float64{bx0 - ox0, by0 - oy0, bx1 - ox1, by1 - oy1} {
		if math.Abs(d) > 0.001 {
			t.Errorf("outline box %.3f..%.3f x %.3f..%.3f mm, the ODX's %.3f..%.3f x %.3f..%.3f", bx0, bx1, by0, by1, ox0, ox1, oy0, oy1)
			break
		}
	}
	filled := filledArea(bg)
	if math.Abs(filled-oarea) > oarea*0.005 {
		t.Errorf("the stack view fills %.1f mm², the ODX's outline encloses %.1f", filled, oarea)
	}
	// The corners are rounded with a 1.5 mm radius. A stack view that fell back to the outline's box,
	// as it does when no ring closes, would fill the very corner too.
	v := render.BoardStack(bg)
	u := v.GetUnitsPerMm()
	if trisCover(v.GetOutlineTris(), (ox0+0.05)*u, (oy0+0.05)*u) {
		t.Error("the stack view fills the board's rounded corner, so it fell back to the outline's box")
	}
	if !trisCover(v.GetOutlineTris(), (ox0+ox1)/2*u, (oy0+oy1)/2*u) {
		t.Error("the stack view leaves the middle of the board empty")
	}
}

// trisCover says whether a point lies inside one of a flat x,y triangle list's triangles.
func trisCover(tris []float64, x, y float64) bool {
	for i := 0; i+5 < len(tris); i += 6 {
		ax, ay, bx, by, cx, cy := tris[i], tris[i+1], tris[i+2], tris[i+3], tris[i+4], tris[i+5]
		d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
		d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
		d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
		if !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0)) {
			return true
		}
	}
	return false
}

// filledArea is the area the stack view fills for the board, its outline less the holes it cuts, in
// square millimetres, plus the holes back, so it compares with an outline's own area.
func filledArea(g *geom.BoardGeometry) float64 {
	v := render.BoardStack(g)
	tris := v.GetOutlineTris()
	a := 0.0
	for i := 0; i+5 < len(tris); i += 6 {
		a += math.Abs((tris[i+2]-tris[i])*(tris[i+5]-tris[i+1])-(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])) / 2
	}
	a /= v.GetUnitsPerMm() * v.GetUnitsPerMm()
	least := render.HoleCutMm * 1e6
	seen := map[[2]int64]bool{}
	for _, pl := range g.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if float64(pad.GetDrill()) < least {
				continue
			}
			w := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
			k := [2]int64{w.GetX(), w.GetY()}
			if seen[k] {
				continue
			}
			seen[k] = true
			r := float64(pad.GetDrill()) / 2 / 1e6
			a += 8 * r * r * math.Sin(math.Pi/8)
		}
	}
	return a
}
