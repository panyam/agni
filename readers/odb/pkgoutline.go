package odb

import (
	"fmt"
	"math"
	"strings"
)

// A package's outline in eda/data is the records between its PKG line and its first PIN (agni
// 1656). RC is a rectangle from its lower-left corner, CR a circle, SQ a square from its centre and
// half side, and CT ... CE a contour of OB ... OE polygons, each an island (I) or a hole (H). The
// records after a PIN outline that pin, and the package outline skips them.
//
// Each tool fills it differently. KiCad's export draws the hull of the pads, Altium's the body as a
// contour, and Allegro's a rectangle or a circle.

// outlineRecord reads one outline record of the package pkg into its outline. It reports false for
// a record that is no outline record.
func outlineRecord(pkg *edaPkg, w []string, u units) (bool, error) {
	nums := func(k int) ([]float64, error) {
		if len(w) < k+1 {
			return nil, fmt.Errorf("%s needs %d values", w[0], k)
		}
		out := make([]float64, k)
		for i := range out {
			v, err := atof(w[i+1])
			if err != nil {
				return nil, fmt.Errorf("%s value %q: %w", w[0], w[i+1], err)
			}
			out[i] = v * u.coord
		}
		return out, nil
	}
	at := func(x, y float64) pt { return pt{x: int64(math.Round(x)), y: int64(math.Round(y))} }
	ring := func(r []pt) {
		if len(pkg.pins) == 0 && len(r) >= 3 {
			pkg.outline = append(pkg.outline, r)
		}
	}
	switch w[0] {
	case "RC":
		v, err := nums(4)
		if err != nil {
			return true, err
		}
		x0, y0, x1, y1 := v[0], v[1], v[0]+v[2], v[1]+v[3]
		ring([]pt{at(x0, y0), at(x1, y0), at(x1, y1), at(x0, y1)})
	case "SQ":
		v, err := nums(3)
		if err != nil {
			return true, err
		}
		x0, y0, x1, y1 := v[0]-v[2], v[1]-v[2], v[0]+v[2], v[1]+v[2]
		ring([]pt{at(x0, y0), at(x1, y0), at(x1, y1), at(x0, y1)})
	case "CR":
		v, err := nums(3)
		if err != nil {
			return true, err
		}
		r := make([]pt, 0, arcSegments)
		for i := range arcSegments {
			a := 2 * math.Pi * float64(i) / arcSegments
			r = append(r, at(v[0]+v[2]*math.Cos(a), v[1]+v[2]*math.Sin(a)))
		}
		ring(r)
	case "CT", "CE":
		pkg.contour = nil
	case "OB":
		if len(w) < 4 {
			return true, fmt.Errorf("OB needs x y and I|H")
		}
		p, err := coordPair(w[1], w[2], u)
		if err != nil {
			return true, err
		}
		pkg.contour = &pkgContour{hole: strings.EqualFold(w[3], "H"), pts: []pt{p}}
	case "OS":
		if pkg.contour == nil || len(w) < 3 {
			return true, fmt.Errorf("OS outside a polygon")
		}
		p, err := coordPair(w[1], w[2], u)
		if err != nil {
			return true, err
		}
		pkg.contour.pts = append(pkg.contour.pts, p)
	case "OC":
		if pkg.contour == nil || len(w) < 6 {
			return true, fmt.Errorf("OC outside a polygon")
		}
		e, err := coordPair(w[1], w[2], u)
		if err != nil {
			return true, err
		}
		c, err := coordPair(w[3], w[4], u)
		if err != nil {
			return true, err
		}
		pts := pkg.contour.pts
		arc := flattenArc(pts[len(pts)-1], e, c, strings.EqualFold(w[5], "Y"))
		pkg.contour.pts = append(pts, arc[1:]...)
	case "OE":
		if pkg.contour == nil {
			return true, fmt.Errorf("OE outside a polygon")
		}
		if !pkg.contour.hole {
			ring(pkg.contour.pts)
		}
		pkg.contour = nil
	default:
		return false, nil
	}
	return true, nil
}

// pkgContour is the polygon an OB record opens, until its OE.
type pkgContour struct {
	hole bool
	pts  []pt
}
