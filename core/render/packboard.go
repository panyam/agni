package render

import (
	"math"

	"github.com/panyam/agni/core/num"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// primTriangles is the filled-area primitive kind (WS7-035), a vertex range drawn as a
// triangle list. Boards need it because copper, pads and via barrels are areas and browser GL
// line width is effectively 1px, but nothing in the kind is board-specific.
const primTriangles uint8 = 4

// Board primitive groups (WS7-035). Boards reuse the PackedSheet ENVELOPE unchanged and add
// only these group constants and primTriangles. The TS mirror is web/src/packed.ts, and the
// group-indexed colors come from Style (C12).
const (
	groupBoardEdge        uint8 = 5  // outline (Edge.Cuts)
	groupBoardCopperFront uint8 = 6  // F.Cu segments, front SMD pads, front zone outlines
	groupBoardCopperBack  uint8 = 7  // B.Cu likewise
	groupBoardCopperInner uint8 = 8  // an inner layer past the group limit (maxBoardGroups)
	groupBoardThrough     uint8 = 9  // via barrels + through-hole pad lands
	groupBoardHole        uint8 = 10 // drill fills, page-colored, drawn last so they punch out
	groupBoardSilk        uint8 = 11 // a silkscreen or courtyard layer past the group limit
	groupBoardFab         uint8 = 12 // a fab layer past the group limit (agni issue 1366)
	// groupBoardPickFront and groupBoardPickBack hold each footprint's pick area (footprintBody),
	// keyed by its ref-des and split by the side the part sits on. No renderer draws them and no
	// highlight lights them. A WebGL click reads them, as an SVG click reads BoardSVG's invisible
	// picks-front and picks-back rects, so a click between a part's pads picks the part (agni issue
	// 1332). The outer copper rows name them, so a part hides with its side's copper. They are the
	// last two u8s, and planBoardStrata hands out groups below them (maxBoardGroups).
	groupBoardPickBack  uint8 = 254
	groupBoardPickFront uint8 = 255

	// boardGroupCount is the first group planBoardStrata hands out. Each inner copper layer, each
	// copper layer's zones and each graphics layer gets a group from there up (agni 1351), and
	// BoardLayerRow.groups names them.
	boardGroupCount = 13
)

// viaSegments tessellates round barrels, drills and circle pads. It is fixed in the packer
// rather than the shader so every consumer sees identical geometry.
const viaSegments = 16

// PackBoard projects the board sidecar into the tier-2 packed form (WS7-035), the board
// analogue of PackSheet in the same envelope. Copper is keyed by net and pads by (ref_des, pad
// number, net), so HighlightPacked and the web's resolveHighlights join unchanged. The
// front/back/inner strata are distinct groups, so a renderer hides a layer by skipping its group.
// See docsite/content/architecture/geometry-and-rendering.md#board-geometry-sidecar.
func PackBoard(b *geom.BoardGeometry, opts ...Option) *geom.PackedSheet {
	style := resolveStyle(opts)
	strata := planBoardStrata(b, style)
	stacks := padstacksOf(b)
	planes := planesOf(b)

	var c primCollector
	add := c.Add

	// The board packs in BoardSVG's paint order, so WebGL stacks the layers as SVG does, and a later
	// primitive is drawn over an earlier one. The canvas picker reads that order to choose between two
	// nets under one click (agni issue 1332). The order is zones, pick areas, each copper layer from
	// the back (pours, tracks, pads, via lands), through copper, the edge, then the artwork.
	front, back, inner := splitByLayer(func(z *geom.Zone) string { return z.GetLayer() }, b.GetZones())
	for _, zones := range [][]*geom.Zone{back, inner, front} {
		for _, z := range zones {
			if z.GetOutline() == nil {
				continue
			}
			add(primLineLoop, strata.zoneGroup(z.GetLayer()), "", z.GetNet(), "", xy(z.GetOutline().Points))
		}
	}

	// Each footprint's pick area, as BoardSVG's drawFootprintPicks draws it, above the zones and
	// under the copper. A placement with no pads or no ref-des gets none, and its silkscreen names
	// it alone.
	for _, pl := range b.GetPlacements() {
		if pl.GetRefDes() == "" || len(pl.GetPads()) == 0 {
			continue
		}
		group := groupBoardPickFront
		if placementSide(pl) == "back" {
			group = groupBoardPickBack
		}
		minX, minY, maxX, maxY := footprintBody(pl)
		add(primLineLoop, group, pl.GetRefDes(), "", "", [][2]int64{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}})
	}

	for _, ly := range copperLayersBackToFront(b) {
		// Filled copper as triangles, with its holes left out (agni 1357). The packer
		// triangulates once, so every renderer draws the same fill.
		for _, nc := range b.GetNets() {
			for _, sh := range nc.GetShapes() {
				if sh.GetLayer() != ly {
					continue
				}
				if tris := shapeTris(sh); len(tris) > 0 {
					add(primTriangles, strata.pourGroup(ly), "", nc.GetNet(), "", tris)
				}
			}
		}
		for _, nc := range b.GetNets() {
			for _, s := range nc.GetSegments() {
				if s.GetLayer() != ly || s.GetA() == nil || s.GetB() == nil {
					continue
				}
				// An arc goes as the quads of its chords, one record, so a pick or highlight on the net
				// takes the whole curve.
				var tris [][2]int64
				for i, pts := 1, geomath.TrackPath(s); i < len(pts); i++ {
					tris = append(tris, quadPts(pts[i-1].X, pts[i-1].Y, pts[i].X, pts[i].Y, s.GetWidth())...)
				}
				add(primTriangles, strata.copperGroup(ly), "", nc.GetNet(), "", tris)
			}
		}
		// A pad with a padstack packs its land into each layer's group (agni 1356). A pad without
		// one lives on its side's outer layer, and a through-hole pad packs once, below.
		for _, pl := range b.GetPlacements() {
			for _, pad := range pl.GetPads() {
				if st := stacks.of(pad.GetPadstack()); st != nil {
					if l := st.on(ly); l != nil {
						packStackLayer(add, strata.copperGroup(ly), pl, pad, l, planes.cuts(ly, pad.GetNet()), pl.GetRefDes(), pad.GetNet(), pad.GetNumber())
					}
					continue
				}
				switch side := padSide(pad); {
				case side == "front" && side == layerSide(ly):
					packPad(add, groupBoardCopperFront, pl, pad)
				case side == "back" && side == layerSide(ly):
					packPad(add, groupBoardCopperBack, pl, pad)
				}
			}
		}
		for _, nc := range b.GetNets() {
			for _, v := range nc.GetVias() {
				if st := stacks.of(v.GetPadstack()); st != nil && v.GetAt() != nil {
					if l := st.on(ly); l != nil {
						packStackLayer(add, strata.copperGroup(ly), &geom.ComponentPlacement{}, &geom.Pad{At: v.GetAt()}, l, planes.cuts(ly, nc.GetNet()), "", nc.GetNet(), "")
					}
				}
			}
		}
	}

	// Through copper, visible from both sides: via barrels, through-hole pads and every drill hole.
	for _, nc := range b.GetNets() {
		for _, v := range nc.GetVias() {
			if v.GetAt() == nil {
				continue
			}
			// A via with a padstack has its lands on their layers, so only its hole is here.
			if stacks.of(v.GetPadstack()) == nil {
				add(primTriangles, groupBoardThrough, "", nc.GetNet(), "", diskPts(v.GetAt().X, v.GetAt().Y, float64(v.GetSize())/2))
			}
			if v.GetDrill() > 0 {
				add(primTriangles, groupBoardHole, "", "", "", diskPts(v.GetAt().X, v.GetAt().Y, float64(v.GetDrill())/2))
			}
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if stacks.of(pad.GetPadstack()) == nil && padSide(pad) == "through" {
				packPad(add, groupBoardThrough, pl, pad)
			}
			if pad.GetDrill() > 0 {
				wx, wy := padWorld(pl, pad)
				add(primTriangles, groupBoardHole, "", "", "", diskPts(wx, wy, float64(pad.GetDrill())/2))
			}
		}
	}

	for _, p := range b.GetOutline().GetPaths() {
		add(primLineStrip, groupBoardEdge, "", "", "", xy(p.Points))
	}

	// Silkscreen and fab body graphics go through the schematic packer's shapeVerts, keyed by
	// ref_des so a component highlight picks them.
	for _, gr := range b.GetGraphics() {
		if gr.GetShape() == nil {
			continue
		}
		kind, pts := shapeVerts(gr.GetShape())
		if len(pts) == 0 {
			continue
		}
		add(kind, strata.artGroup(gr.GetLayer()), gr.GetRefDes(), "", "", pts)
	}

	vertices, records, keys, ox, oy := c.build()
	// A board net keys its schematic id beside its name where the caller gave one (WithNetIDs), so
	// a highlight naming the net by id lights it here as on a schematic (agni issue 1440).
	netID := boardNetIDs(b, style)
	for _, k := range keys {
		if k.GetNet() != "" {
			k.NetId = netID(k.GetNet())
		}
	}

	// Silkscreen and legend labels arrive in board coordinates from the reader, so they only
	// rebase to the vertex origin here. The overlay draws the glyphs.
	var labels []*geom.PackedLabel
	for _, t := range boardLabels(b) {
		if t.GetAt() == nil {
			continue
		}
		labels = append(labels, &geom.PackedLabel{
			X: num.Int32(t.GetAt().X - ox), Y: num.Int32(t.GetAt().Y - oy),
			Text: boardTextLine(t), Height: num.Int32(boardTextEm(t)),
			RotationDeg: int32(math.Round(t.GetRotationDeg())),
			Justify:     justifyWord(t), Color: boardTextColor(style, t.GetLayer()),
			Group: uint32(strata.art[t.GetLayer()]),
		})
	}

	return &geom.PackedSheet{
		SheetId:         "board",
		LayoutVersion:   packLayoutVersion,
		FontFamily:      style.Font,
		GroupColors:     strata.colors,
		BackgroundColor: style.Page,
		OriginX:         ox,
		OriginY:         oy,
		Vertices:        vertices,
		Primitives:      records,
		Keys:            keys,
		Labels:          labels,
	}
}

// packPad packs a pad that has no padstack, a disk or a turned rectangle, keyed as its part's pin.
func packPad(add func(kind, group uint8, refDes, net, pin string, pts [][2]int64), group uint8, pl *geom.ComponentPlacement, pad *geom.Pad) {
	wx, wy := padWorld(pl, pad)
	if pad.GetShape() == "circle" {
		add(primTriangles, group, pl.GetRefDes(), pad.GetNet(), pad.GetNumber(), diskPts(wx, wy, float64(pad.GetSize().GetX())/2))
		return
	}
	add(primTriangles, group, pl.GetRefDes(), pad.GetNet(), pad.GetNumber(), padQuadPts(wx, wy, pad.GetSize(), geomath.PadAngle(pl, pad)))
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
	colors[groupBoardFab] = style.Fab
	return colors
}

// shapeTris triangulates a copper shape, its outline less its holes, by earcut. The vertices are
// relative to the shape's first point while earcut runs, so float64 keeps them exact on a board
// metres across.
func shapeTris(sh *geom.CopperShape) [][2]int64 {
	pts := sh.GetOutline().GetPoints()
	if len(pts) < 3 {
		return nil
	}
	ox, oy := pts[0].GetX(), pts[0].GetY()
	var flat []float64
	var all []*geom.Point
	addRing := func(ring []*geom.Point) {
		// A ring that repeats its first point at the end closes on itself, which earcut does not want.
		if n := len(ring); n > 1 && ring[0].GetX() == ring[n-1].GetX() && ring[0].GetY() == ring[n-1].GetY() {
			ring = ring[:n-1]
		}
		for _, p := range ring {
			flat = append(flat, float64(p.GetX()-ox), float64(p.GetY()-oy))
			all = append(all, p)
		}
	}
	addRing(pts)
	var holes []int
	for _, h := range sh.GetHoles() {
		if len(h.GetPoints()) < 3 {
			continue
		}
		holes = append(holes, len(all))
		addRing(h.GetPoints())
	}
	idx := geomath.Earcut(flat, holes)
	out := make([][2]int64, 0, len(idx))
	for _, i := range idx {
		out = append(out, [2]int64{all[i].GetX(), all[i].GetY()})
	}
	return out
}

// packStackLayer packs a padstack's land on one layer as filled triangles, keyed as the pad or via
// is, and its antipad as an outline where cut says the layer clears around it. A land sits at its
// offset and a custom one fills its outline (agni 1407), placed as landAt places it.
func packStackLayer(add func(kind, group uint8, ref, net, pin string, pts [][2]int64), group uint8, pl *geom.ComponentPlacement, pad *geom.Pad, l *geom.PadstackLayer, cut bool, ref, net, pin string) {
	deg := geomath.PadAngle(pl, pad)
	if a := l.GetAntipad(); cut && a != nil {
		cx, cy, ring := landAt(pl, pad, a)
		if ring == nil {
			ring = outlinePts(cx, cy, a.GetShape(), a.GetSize(), deg)
		}
		add(primLineLoop, group, "", "", "", xy(ring))
	}
	if p := l.GetPad(); p != nil {
		cx, cy, ring := landAt(pl, pad, p)
		switch {
		case ring != nil:
			add(primTriangles, group, ref, net, pin, shapeTris(&geom.CopperShape{Outline: &geom.Polyline{Points: ring}}))
		case p.GetShape() == "circle":
			add(primTriangles, group, ref, net, pin, diskPts(cx, cy, float64(p.GetSize().GetX())/2))
		default:
			add(primTriangles, group, ref, net, pin, padQuadPts(cx, cy, p.GetSize(), deg))
		}
	}
}

// outlinePts is a pad shape's outline as a closed loop, a circle for a circle and the rectangle of
// its extent, turned deg degrees CCW, otherwise.
func outlinePts(cx, cy int64, shape string, size *geom.Point, deg float64) []*geom.Point {
	if shape == "circle" {
		r := float64(size.GetX()) / 2
		pts := make([]*geom.Point, 0, viaSegments)
		for i := 0; i < viaSegments; i++ {
			a := 2 * math.Pi * float64(i) / viaSegments
			pts = append(pts, &geom.Point{X: cx + int64(r*math.Cos(a)), Y: cy + int64(r*math.Sin(a))})
		}
		return pts
	}
	q := padQuadPts(cx, cy, size, deg)
	if len(q) < 6 {
		return nil
	}
	var out []*geom.Point
	for _, i := range []int{0, 1, 2, 4} {
		out = append(out, &geom.Point{X: q[i][0], Y: q[i][1]})
	}
	return out
}

// quadPts returns a track segment as a filled quad (two triangles) at true width, floored to
// minStrokeNm. The SVG backend's copperStrokePx uses the same floor, so both draw copper alike.
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

// padQuadPts is a rect, roundrect or oval pad's extent as two triangles, turned deg degrees CCW on
// the board (geomath.PadAngle).
func padQuadPts(cx, cy int64, size *geom.Point, deg float64) [][2]int64 {
	w, h := float64(size.GetX())/2, float64(size.GetY())/2
	if w <= 0 || h <= 0 {
		return nil
	}
	sin, cos := math.Sincos(deg * math.Pi / 180)
	rot := func(x, y float64) [2]int64 {
		return [2]int64{cx + int64(math.Round(cos*x-sin*y)), cy + int64(math.Round(sin*x+cos*y))}
	}
	c1, c2, c3, c4 := rot(-w, -h), rot(w, -h), rot(w, h), rot(-w, h)
	return [][2]int64{c1, c2, c3, c3, c4, c1}
}
