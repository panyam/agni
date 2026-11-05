package orcad

import (
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// unitNm is a Capture unit, a hundredth of an inch, in nanometres.
const unitNm = 254000

// ReadGeometry parses an OrCAD Capture design's pages into schematic geometry, one sheet per page
// of every block it places. A block page placed more than once is drawn once, with the designators
// of its first placement. Wires carry the net names Read gives, so a sheet's nets join the netlist.
// open opens a block's design, as for Read.
//
// Capture draws with Y pointing down and the contract with Y up, so every coordinate's Y is negated
// and a quarter turn reads anticlockwise.
func ReadGeometry(r io.Reader, sourceFile string, open Opener) (*geom.SchematicGeometry, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	top, err := load(b, sourceFile)
	if err != nil {
		return nil, err
	}
	f := newFlattener(top, open)
	if err := f.run(top); err != nil {
		return nil, err
	}
	f.design(top) // names every net
	return f.geometry(top), nil
}

func gp(p point) *geom.Point { return &geom.Point{X: int64(p.x), Y: -int64(p.y)} }

// placeTransform is a symbol's transform for a placement at `at`, turned by orient, in a box of the
// given width and height. Capture turns a symbol inside its box. A quarter turn takes a connection
// point (x, y) to (y, w-x) from the box's corner, so the corner moves with each turn:
//
//	0: (x, y)   1: (y, w-x)   2: (w-x, h-y)   3: (h-y, x)
//
// A mirror (bit 2) flips x to w-x before the turn. No public design checks the mirror: Clay R6
// places none.
func placeTransform(at point, orient byte, w, h int32) *geom.Transform {
	turns := int(orient & 3)
	t := &geom.Transform{RotationDeg: int32(90 * turns)} // #nosec G115 -- at most 270
	ax, ay := int64(at.x), int64(at.y)
	W, H := int64(w), int64(h)
	switch turns {
	case 0:
		t.Origin = &geom.Point{X: ax, Y: -ay}
	case 1:
		t.Origin = &geom.Point{X: ax, Y: -(ay + W)}
	case 2:
		t.Origin = &geom.Point{X: ax + W, Y: -(ay + H)}
	case 3:
		t.Origin = &geom.Point{X: ax + H, Y: -ay}
	}
	if orient&4 != 0 {
		t.MirrorY = true
		// The flip is about the box's own x = w/2, so the turned width moves the origin.
		dx, dy := rotate(W, 0, turns)
		t.Origin.X += dx
		t.Origin.Y += dy
	}
	return t
}

func rotate(x, y int64, turns int) (int64, int64) {
	for i := 0; i < turns; i++ {
		x, y = -y, x
	}
	return x, y
}

// symbolShapes is a drawing's primitives in the contract's frame, with each pin's line.
func symbolShapes(d drawing, pins []symPin) ([]*geom.Shape, []*geom.Label) {
	var shapes []*geom.Shape
	var labels []*geom.Label
	for _, p := range d.prims {
		switch p.kind {
		case primLine, primPolyline, primBezier:
			s := &geom.Shape{Kind: geom.Shape_KIND_POLYLINE}
			for _, q := range p.pts {
				s.Points = append(s.Points, gp(q))
			}
			shapes = append(shapes, s)
		case primPolygon:
			s := &geom.Shape{Kind: geom.Shape_KIND_POLYLINE}
			for _, q := range p.pts {
				s.Points = append(s.Points, gp(q))
			}
			if len(p.pts) > 0 {
				s.Points = append(s.Points, gp(p.pts[0]))
			}
			if p.filled {
				s.Fill = geom.Shape_FILL_OUTLINE
			}
			shapes = append(shapes, s)
		case primRect:
			lo, hi := corners(p.pts[0], p.pts[1])
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{gp(point{lo.x, hi.y}), gp(point{hi.x, lo.y})}})
		case primEllipse:
			shapes = append(shapes, ellipse(p.pts[0], p.pts[1]))
		case primArc:
			if s := arc(p.pts); s != nil {
				shapes = append(shapes, s)
			}
		case primText:
			labels = append(labels, &geom.Label{Text: p.text, Origin: gp(p.pts[0]), Justify: "left bottom"})
		}
	}
	for _, p := range pins {
		if p.leg != p.hot {
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{gp(p.leg), gp(p.hot)}})
		}
	}
	return shapes, labels
}

func corners(a, b point) (point, point) {
	return point{min(a.x, b.x), min(a.y, b.y)}, point{max(a.x, b.x), max(a.y, b.y)}
}

// ellipse is a circle where the box is square, and a polyline of 32 chords where it is not.
func ellipse(a, b point) *geom.Shape {
	lo, hi := corners(a, b)
	cx, cy := float64(lo.x+hi.x)/2, float64(lo.y+hi.y)/2
	rx, ry := float64(hi.x-lo.x)/2, float64(hi.y-lo.y)/2
	if rx == ry {
		return &geom.Shape{Kind: geom.Shape_KIND_CIRCLE, Points: []*geom.Point{{X: round(cx), Y: -round(cy)}}, Radius: round(rx)}
	}
	s := &geom.Shape{Kind: geom.Shape_KIND_POLYLINE}
	for i := 0; i <= 32; i++ {
		t := 2 * math.Pi * float64(i) / 32
		s.Points = append(s.Points, &geom.Point{X: round(cx + rx*math.Cos(t)), Y: -round(cy + ry*math.Sin(t))})
	}
	return s
}

// arc is a circular arc from its start to its end through the middle of its sweep, which runs
// anticlockwise on the page. Capture stores the circle's box with the two ends, in the page's Y-down
// frame, so the sweep runs clockwise there.
func arc(pts []point) *geom.Shape {
	if len(pts) < 4 {
		return nil
	}
	lo, hi := corners(pts[0], pts[1])
	cx, cy := float64(lo.x+hi.x)/2, -float64(lo.y+hi.y)/2
	r := float64(hi.x-lo.x) / 2
	sx, sy := float64(pts[2].x), -float64(pts[2].y)
	ex, ey := float64(pts[3].x), -float64(pts[3].y)
	a0, a1 := math.Atan2(sy-cy, sx-cx), math.Atan2(ey-cy, ex-cx)
	for a1 <= a0 {
		a1 += 2 * math.Pi
	}
	m := (a0 + a1) / 2
	return &geom.Shape{Kind: geom.Shape_KIND_ARC, Points: []*geom.Point{
		{X: round(sx), Y: round(sy)},
		{X: round(cx + r*math.Cos(m)), Y: round(cy + r*math.Sin(m))},
		{X: round(ex), Y: round(ey)},
	}}
}

func round(v float64) int64 { return int64(math.Round(v)) }

// overbars rewrites Capture's overbar marking into KiCad's. Capture follows each barred character
// with a backslash, so C\S\ is CS under one bar, and KiCad writes that ~{CS}. A backslash after a
// character that is itself a backslash, or at the start, is kept as written.
func overbars(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); {
		j := i
		for j+1 < len(r) && r[j] != '\\' && r[j+1] == '\\' {
			j += 2
		}
		if j == i {
			b.WriteRune(r[i])
			i++
			continue
		}
		b.WriteString("~{")
		for k := i; k < j; k += 2 {
			b.WriteRune(r[k])
		}
		b.WriteString("}")
		i = j
	}
	return b.String()
}

// pinPoint is a symbol pin with its name inside the body, past the leg's inner end, and its number
// above the pin's line, as Capture draws them. A pin whose leg has no length draws neither.
func pinPoint(sp symPin, num string, height int64) *geom.PinPoint {
	pp := &geom.PinPoint{PortRef: num, Name: overbars(sp.name), Loc: gp(sp.hot), Height: height}
	dx, dy := sp.leg.x-sp.hot.x, sp.leg.y-sp.hot.y // from the connection point into the body
	if dx == 0 && dy == 0 {
		return pp
	}
	const gap = 2
	mid := point{(sp.leg.x + sp.hot.x) / 2, (sp.leg.y + sp.hot.y) / 2}
	switch {
	case dx > 0: // the pin enters from the left
		pp.LabelOrigin, pp.Justify = gp(point{sp.leg.x + gap, sp.leg.y}), "left middle"
		pp.NumberOrigin, pp.NumberJustify = gp(point{mid.x, mid.y - 1}), "center bottom"
	case dx < 0:
		pp.LabelOrigin, pp.Justify = gp(point{sp.leg.x - gap, sp.leg.y}), "right middle"
		pp.NumberOrigin, pp.NumberJustify = gp(point{mid.x, mid.y - 1}), "center bottom"
	case dy > 0: // from the top, so its text runs along it
		pp.LabelOrigin, pp.Justify = gp(point{sp.leg.x, sp.leg.y + gap}), "right middle"
		pp.NumberOrigin, pp.NumberJustify = gp(point{mid.x - 1, mid.y}), "center bottom"
		pp.TextRotationDeg = 90
	default:
		pp.LabelOrigin, pp.Justify = gp(point{sp.leg.x, sp.leg.y - gap}), "left middle"
		pp.NumberOrigin, pp.NumberJustify = gp(point{mid.x - 1, mid.y}), "center bottom"
		pp.TextRotationDeg = 90
	}
	return pp
}

// cellRef names a placed part's symbol for one unit of one package. A unit's pins are numbered
// by its own device, so two units of a part are two symbols.
func cellRef(part placedPart) string {
	return fmt.Sprintf("%s|%s|%d", part.cacheKey, part.pkg, part.unit)
}

// geometry builds the sheets from the pages the walk visited.
func (f *flattener) geometry(top *dsn) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		DesignRef: strings.TrimSuffix(path.Base(top.name), path.Ext(top.name)),
		UnitNm:    unitNm,
		// Capture's overbars are rewritten into KiCad's markup, which the renderers already draw.
		TextMarkup: "kicad",
		Prov:       &geom.Provenance{SourceFile: top.name},
	}
	defs := map[string]bool{}
	for _, v := range f.visits {
		g.Sheets = append(g.Sheets, f.sheet(g, v, defs))
	}
	geomath.MarkUndrawn(g)
	return g
}

func (f *flattener) sheet(g *geom.SchematicGeometry, v visit, defs map[string]bool) *geom.SheetGeometry {
	d, pg := v.d, v.pg
	name := pg.name
	if v.bpath != "" {
		name = strings.TrimPrefix(v.bpath, "/") + " " + pg.name
	}
	s := &geom.SheetGeometry{
		Id:       v.sheet,
		Name:     name,
		ParentId: v.parent,
		Size:     &geom.BBox{Min: &geom.Point{X: 0, Y: -int64(pg.size.y)}, Max: &geom.Point{X: int64(pg.size.x), Y: 0}},
		Prov:     &geom.Provenance{SourceFile: d.name, SourceId: pg.name},
	}
	height := d.lib.fontHeight(0)

	for _, part := range pg.parts {
		pl := f.place(d, part, v.occ, v.pk)
		lp := pl.lp
		if lp == nil {
			continue
		}
		key := cellRef(part)
		lib := path.Base(d.name)
		if !defs[lib+"\x00"+key] {
			defs[lib+"\x00"+key] = true
			shapes, labels := symbolShapes(lp.draw, lp.pins)
			sd := &geom.SymbolDef{CellRef: key, LibraryRef: lib, Shapes: shapes, Annotations: labels,
				Bbox: &geom.BBox{Min: gp(point{lp.draw.box[0].x, lp.draw.box[1].y}), Max: gp(point{lp.draw.box[1].x, lp.draw.box[0].y})},
				Prov: &geom.Provenance{SourceFile: d.name, SourceId: part.cacheKey}}
			var numbers []string
			if pl.pkg != nil && part.unit < len(pl.pkg.devices) {
				numbers = pl.pkg.devices[part.unit].pins
			}
			for _, sp := range lp.pins {
				num := ""
				if sp.slot < len(numbers) {
					num = numbers[sp.slot]
				}
				if num == "" {
					continue
				}
				pp := pinPoint(sp, num, height)
				if !lp.showNames {
					pp.LabelOrigin, pp.Justify = nil, ""
				}
				if !lp.showNumbers {
					pp.NumberOrigin, pp.NumberJustify = nil, ""
				}
				sd.Pins = append(sd.Pins, pp)
			}
			g.Symbols = append(g.Symbols, sd)
		}
		sp := &geom.SymbolPlacement{
			RefDes:     pl.ref,
			CellRef:    key,
			LibraryRef: lib,
			Transform:  placeTransform(part.at, part.orient, lp.draw.width(), lp.draw.height()),
			Prov:       &geom.Provenance{SourceFile: d.name, SourceId: fmt.Sprint(part.db)},
		}
		for _, sh := range part.shown {
			field := &geom.Field{Name: sh.name, Origin: gp(point{part.at.x + sh.at.x, part.at.y + sh.at.y}),
				RotationDeg: int32(90 * sh.turns), Height: d.lib.fontHeight(sh.font), Justify: "left bottom", Visible: true} // #nosec G115 -- at most 270
			switch sh.name {
			case "Part Reference":
				field.Name, field.Value = "Reference", pl.ref
			case "Value":
				field.Value = part.value
			default:
				field.Value = part.props[sh.name]
				if field.Value == "" && lp.props != nil {
					field.Value = lp.props[sh.name]
				}
			}
			if field.Value != "" {
				sp.Fields = append(sp.Fields, field)
			}
		}
		s.Placements = append(s.Placements, sp)
	}

	byNet := map[string]*geom.WireGeometry{}
	var nets []string
	for _, w := range pg.wires {
		kind, net := geom.WireGeometry_KIND_WIRE, ""
		if w.bus {
			kind = geom.WireGeometry_KIND_BUS
		} else if node := fmt.Sprintf("w|%s|%d", v.pk, w.net); f.sets.parent[node] != "" {
			net = f.netOf[f.sets.find(node)] // "" for a wire no pin reaches
		}
		key := fmt.Sprint(kind) + "|" + net
		wg := byNet[key]
		if wg == nil {
			wg = &geom.WireGeometry{Net: net, Kind: kind, Prov: &geom.Provenance{SourceFile: d.name}}
			byNet[key] = wg
			nets = append(nets, key)
		}
		wg.Polylines = append(wg.Polylines, &geom.Polyline{Points: []*geom.Point{gp(w.a), gp(w.b)}})
	}
	sort.Strings(nets)
	for _, k := range nets {
		s.Wires = append(s.Wires, byNet[k])
	}
	for _, p := range junctions(pg) {
		s.Shapes = append(s.Shapes, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{gp(p)}})
	}

	for _, a := range pg.aliases {
		s.Labels = append(s.Labels, &geom.Label{Text: a.text, Origin: gp(a.at), Height: d.lib.fontHeight(a.font),
			RotationDeg: int32(90 * a.turns), Justify: "left bottom"}) // #nosec G115 -- at most 270
	}
	// A power symbol, port or off-page connector names the net it touches and is not a part, so it
	// is a placement with no designator, anchored to that net (SymbolPlacement.net_anchor), with its
	// name as its Value field.
	for i, sym := range pg.symbols {
		cs := d.cache.symbols[sym.symbol]
		if cs == nil {
			if sym.name != "" {
				s.Labels = append(s.Labels, symbolLabel(sym, nil, height))
			}
			continue
		}
		lib := path.Base(d.name)
		key := "symbol|" + sym.symbol
		if !defs[lib+"\x00"+key] {
			defs[lib+"\x00"+key] = true
			shapes, labels := symbolShapes(cs.draw, cs.pins)
			g.Symbols = append(g.Symbols, &geom.SymbolDef{CellRef: key, LibraryRef: lib, Shapes: shapes, Annotations: labels,
				Bbox: &geom.BBox{Min: gp(point{cs.draw.box[0].x, cs.draw.box[1].y}), Max: gp(point{cs.draw.box[1].x, cs.draw.box[0].y})},
				Prov: &geom.Provenance{SourceFile: d.name, SourceId: sym.symbol}})
		}
		sp := &geom.SymbolPlacement{
			CellRef:    key,
			LibraryRef: lib,
			Transform:  placeTransform(sym.lo, sym.orient, cs.draw.width(), cs.draw.height()),
			NetAnchor:  f.symbolNet(v, pg, sym),
			Prov:       &geom.Provenance{SourceFile: d.name, SourceId: fmt.Sprintf("%s#%d", sym.symbol, i)},
		}
		if sym.name != "" {
			l := symbolLabel(sym, cs, height)
			sp.Fields = []*geom.Field{{Name: "Value", Value: sym.name, Origin: l.Origin, Justify: l.Justify, Height: l.Height, Visible: true}}
		}
		s.Placements = append(s.Placements, sp)
	}
	for _, bl := range pg.blocks {
		hi := point{bl.at.x + bl.iface.width(), bl.at.y + bl.iface.height()}
		s.Shapes = append(s.Shapes, &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{gp(point{bl.at.x, hi.y}), gp(point{hi.x, bl.at.y})}})
		ref := bl.ref
		if o := v.occ.find(bl.db); o != nil && o.ref != "" {
			ref = o.ref
		}
		s.Labels = append(s.Labels, &geom.Label{Text: ref, Origin: gp(point{bl.at.x, bl.at.y - 2}), Height: height, Justify: "left bottom"})
		for i, pin := range bl.pins {
			if i < len(bl.pinNames) {
				s.Labels = append(s.Labels, blockPinLabel(bl.pinNames[i], pin.at, bl.at, hi, height))
			}
		}
	}
	for _, tb := range pg.titles {
		// A title block's position is from the page's bottom-right corner, where Capture draws it.
		if cs := d.cache.symbols[tb.symbol]; cs != nil {
			at := point{pg.size.x - cs.draw.width() - tb.at.x, pg.size.y - cs.draw.height() - tb.at.y}
			t := placeTransform(at, 0, cs.draw.width(), cs.draw.height())
			shapes, labels := symbolShapes(cs.draw, nil)
			for _, sh := range shapes {
				s.Shapes = append(s.Shapes, geomath.PlaceShape(t, sh))
			}
			for _, l := range labels {
				l.Origin = geomath.ApplyTransform(t, l.Origin)
				l.Height = d.lib.fontHeight(0)
				s.Labels = append(s.Labels, l)
			}
		}
		s.TitleBlock = &geom.TitleBlock{Title: tb.props["Title"], Rev: tb.props["RevCode"], Date: tb.props["Page Modify Date"], Company: tb.props["OrgName"]}
		s.SuppressWorksheet = true
	}
	return s
}

// symbolNet is the net a page symbol touches, the one named for the first wire that ends inside its
// box, or "" when it touches none that reaches a pin.
func (f *flattener) symbolNet(v visit, pg *page, sym pageSymbol) string {
	for _, w := range pg.wires {
		if w.bus || !(sym.covers(w.a) || sym.covers(w.b)) {
			continue
		}
		if node := fmt.Sprintf("w|%s|%d", v.pk, w.net); f.sets.parent[node] != "" {
			if n := f.netOf[f.sets.find(node)]; n != "" {
				return n
			}
		}
	}
	return ""
}

// symbolLabel names a page symbol. A power symbol's name sits beyond the side away from its
// connection, below a ground and above a supply, and a port's or connector's name sits inside it.
func symbolLabel(sym pageSymbol, cs *symbol, height int64) *geom.Label {
	cx := (sym.lo.x + sym.hi.x) / 2
	l := &geom.Label{Text: sym.name, Height: height}
	if sym.kind != symGlobal || cs == nil || len(cs.pins) == 0 {
		l.Origin, l.Justify = gp(point{cx, (sym.lo.y + sym.hi.y) / 2}), "center middle"
		return l
	}
	if hot := cs.pins[0].hot; hot.y*2 <= cs.draw.height() {
		l.Origin, l.Justify = gp(point{cx, sym.lo.y + cs.draw.height() + 1}), "center top" // connected at its top: a ground
	} else {
		l.Origin, l.Justify = gp(point{cx, sym.lo.y - 1}), "center bottom"
	}
	return l
}

// blockPinLabel names a block pin inside the block, along the edge it sits on.
func blockPinLabel(name string, at, lo, hi point, height int64) *geom.Label {
	const gap = 2
	l := &geom.Label{Text: name, Height: height}
	switch {
	case at.x <= lo.x:
		l.Origin, l.Justify = gp(point{at.x + gap, at.y}), "left middle"
	case at.x >= hi.x:
		l.Origin, l.Justify = gp(point{at.x - gap, at.y}), "right middle"
	case at.y <= lo.y:
		l.Origin, l.Justify, l.RotationDeg = gp(point{at.x, at.y + gap}), "right middle", 90
	default:
		l.Origin, l.Justify, l.RotationDeg = gp(point{at.x, at.y - gap}), "left middle", 90
	}
	return l
}

// junctions are the points where three or more wire ends meet, or where a wire ends on another
// wire's span. Capture draws a dot at each, and stores none.
func junctions(pg *page) []point {
	ends := map[point]int{}
	for _, w := range pg.wires {
		if !w.bus {
			ends[w.a]++
			ends[w.b]++
		}
	}
	var out []point
	for p, n := range ends {
		if n >= 3 {
			out = append(out, p)
			continue
		}
		for _, w := range pg.wires {
			if !w.bus && p != w.a && p != w.b && onSegment(p, w.a, w.b) {
				out = append(out, p)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].x < out[j].x || (out[i].x == out[j].x && out[i].y < out[j].y) })
	return out
}
