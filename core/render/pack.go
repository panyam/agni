package render

import (
	"encoding/binary"
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// Tier-2 packing (CONSTRAINTS C8): a derived projection of the tier-1 sidecar into the
// columnar GPU-upload form (geom.PackedSheet), not a second schema. One sheet's drawable
// primitives are flattened into a single int32 vertex pool plus fixed-width primitive
// records, so the browser can make typed-array views and upload once (C4).

const packLayoutVersion = 1

// Primitive kinds: the GPU draw mode for a primitive's vertex range.
const (
	primLineStrip uint8 = 1 // open polyline (wires, arcs)
	primLineLoop  uint8 = 2 // closed outline (rects, circles)
	primPoints    uint8 = 3 // discrete points (pin dots)
)

// Primitive groups: a category for coloring and picking.
const (
	groupSymbol uint8 = 0
	groupWire   uint8 = 1
	groupPin    uint8 = 2
	groupFree   uint8 = 3 // sheet-level free graphics: junction dots, no-connects, notes
	groupFrame  uint8 = 4 // synthesized worksheet furniture: frame, ruler ticks, title block
	// groupBus (WS7-042) sits after the board strata (5-11, packboard.go) in the shared flat
	// group space: a bus trunk/entry, tessellated to true-width triangle quads because GL cannot
	// stroke a line wider than 1px (the same reason copper is quads).
	groupBus uint8 = 12
)

// busWidthNm is the world-space (nanometer) width of a bus trunk/entry on the WebGL canvas. GL
// draws it as true-width quads, so unlike the SVG bus (a fixed-pixel line) it scales with zoom;
// 0.3mm reads clearly thicker than a hairline wire at a fitted schematic. Well above minStrokeNm.
const busWidthNm = 300_000

// schematicGroupColors is the packed-sheet palette indexed by group constant. A plain sheet
// carries only the schematic groups (0-4); a sheet that packed a bus extends the slice to
// groupBus (12), which sits past the board strata (5-11) in the shared group space, leaving those
// slots empty (a schematic sheet never uses them). Bus-less sheets keep the shorter slice so their
// packed bytes are unchanged.
func schematicGroupColors(style Style, hasBus bool) []string {
	colors := []string{
		groupSymbol: style.Symbol,
		groupWire:   style.Wire,
		groupPin:    style.Pin,
		groupFree:   style.Free,
		groupFrame:  style.Frame,
	}
	if hasBus {
		full := make([]string, groupBus+1)
		copy(full, colors)
		full[groupBus] = style.Bus
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

// PackSheet projects one tier-1 sheet into the tier-2 PackedSheet columnar form: rebased
// int32 vertices (sheet-relative to the min corner, so GLSL ES's 32-bit attributes are
// exact), a fixed-width primitives table, and PrimitiveKeys back to ref_des/net for
// picking. This is a derived projection of the sidecar, computed on demand, not stored.
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
	hasBus := false
	for _, w := range sheet.Wires {
		switch w.GetKind() {
		case geom.WireGeometry_KIND_BUS, geom.WireGeometry_KIND_BUS_ENTRY:
			// A bus draws as true-width quads (GL can't stroke >1px), one per polyline segment,
			// in groupBus so it takes the bus color. No net id: a bus's member nets are unmodeled;
			// its join key is the bus NAME (Net), so a bus finding highlights it by name (WS7-042b).
			// An unlabeled entry stub has an empty name and just draws (no pick key).
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
	for _, pl := range sheet.Placements {
		sym := symbolFor(syms, pl)
		if sym == nil {
			continue
		}
		for _, s := range sym.Shapes {
			kind, pts := shapeVerts(geomath.PlaceShape(pl.Transform, s))
			add(kind, groupSymbol, pl.RefDes, "", "", pts)
		}
		for _, pin := range sym.Pins {
			wp := geomath.PlacePin(pl.Transform, pin)
			if wp != nil {
				add(primPoints, groupPin, pl.RefDes, "", pin.PortRef, [][2]int64{{wp.X, wp.Y}})
			}
		}
	}

	vertices, records, keys, ox, oy := c.build()

	// Text labels, rebased to the same origin as the vertices; the overlay draws them.
	var labels []*geom.PackedLabel
	for _, pl := range collectLabels(g, sheet, style) {
		labels = append(labels, &geom.PackedLabel{
			X:           int32(pl.x - ox),
			Y:           int32(pl.y - oy),
			Text:        pl.text,
			Height:      int32(pl.height),
			RotationDeg: pl.rotationDeg,
			Justify:     pl.justify,
			Color:       pl.color,
			MaxWidth:    int32(pl.maxWidth),
		})
	}

	// Sheet-level raster images, rebased to the same origin as the vertices/labels; the overlay
	// draws them. Only sheet.Images, matching the SVG backend (which draws no SymbolDef images).
	var images []*geom.PackedImage
	for _, im := range sheet.Images {
		if im.GetBbox() == nil || len(im.GetData()) == 0 {
			continue
		}
		bb := im.Bbox
		images = append(images, &geom.PackedImage{
			X:           int32(bb.Min.X - ox),
			Y:           int32(bb.Min.Y - oy),
			W:           int32(bb.Max.X - bb.Min.X),
			H:           int32(bb.Max.Y - bb.Min.Y),
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
		GroupColors: schematicGroupColors(style, hasBus),
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
