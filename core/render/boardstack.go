package render

import (
	"math"

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
		if l.Thickness != nil {
			mm := float64(l.GetThickness()*unit) / 1e6
			row.ThicknessMm = &mm
		}
		v.Rows = append(v.Rows, row)
	}
	v.Summary, v.Disagrees = stackSummary(s, unit)
	v.Drills = drillSpans(b)
	v.OutlineTris = outlineTris(b)
	v.Bodies = partBodies(b, style)
	v.BodyColor, v.DefaultBodyColor = style.PartBody, style.PartBodyDefault
	v.BoardLook = &geom.BoardLook{Mask: style.BoardMask, MaskOverCopper: style.MaskOverCopper, PadFinish: style.PadFinish, Silk: style.BoardSilk}
	return v
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
// edge as separate lines and arcs, so outlineRing chains them first. An outline that does not close
// fills its bounding box instead, and a board with no outline fills nothing.
func outlineTris(b *geom.BoardGeometry) []float64 {
	var pts []*geom.Point
	if ring := outlineRing(b.GetOutline(), outlineGap(b)); len(ring) >= 3 {
		pts = ring
	} else if box := outlineBox(b.GetOutline()); box != nil {
		pts = box
	}
	var out []float64
	for _, p := range polygonTris(pts, nil) {
		out = append(out, float64(p[0]), float64(p[1]))
	}
	return out
}

// outlineGap is how far apart two path ends can sit and still join, ten micrometres in the board's
// unit. Readers round arcs to polylines, so a joint rarely matches to the nanometre.
func outlineGap(b *geom.BoardGeometry) int64 {
	return max(10_000/unitNm(b), 1)
}

// outlineRing chains the outline's paths end to end into the closed ring with the largest area. A
// path joins the chain at either end, reversed if it must be. It returns nil when no chain closes.
func outlineRing(o *geom.BoardOutline, gap int64) []*geom.Point {
	near := func(a, b *geom.Point) bool {
		return absUnits(a.GetX()-b.GetX()) <= gap && absUnits(a.GetY()-b.GetY()) <= gap
	}
	var paths [][]*geom.Point
	for _, p := range o.GetPaths() {
		if len(p.GetPoints()) >= 2 {
			paths = append(paths, p.GetPoints())
		}
	}
	used := make([]bool, len(paths))
	var best []*geom.Point
	bestArea := 0.0
	for i := range paths {
		if used[i] {
			continue
		}
		used[i] = true
		ring := append([]*geom.Point{}, paths[i]...)
		for !near(ring[0], ring[len(ring)-1]) || len(ring) < 3 {
			grew := false
			for j := range paths {
				if used[j] {
					continue
				}
				end, q := ring[len(ring)-1], paths[j]
				switch {
				case near(end, q[0]):
					ring = append(ring, q[1:]...)
				case near(end, q[len(q)-1]):
					for k := len(q) - 2; k >= 0; k-- {
						ring = append(ring, q[k])
					}
				default:
					continue
				}
				used[j], grew = true, true
				break
			}
			if !grew {
				break
			}
		}
		if len(ring) < 3 || !near(ring[0], ring[len(ring)-1]) {
			continue
		}
		if a := math.Abs(ringArea(ring)); a > bestArea {
			best, bestArea = ring, a
		}
	}
	return best
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
