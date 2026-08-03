package render

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// primTriangles is the filled-area primitive kind (WS7-035): a vertex range drawn as a
// triangle list. Boards forced it into the packed vocabulary — copper at true width, pads,
// and via barrels are areas, and browser GL line width is effectively 1px — but the kind is
// generic; nothing in it is board-specific.
const primTriangles uint8 = 4

// Board primitive groups (WS7-035): the packed transport for boards reuses the PackedSheet
// ENVELOPE unchanged (int32 vertex pool, 12-byte records, keys, group_colors) — boards are
// new group constants and one new kind, exactly like the schematic groups. The TS mirror is
// web/src/packed.ts; the group-indexed colors come from Style (C12).
const (
	groupBoardEdge        uint8 = 5  // outline (Edge.Cuts)
	groupBoardCopperFront uint8 = 6  // F.Cu segments, front SMD pads, front zone outlines
	groupBoardCopperBack  uint8 = 7  // B.Cu likewise
	groupBoardCopperInner uint8 = 8  // any other signal layer
	groupBoardThrough     uint8 = 9  // via barrels + through-hole pad lands
	groupBoardHole        uint8 = 10 // drill fills, page-colored, drawn last so they punch out
	groupBoardSilk        uint8 = 11 // silkscreen / fab body-outline graphics (WS7)

	boardGroupCount = 12
)

// viaSegments tessellates round barrels/drills/circle pads; fixed in the packer (not the
// shader) so every consumer sees identical geometry.
const viaSegments = 16

// PackBoard projects the board sidecar into the tier-2 packed form (WS7-035) — the board
// analogue of PackSheet, sharing its envelope so the web canvas needs one new draw mode and
// zero new decode paths. Copper is keyed by net and pads by (ref_des, pad number, net), so
// resolveHighlights/HighlightPacked join unchanged. Layer visibility is group visibility:
// the front/back/inner strata are distinct groups a renderer can skip at draw time.
func PackBoard(b *geom.BoardGeometry, opts ...Option) *geom.PackedSheet {
	style := resolveStyle(opts)

	var c primCollector
	add := c.Add

	for _, p := range b.GetOutline().GetPaths() {
		add(primLineStrip, groupBoardEdge, "", "", "", xy(p.Points))
	}
	for _, z := range b.GetZones() {
		if z.GetOutline() == nil {
			continue
		}
		add(primLineLoop, copperGroup(z.GetLayer()), "", z.GetNet(), "", xy(z.GetOutline().Points))
	}
	for _, nc := range b.GetNets() {
		for _, s := range nc.GetSegments() {
			if s.GetA() == nil || s.GetB() == nil {
				continue
			}
			add(primTriangles, copperGroup(s.GetLayer()), "", nc.GetNet(), "",
				quadPts(s.GetA().X, s.GetA().Y, s.GetB().X, s.GetB().Y, s.GetWidth()))
		}
		for _, v := range nc.GetVias() {
			if v.GetAt() == nil {
				continue
			}
			add(primTriangles, groupBoardThrough, "", nc.GetNet(), "",
				diskPts(v.GetAt().X, v.GetAt().Y, float64(v.GetSize())/2))
			if v.GetDrill() > 0 {
				add(primTriangles, groupBoardHole, "", "", "",
					diskPts(v.GetAt().X, v.GetAt().Y, float64(v.GetDrill())/2))
			}
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			wx, wy := padWorld(pl, pad)
			group := groupBoardThrough
			switch padSide(pad) {
			case "front":
				group = groupBoardCopperFront
			case "back":
				group = groupBoardCopperBack
			}
			if pad.GetShape() == "circle" {
				add(primTriangles, group, pl.GetRefDes(), pad.GetNet(), pad.GetNumber(),
					diskPts(wx, wy, float64(pad.GetSize().GetX())/2))
			} else {
				add(primTriangles, group, pl.GetRefDes(), pad.GetNet(), pad.GetNumber(),
					padQuadPts(wx, wy, pad))
			}
			if pad.GetDrill() > 0 {
				add(primTriangles, groupBoardHole, "", "", "", diskPts(wx, wy, float64(pad.GetDrill())/2))
			}
		}
	}

	// Silkscreen / fab body graphics: outline primitives (the same shapeVerts the schematic
	// packer uses), keyed by ref_des so a component highlight can pick them.
	for _, gr := range b.GetGraphics() {
		if gr.GetShape() == nil {
			continue
		}
		kind, pts := shapeVerts(gr.GetShape())
		if len(pts) == 0 {
			continue
		}
		add(kind, groupBoardSilk, gr.GetRefDes(), "", "", pts)
	}

	vertices, records, keys, ox, oy := c.build()

	// Silkscreen / legend labels: the same board texts the SVG backend draws (ref-des,
	// value, title block), already composed to board coordinates by the reader. The overlay
	// draws the glyphs; here they only rebase to the vertex origin.
	var labels []*geom.PackedLabel
	for _, t := range b.GetTexts() {
		if t.GetAt() == nil {
			continue
		}
		labels = append(labels, &geom.PackedLabel{
			X: int32(t.GetAt().X - ox), Y: int32(t.GetAt().Y - oy),
			Text: boardTextLine(t), Height: int32(boardTextHeight(t)),
			RotationDeg: int32(math.Round(t.GetRotationDeg())),
			Justify:     justifyWord(t), Color: style.Field,
		})
	}

	return &geom.PackedSheet{
		SheetId:         "board",
		LayoutVersion:   packLayoutVersion,
		FontFamily:      style.Font,
		GroupColors:     boardGroupColors(style),
		BackgroundColor: style.Page,
		OriginX:         ox,
		OriginY:         oy,
		Vertices:        vertices,
		Primitives:      records,
		Keys:            keys,
		Labels:          labels,
	}
}

// boardGroupColors extends the schematic group palette with the board strata, indexed by
// the group constants so the mapping cannot drift.
func boardGroupColors(style Style) []string {
	colors := make([]string, boardGroupCount)
	colors[groupSymbol] = style.Symbol
	colors[groupWire] = style.Wire
	colors[groupPin] = style.Pin
	colors[groupFree] = style.Free
	colors[groupFrame] = style.Frame
	colors[groupBoardEdge] = style.BoardOutline
	colors[groupBoardCopperFront] = style.CopperFront
	colors[groupBoardCopperBack] = style.CopperBack
	colors[groupBoardCopperInner] = style.CopperInner
	colors[groupBoardThrough] = style.Via
	colors[groupBoardHole] = style.Page
	colors[groupBoardSilk] = style.Silk
	return colors
}

// copperGroup maps a layer name to its board group.
func copperGroup(layer string) uint8 {
	switch layerSide(layer) {
	case "front":
		return groupBoardCopperFront
	case "back":
		return groupBoardCopperBack
	default:
		return groupBoardCopperInner
	}
}

// quadPts is a track segment as a filled quad (two triangles) at true width, floored to the
// physical minStrokeNm minimum — the same floor the SVG backend's copperStrokePx applies, so
// both renderers draw copper at identical width.
func quadPts(ax, ay, bx, by, width int64) [][2]int64 {
	dx, dy := float64(bx-ax), float64(by-ay)
	l := math.Hypot(dx, dy)
	if l == 0 {
		return nil
	}
	w := math.Max(float64(width), minStrokeNm)
	hx, hy := -dy/l*w/2, dx/l*w/2
	a1 := [2]int64{ax + int64(hx), ay + int64(hy)}
	a2 := [2]int64{ax - int64(hx), ay - int64(hy)}
	b1 := [2]int64{bx + int64(hx), by + int64(hy)}
	b2 := [2]int64{bx - int64(hx), by - int64(hy)}
	return [][2]int64{a1, a2, b1, b1, a2, b2}
}

// diskPts is a filled circle as a triangle fan flattened to a list.
func diskPts(cx, cy int64, r float64) [][2]int64 {
	if r <= 0 {
		return nil
	}
	pts := make([][2]int64, 0, viaSegments*3)
	for i := 0; i < viaSegments; i++ {
		a0 := 2 * math.Pi * float64(i) / viaSegments
		a1 := 2 * math.Pi * float64(i+1) / viaSegments
		pts = append(pts,
			[2]int64{cx, cy},
			[2]int64{cx + int64(r*math.Cos(a0)), cy + int64(r*math.Sin(a0))},
			[2]int64{cx + int64(r*math.Cos(a1)), cy + int64(r*math.Sin(a1))})
	}
	return pts
}

// padQuadPts is a rectangular-ish pad (rect/roundrect/oval at tier 2) as two triangles,
// rotated by the pad's own verbatim angle (negated for the Y-flip; source-cumulative, so
// not composed with the placement again — same rule as drawPad).
func padQuadPts(cx, cy int64, pad *geom.Pad) [][2]int64 {
	w, h := float64(pad.GetSize().GetX())/2, float64(pad.GetSize().GetY())/2
	if w <= 0 || h <= 0 {
		return nil
	}
	rad := -pad.GetRotationDeg() * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	rot := func(x, y float64) [2]int64 {
		return [2]int64{cx + int64(cos*x-sin*y), cy + int64(sin*x+cos*y)}
	}
	c1, c2, c3, c4 := rot(-w, -h), rot(w, -h), rot(w, h), rot(-w, h)
	return [][2]int64{c1, c2, c3, c3, c4, c1}
}
