package render

import (
	"encoding/binary"
	"math"

	"github.com/panyam/agni/core/num"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// Tier-2 packing (CONSTRAINTS C8) is a derived projection of the tier-1 sidecar into the
// columnar GPU-upload form (geom.PackedSheet), not a second schema. One sheet's drawable
// primitives are flattened into a single int32 vertex pool plus fixed-width primitive
// records, so the browser can make typed-array views and upload once (C4).

const packLayoutVersion = 1

// Primitive kinds, the GPU draw mode for a primitive's vertex range.
const (
	primLineStrip uint8 = 1 // open polyline (wires, arcs)
	primLineLoop  uint8 = 2 // closed outline (rects, circles)
	primPoints    uint8 = 3 // discrete points (pin dots)
)

// Primitive groups, a category for coloring and picking.
const (
	groupSymbol uint8 = 0
	groupWire   uint8 = 1
	groupPin    uint8 = 2
	groupFree   uint8 = 3 // sheet-level free graphics: junction dots, no-connects, notes
	groupFrame  uint8 = 4 // synthesized worksheet furniture: frame, ruler ticks, title block
	// groupBus (WS7-042) sits after the board strata (5-11, packboard.go) in the shared flat
	// group space. A bus trunk/entry tessellates to true-width triangle quads because GL cannot
	// stroke a line wider than 1px, the same reason copper is quads.
	groupBus uint8 = 12
	// groupDNP is the cross over a do-not-populate part (agni 1344), true-width quads like a bus.
	groupDNP uint8 = 13
)

// busWidthNm is the world-space (nanometer) width of a bus trunk/entry on the WebGL canvas. GL
// draws it as true-width quads, so it scales with zoom. The SVG bus is a fixed-pixel line and
// does not. 0.3mm reads clearly thicker than a hairline wire at a fitted schematic, and it is well
// above minStrokeNm.
const busWidthNm = 300_000

// schematicGroupColors is the packed-sheet palette indexed by group constant. A plain sheet
// carries only the schematic groups (0-4). A sheet that packed a bus extends the slice to groupBus,
// and one that crossed out a part extends it to groupDNP, leaving the board slots (5-11) empty.
// Other sheets keep the shorter slice so their packed bytes are unchanged.
func schematicGroupColors(style Style, hasBus, hasDNP bool) []string {
	colors := []string{
		groupSymbol: style.Symbol,
		groupWire:   style.Wire,
		groupPin:    style.Pin,
		groupFree:   style.Free,
		groupFrame:  style.Frame,
	}
	if hasBus || hasDNP {
		n := groupBus + 1
		if hasDNP {
			n = groupDNP + 1
		}
		full := make([]string, n)
		copy(full, colors)
		if hasBus {
			full[groupBus] = style.Bus
		}
		if hasDNP {
			full[groupDNP] = style.DNP
		}
		colors = full
	}
	return colors
}

// primRecordBytes is the fixed width of one primitives-table record:
// kind:u8, group:u8, _pad:u16, first_vertex:u32, count:u32.
const primRecordBytes = 12

// circleSegments is how finely a circle is tessellated into a line loop for packing, since
// the GPU line pipeline draws segments, not analytic circles.
const circleSegments = 24

// PackSheet projects one tier-1 sheet into the tier-2 PackedSheet columnar form. That form
// holds rebased int32 vertices, a fixed-width primitives table, and PrimitiveKeys back to
// ref_des/net for picking. The vertices are relative to the sheet's min corner, so GLSL ES's
// 32-bit attributes are exact. This is a derived projection of the sidecar, computed on demand,
// not stored.
func PackSheet(g *geom.SchematicGeometry, sheet *geom.SheetGeometry, opts ...Option) *geom.PackedSheet {
	style := resolveStyle(opts)
	syms := indexSymbols(g)

	// Gather primitives in world coordinates first; build() rebases to their min corner.
	var c primCollector
	add := c.Add

	// Worksheet furniture first, so the frame/title block sit behind the schematic.
	for _, fp := range worksheetLines(g, sheet) {
		add(fp.kind, groupFrame, "", "", "", fp.pts)
	}
	hasBus, hasDNP := false, false
	for _, w := range sheet.Wires {
		switch w.GetKind() {
		case geom.WireGeometry_KIND_BUS, geom.WireGeometry_KIND_BUS_ENTRY:
			// One true-width quad per polyline segment, in groupBus for the bus color. No net id,
			// since a bus's member nets are unmodeled. Its join key is the bus NAME (Net), so a bus
			// finding highlights it by name (WS7-042b). An unlabeled entry stub draws with no pick key.
			hasBus = true
			busID := w.GetNet()
			for _, pl := range w.Polylines {
				pts := pl.Points
				for i := 0; i+1 < len(pts); i++ {
					c.AddBus(groupBus, busID, quadPts(pts[i].X, pts[i].Y, pts[i+1].X, pts[i+1].Y, busWidthNm))
				}
			}
		default:
			for _, pl := range w.Polylines {
				c.AddWire(groupWire, w.Net, w.GetNetId(), xy(pl.Points))
			}
		}
	}
	// Sheet-level free graphics (junction dots, no-connect markers, notes). These are
	// already in sheet coordinates and owned by no placement, so no transform applies; the
	// SVG backend draws them straight from sheet.Shapes and the WebGL path must match.
	for _, s := range sheet.Shapes {
		kind, pts := shapeVerts(s)
		add(kind, groupFree, "", "", "", pts)
	}
	// The outline around a flagged label, which the SVG backend draws beside the label's text.
	def := defaultTextHeight(g, sheet)
	for _, l := range sheet.Labels {
		if pts := flagOutline(l, labelHeight(l, def)); pts != nil {
			add(primLineStrip, groupFree, "", "", "", xy(pts))
		}
	}
	for i, pl := range sheet.Placements {
		sym := symbolFor(syms, pl)
		if sym == nil {
			continue
		}
		// Each placement's primitives carry its index, so two placements of one ref-des light as
		// two entities (agni issue 1477).
		c.placement = num.Uint32(i + 1)
		// A glyph that names a net (a ground or rail symbol) keys as that net, as symbolKeys does
		// for SVG, so a click on it picks the net (agni issue 764). Its pins key nothing, since a
		// pin needs a ref-des.
		ref, anchor := pl.RefDes, pl.GetNetAnchor()
		if anchor != "" {
			ref = ""
		}
		for _, s := range sym.Shapes {
			kind, pts := shapeVerts(geomath.PlaceShape(pl.Transform, s))
			add(kind, groupSymbol, ref, anchor, "", pts)
		}
		for _, pin := range sym.Pins {
			wp := geomath.PlacePin(pl.Transform, pin)
			if wp != nil {
				add(primPoints, groupPin, ref, "", pin.PortRef, [][2]int64{{wp.X, wp.Y}})
			}
		}
		if a, b, ok := dnpCross(sym, pl); ok {
			hasDNP = true
			for _, d := range [][2]*geom.Point{a, b} {
				add(primTriangles, groupDNP, ref, anchor, "", quadPts(d[0].X, d[0].Y, d[1].X, d[1].Y, busWidthNm))
			}
		}
	}

	c.placement = 0

	vertices, records, keys, ox, oy := c.build()

	// Text labels, rebased to the same origin as the vertices; the overlay draws them.
	var labels []*geom.PackedLabel
	for _, pl := range collectLabels(g, sheet, style) {
		labels = append(labels, &geom.PackedLabel{
			X:           num.Int32(pl.x - ox),
			Y:           num.Int32(pl.y - oy),
			Text:        pl.text,
			Height:      num.Int32(pl.height),
			RotationDeg: pl.rotationDeg,
			Justify:     pl.justify,
			Color:       pl.color,
			MaxWidth:    num.Int32(pl.maxWidth),
			Runs:        packedRuns(pl.runs),
		})
	}

	// Sheet-level raster images, rebased to the same origin as the vertices/labels; the overlay
	// draws them. Only sheet.Images are packed, since the SVG backend draws no SymbolDef images
	// either.
	var images []*geom.PackedImage
	for _, im := range sheet.Images {
		if im.GetBbox() == nil || len(im.GetData()) == 0 {
			continue
		}
		bb := im.Bbox
		images = append(images, &geom.PackedImage{
			X:           num.Int32(bb.Min.X - ox),
			Y:           num.Int32(bb.Min.Y - oy),
			W:           num.Int32(bb.Max.X - bb.Min.X),
			H:           num.Int32(bb.Max.Y - bb.Min.Y),
			Mime:        im.Mime,
			Data:        im.Data,
			RotationDeg: im.RotationDeg,
			Mirror:      im.Mirror,
		})
	}

	return &geom.PackedSheet{
		SheetId:       sheet.Id,
		LayoutVersion: packLayoutVersion,
		FontFamily:    style.Font,
		// Indexed by group so the mapping can't drift from the group constants.
		GroupColors:     schematicGroupColors(style, hasBus, hasDNP),
		BackgroundColor: style.Page,
		OriginX:         ox,
		OriginY:         oy,
		Vertices:        vertices,
		Primitives:      records,
		Keys:            keys,
		Labels:          labels,
		Images:          images,
	}
}

// shapeVerts returns the GPU primitive kind and world vertices for a placed shape. Circles
// and rects become line loops (tessellated / cornered); polylines and arcs are line strips;
// dots are points.
func shapeVerts(s *geom.Shape) (uint8, [][2]int64) {
	switch s.Kind {
	case geom.Shape_KIND_RECT:
		if len(s.Points) < 2 {
			return primLineStrip, nil
		}
		x0, y0, x1, y1 := s.Points[0].X, s.Points[0].Y, s.Points[1].X, s.Points[1].Y
		return primLineLoop, [][2]int64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	case geom.Shape_KIND_CIRCLE:
		if len(s.Points) < 1 {
			return primLineLoop, nil
		}
		cx, cy, r := float64(s.Points[0].X), float64(s.Points[0].Y), float64(s.Radius)
		pts := make([][2]int64, 0, circleSegments)
		for i := 0; i < circleSegments; i++ {
			a := 2 * math.Pi * float64(i) / circleSegments
			pts = append(pts, [2]int64{int64(cx + r*math.Cos(a)), int64(cy + r*math.Sin(a))})
		}
		return primLineLoop, pts
	case geom.Shape_KIND_DOT:
		if len(s.Points) < 1 {
			return primPoints, nil
		}
		return primPoints, [][2]int64{{s.Points[0].X, s.Points[0].Y}}
	default: // POLYLINE, ARC
		return primLineStrip, xy(s.Points)
	}
}

func appendRecord(dst []byte, kind, group uint8, firstVertex, count uint32) []byte {
	dst = append(dst, kind, group, 0, 0) // kind, group, 2-byte pad
	dst = binary.LittleEndian.AppendUint32(dst, firstVertex)
	dst = binary.LittleEndian.AppendUint32(dst, count)
	return dst
}

func xy(pts []*geom.Point) [][2]int64 {
	out := make([][2]int64, 0, len(pts))
	for _, p := range pts {
		out = append(out, [2]int64{p.X, p.Y})
	}
	return out
}

// packedRuns carries a label's markup runs to the overlay, so it draws them as the SVG backend
// does without parsing the markup again.
func packedRuns(runs []textRun) []*geom.TextRun {
	if runs == nil {
		return nil
	}
	out := make([]*geom.TextRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, &geom.TextRun{Text: r.text, Kind: r.kind})
	}
	return out
}
