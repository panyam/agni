package render

import (
	"math"
	"sort"

	"github.com/panyam/agni/core/stackup"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// BoardStack is the board's physical layer stack as the viewer's layer list shows it (agni 1603).
// It returns nil for a board whose file states no stack, so the list shows no stack section. The
// rows take their colours from the style in opts.
func BoardStack(b *geom.BoardGeometry, opts ...Option) *geom.BoardStackView {
	s := b.GetStackup()
	if len(s.GetLayers()) == 0 {
		return nil
	}
	unit := unitNm(b)
	style := resolveStyle(opts)
	copperColor := copperColors(style, b)
	v := &geom.BoardStackView{UnitsPerMm: 1e6 / float64(unit)}
	for _, l := range s.GetLayers() {
		row := &geom.BoardStackRow{Name: l.GetName(), Kind: l.GetKind(), Layer: l.GetLayer(),
			Thickness: thicknessText(l, unit), Material: l.GetMaterial(), Color: stackColor(style, copperColor, l)}
		drawn, defaulted := TypicalMm[l.GetKind()], true
		if l.Thickness != nil {
			mm := float64(l.GetThickness()*unit) / 1e6
			row.ThicknessMm = &mm
			if mm > 0 {
				drawn, defaulted = mm, false
			}
		}
		row.TopMm, row.BottomMm, row.ThicknessDefaulted = v.TotalMm, toNm(v.TotalMm+drawn), defaulted
		v.TotalMm = row.BottomMm
		v.Rows = append(v.Rows, row)
	}
	v.Summary, v.Disagrees = stackSummary(s, unit)
	v.Drills = drillSpans(b)
	spanDepths(v)
	v.OutlineTris = outlineTris(b)
	v.Bodies = partBodies(b, style)
	v.BodyColor, v.DefaultBodyColor = style.PartBody, style.PartBodyDefault
	v.BoardLook = &geom.BoardLook{Mask: style.BoardMask, MaskOverCopper: style.MaskOverCopper, PadFinish: style.PadFinish, Silk: style.BoardSilk}
	return v
}

// TypicalMm is the thickness a stack row of each kind draws at when the file gives none, in
// millimetres (agni issues 1593 and 1635). BoardStackRow.thickness_defaulted marks such a row.
// Silkscreen and paste draw on the surface, at 0, since a board's stated thickness leaves them out.
// A kind not listed draws at 0. KiCad gives silk and paste no thickness, and the Jetson cut's 28
// rows then sum to its stated 1.711 mm.
var TypicalMm = map[string]float64{"copper": 0.035, "dielectric": 0.1, "mask": 0.01, "paste": 0, "silk": 0}

// toNm rounds a depth in millimetres to the nanometre, so a sum of thicknesses reads as the depth it
// is rather than carrying floating-point noise onto the wire.
func toNm(mm float64) float64 { return math.Round(mm*1e6) / 1e6 }

// spanDepths sets the depths each drill span runs between, from the top of its first layer's stack
// row to the bottom of its last's. A span whose layer the stack names nowhere keeps neither.
func spanDepths(v *geom.BoardStackView) {
	row := map[string]*geom.BoardStackRow{}
	for _, r := range v.GetRows() {
		if r.GetLayer() != "" && row[r.GetLayer()] == nil {
			row[r.GetLayer()] = r
		}
	}
	for _, sp := range v.GetDrills() {
		a, b := row[sp.GetLayerFrom()], row[sp.GetLayerTo()]
		if a == nil || b == nil {
			continue
		}
		top, bottom := math.Min(a.GetTopMm(), b.GetTopMm()), math.Max(a.GetBottomMm(), b.GetBottomMm())
		sp.TopMm, sp.BottomMm = &top, &bottom
	}
}

// drillSpans groups the board's plated holes by the copper layers they join, in the order the board
// first names each pair (agni 1593). A via names its own pair, and one that names none joins the
// stack's first and last copper, as a through-hole pad does. A hole with no drill size takes its
// via's size.
func drillSpans(b *geom.BoardGeometry) []*geom.BoardDrillSpan {
	top, bottom := copperEnds(b.GetStackup())
	var spans []*geom.BoardDrillSpan
	at := map[[2]string]*geom.BoardDrillSpan{}
	add := func(from, to string, x, y, width int64, net string) {
		if from == "" || to == "" {
			from, to = top, bottom
		}
		key := [2]string{from, to}
		sp := at[key]
		if sp == nil {
			sp = &geom.BoardDrillSpan{LayerFrom: from, LayerTo: to}
			at[key] = sp
			spans = append(spans, sp)
		}
		sp.Xy = append(sp.Xy, float64(x), float64(y))
		sp.Diameter = append(sp.Diameter, float64(width))
		sp.Net = append(sp.Net, net)
	}
	for _, nc := range b.GetNets() {
		for _, v := range nc.GetVias() {
			if v.GetAt() == nil {
				continue
			}
			width := v.GetDrill()
			if width <= 0 {
				width = v.GetSize()
			}
			add(v.GetLayerFrom(), v.GetLayerTo(), v.GetAt().X, v.GetAt().Y, width, nc.GetNet())
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if pad.GetDrill() > 0 {
				x, y := padWorld(pl, pad)
				add("", "", x, y, pad.GetDrill(), pad.GetNet())
			}
		}
	}
	return spans
}

// copperEnds names the stack's first and last copper layers, which a through hole joins.
func copperEnds(s *geom.Stackup) (top, bottom string) {
	for _, l := range s.GetLayers() {
		if l.GetKind() == "copper" && l.GetLayer() != "" {
			if top == "" {
				top = l.GetLayer()
			}
			bottom = l.GetLayer()
		}
	}
	return top, bottom
}

// outlineTris fills the board outline as a triangle list, flattened to x,y pairs. Sources draw the
// edge as separate lines and arcs, so outlineRings chains them first. The largest ring is the board,
// and a ring inside it is a cutout, as a slot is. A drilled hole of HoleCutMm or wider is cut too, as
// a mounting hole is (agni 1607). An outline that does not close fills its bounding box instead, and
// a board with no outline fills nothing.
func outlineTris(b *geom.BoardGeometry) []float64 {
	var pts []*geom.Point
	var holes [][]*geom.Point
	if rings := outlineRings(b.GetOutline(), outlineGap(b), outlineReach(b)); len(rings) > 0 {
		pts = rings[0]
		for _, r := range rings[1:] {
			if insideRing(pts, r[0]) {
				holes = append(holes, r)
			}
		}
	} else if box := outlineBox(b.GetOutline()); box != nil {
		pts = box
	}
	if len(pts) >= 3 {
		holes = append(holes, drillHoles(b, pts)...)
	}
	var out []float64
	for _, p := range polygonTris(pts, holes) {
		out = append(out, float64(p[0]), float64(p[1]))
	}
	return out
}

// HoleCutMm is the narrowest drill the stack view cuts out of the board, in millimetres. A mounting
// hole is 2 mm or more and a via well under 1 mm, so the cut shows the holes a reader looks for and
// keeps Jetson's thousands of vias out of the triangulation.
const HoleCutMm = 1.0

// drillHoles is a ring for each drilled pad at least HoleCutMm wide whose centre sits inside the
// board, one per centre.
func drillHoles(b *geom.BoardGeometry, board []*geom.Point) [][]*geom.Point {
	least := int64(HoleCutMm * 1e6 / float64(unitNm(b)))
	seen := map[[2]int64]bool{}
	var out [][]*geom.Point
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if pad.GetDrill() < least {
				continue
			}
			x, y := padWorld(pl, pad)
			c := &geom.Point{X: x, Y: y}
			if seen[[2]int64{x, y}] || !insideRing(board, c) {
				continue
			}
			seen[[2]int64{x, y}] = true
			r := float64(pad.GetDrill()) / 2
			ring := make([]*geom.Point, 0, holeSides)
			for i := range holeSides {
				a := 2 * math.Pi * float64(i) / holeSides
				ring = append(ring, &geom.Point{X: x + int64(math.Round(r*math.Cos(a))), Y: y + int64(math.Round(r*math.Sin(a)))})
			}
			out = append(out, ring)
		}
	}
	return out
}

// holeSides is how many sides a cut hole has.
const holeSides = 16

// insideRing says whether p sits inside the ring, by the crossings of a ray to its right.
func insideRing(ring []*geom.Point, p *geom.Point) bool {
	in := false
	x, y := float64(p.GetX()), float64(p.GetY())
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi := float64(ring[i].GetX()), float64(ring[i].GetY())
		xj, yj := float64(ring[j].GetX()), float64(ring[j].GetY())
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// outlineReach is how far apart two path ends can sit and still join when no end lies within
// outlineGap, a quarter millimetre in the board's unit.
func outlineReach(b *geom.BoardGeometry) int64 {
	return max(250_000/unitNm(b), 1)
}

// outlineGap is how far apart two path ends can sit and still join, ten micrometres in the board's
// unit. Readers round arcs to polylines, so a joint rarely matches to the nanometre.
func outlineGap(b *geom.BoardGeometry) int64 {
	return max(10_000/unitNm(b), 1)
}

// outlineRings chains the outline's paths end to end into closed rings, the largest first. A path
// joins the chain at either end, reversed if it must be, where its end lies within gap of the chain's.
// When none does, the nearest end within reach joins instead, as Allegro's parasdr runs an edge
// 0.1 mm past the corner arc it meets (agni 1669). A chain closes when its ends lie within gap. It
// returns nil when no chain closes.
func outlineRings(o *geom.BoardOutline, gap, reach int64) [][]*geom.Point {
	dist := func(a, b *geom.Point) int64 {
		return max(absUnits(a.GetX()-b.GetX()), absUnits(a.GetY()-b.GetY()))
	}
	var paths [][]*geom.Point
	for _, p := range o.GetPaths() {
		if len(p.GetPoints()) >= 2 {
			paths = append(paths, p.GetPoints())
		}
	}
	used := make([]bool, len(paths))
	// next is the unused path whose end lies nearest end, within limit, and whether to reverse it.
	next := func(end *geom.Point, limit int64) (int, bool) {
		best, bestD, rev := -1, limit+1, false
		for j := range paths {
			if used[j] {
				continue
			}
			q := paths[j]
			if d := dist(end, q[0]); d < bestD {
				best, bestD, rev = j, d, false
			}
			if d := dist(end, q[len(q)-1]); d < bestD {
				best, bestD, rev = j, d, true
			}
		}
		return best, rev
	}
	var rings [][]*geom.Point
	for i := range paths {
		if used[i] {
			continue
		}
		used[i] = true
		ring := append([]*geom.Point{}, paths[i]...)
		for dist(ring[0], ring[len(ring)-1]) > gap || len(ring) < 3 {
			j, rev := next(ring[len(ring)-1], gap)
			if j < 0 {
				j, rev = next(ring[len(ring)-1], reach)
			}
			if j < 0 {
				break
			}
			q := paths[j]
			if rev {
				for k := len(q) - 2; k >= 0; k-- {
					ring = append(ring, q[k])
				}
			} else {
				ring = append(ring, q[1:]...)
			}
			used[j] = true
		}
		if len(ring) < 3 || dist(ring[0], ring[len(ring)-1]) > gap {
			continue
		}
		rings = append(rings, ring)
	}
	sort.SliceStable(rings, func(i, j int) bool { return math.Abs(ringArea(rings[i])) > math.Abs(ringArea(rings[j])) })
	return rings
}

func ringArea(ring []*geom.Point) float64 {
	var a float64
	for i := range ring {
		p, q := ring[i], ring[(i+1)%len(ring)]
		a += float64(p.GetX())*float64(q.GetY()) - float64(q.GetX())*float64(p.GetY())
	}
	return a / 2
}

// outlineBox is the outline's bounding box as a ring, or nil when the outline has no points.
func outlineBox(o *geom.BoardOutline) []*geom.Point {
	first := true
	var x0, y0, x1, y1 int64
	for _, p := range o.GetPaths() {
		for _, pt := range p.GetPoints() {
			if first {
				x0, y0, x1, y1, first = pt.X, pt.Y, pt.X, pt.Y, false
				continue
			}
			x0, y0, x1, y1 = min(x0, pt.X), min(y0, pt.Y), max(x1, pt.X), max(y1, pt.Y)
		}
	}
	if first || x0 == x1 || y0 == y1 {
		return nil
	}
	return []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func absUnits(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// stackColor is the colour a stack row draws in.
func stackColor(style Style, copperColor func(string) string, l *geom.StackLayer) string {
	switch l.GetKind() {
	case "copper":
		return copperColor(l.GetLayer())
	case "dielectric":
		return style.Dielectric
	case "mask":
		return style.SolderMask
	case "silk":
		return boardArtColor(style, l.GetLayer())
	}
	return ""
}

// stackSummary compares the stack with the board thickness the file states, in words.
func stackSummary(s *geom.Stackup, unit int64) (string, bool) {
	if c := stackup.Check(s, unit); c != "" {
		return c, true
	}
	sum, missing := stackup.Sum(s)
	stated := s.Thickness != nil && s.GetThickness() > 0
	switch {
	case stated && missing == 0:
		return stackup.MM(s.GetThickness(), unit) + " mm, as the file states", false
	case stated:
		return "the file states a board thickness of " + stackup.MM(s.GetThickness(), unit) +
			" mm, and some layers give no thickness to sum", false
	case missing == 0 && sum > 0:
		return "the layers sum to " + stackup.MM(sum, unit) + " mm, and the file states no board thickness", false
	}
	return "", false
}

// stackThickness gives each layer row the thickness of the stack row whose layer names it. A row
// no stack row names keeps "", so Edge, Through and Zones say nothing.
func stackThickness(b *geom.BoardGeometry, rows []*geom.BoardLayerRow) {
	unit := unitNm(b)
	byLayer := map[string]*geom.StackLayer{}
	for _, l := range b.GetStackup().GetLayers() {
		if l.GetLayer() != "" {
			if _, ok := byLayer[l.GetLayer()]; !ok {
				byLayer[l.GetLayer()] = l
			}
		}
	}
	for _, r := range rows {
		if l, ok := byLayer[r.GetName()]; ok {
			r.Thickness = thicknessText(l, unit)
		}
	}
}

func thicknessText(l *geom.StackLayer, unit int64) string {
	if l.Thickness == nil {
		return "thickness unknown"
	}
	return stackup.MM(l.GetThickness(), unit) + " mm"
}

// unitNm is the board's unit, with KiCad's plain nanometres when the board names none.
func unitNm(b *geom.BoardGeometry) int64 {
	if u := b.GetUnitNm(); u > 0 {
		return u
	}
	return 1
}
