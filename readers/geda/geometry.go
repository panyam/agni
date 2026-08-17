package geda

import (
	"io"
	"math"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/symread"
)

// Faithful schematic geometry: the drawing (symbol artwork + wires + labels), for the WebGL/SVG
// renderers, as opposed to the netlist (read.go). gEDA is Y-up like geom, so coordinates pass
// through unscaled; the placement transform maps directly (an angle is a geom CCW rotation, a
// mirror is a y-axis mirror). A .sym's L/B/V/A drawing objects become geom Shapes; an embedded G
// picture becomes a geom.Image. Symbols resolve through the same opener as the netlist; an
// unresolved symbol falls back to a placeholder box.

func gedaPt(x, y float64) *geom.Point {
	return &geom.Point{X: int64(math.Round(x)), Y: int64(math.Round(y))}
}

// ReadSchematicGeometry parses a gEDA schematic into faithful geometry. open resolves the
// referenced .sym files for their artwork.
func ReadSchematicGeometry(r io.Reader, sourceFile string, open SymbolOpener) (*geom.SchematicGeometry, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if !IsGeda([]byte(strings.Join(firstN(lines, 3), "\n"))) {
		return nil, errNotGeda
	}
	return extractGeometry(lines, sourceFile, open), nil
}

func extractGeometry(lines []string, src string, open SymbolOpener) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}}
	sheet := &geom.SheetGeometry{Id: "root", SuppressWorksheet: true, Prov: &geom.Provenance{SourceFile: src}}

	symByRef := map[string]*geom.SymbolDef{}
	resolveSym := func(symref string) *geom.SymbolDef {
		base := symread.SymbolBase(symref)
		if sd, ok := symByRef[base]; ok {
			return sd
		}
		sd := loadSymbol(symref, base, open)
		symByRef[base] = sd
		return sd
	}

	i := 0
	for i < len(lines) {
		f := strings.Fields(lines[i])
		if len(f) == 0 {
			i++
			continue
		}
		switch f[0] {
		case "C":
			cx, _ := atof(field(f, 1))
			cy, _ := atof(field(f, 2))
			angle := atoiInt(field(f, 4))
			mirror := atoiInt(field(f, 5))
			symref := field(f, 6)
			base := symread.SymbolBase(symref)
			attrs, fields, next := attrBlockFields(lines, i+1)
			i = next
			if annotationSymbols[base] && !geomRenderAnnotation[base] {
				continue // clutter annotation: not drawn in geometry (WS7-037)
			}
			resolveSym(symref)
			// A power/ground symbol names the net at its pin rather than being a part (read.go keeps
			// it out of Components), and it spells that name in its instance net= attribute. Carrying
			// it as net_anchor is what makes the glyph addressable; carrying a refdes would offer a
			// component that does not exist.
			ref, anchor := attrs["refdes"], ""
			if isPowerSymbol(base) {
				// The same fallback the netlist side resolves an anchor by (resolveAnchors): the
				// instance's net= names it, else the symbol's own, else the conventional supply for
				// that symbol family. Sharing the ladder is what keeps the glyph's key and the net
				// it actually joins from disagreeing — a ground symbol that keyed one name while the
				// solver used another would be worse than one that keyed nothing.
				anchor = netFromNetAttr(attrs["net"])
				if anchor == "" {
					if open != nil {
						if data, err := open(symref); err == nil {
							anchor = symbolNet(splitLines(data))
						}
					}
				}
				if anchor == "" {
					anchor = conventionalSupply(base)
				}
				ref = ""
			}
			pl := &geom.SymbolPlacement{
				RefDes:    ref,
				NetAnchor: anchor,
				CellRef:   base,
				Transform: &geom.Transform{Origin: gedaPt(cx, cy), RotationDeg: int32(angle % 360), MirrorY: mirror == 1},
				Prov:      &geom.Provenance{SourceFile: src, SourceId: attrs["refdes"]},
				Fields:    fields,
			}
			sheet.Placements = append(sheet.Placements, pl)
		case "N", "U":
			x1, _ := atof(field(f, 1))
			y1, _ := atof(field(f, 2))
			x2, _ := atof(field(f, 3))
			y2, _ := atof(field(f, 4))
			attrs, _, next := readAttrBlock(lines, i+1)
			i = next
			pts := []*geom.Point{gedaPt(x1, y1), gedaPt(x2, y2)}
			if field(f, 0) == "U" {
				// A gEDA `U` is a bus segment (WS7-042): draw it as a KIND_BUS wire named by its inline
				// netname (the same name the netlist reader flags on ir.BusNotModeled.Label), so it
				// styles as a bus and a bus-not-modeled finding highlights it by name (WS7-042b). Not
				// routed through findWire, so it never merges into the plain-net pool.
				sheet.Wires = append(sheet.Wires, &geom.WireGeometry{
					Kind: geom.WireGeometry_KIND_BUS, Net: attrs["netname"],
					Polylines: []*geom.Polyline{{Points: pts}},
				})
			} else {
				w := findWire(sheet, attrs["netname"])
				w.Polylines = append(w.Polylines, &geom.Polyline{Points: pts})
			}
		case "T":
			text, next := readText(lines, i)
			i = next
			tx, _ := atof(field(f, 1))
			ty, _ := atof(field(f, 2))
			if k, v, ok := splitAttr(text); ok {
				if k == "netname" {
					sheet.Labels = append(sheet.Labels, &geom.Label{Text: v, Origin: gedaPt(tx, ty), Height: 100})
				}
				continue // other attributes (device=, footprint=) are not drawn
			}
			sheet.Labels = append(sheet.Labels, &geom.Label{Text: firstLine(text), Origin: gedaPt(tx, ty), Height: 100})
		case "G":
			img, next := readPicture(lines, i, gedaPt)
			i = next
			if img != nil {
				sheet.Images = append(sheet.Images, img)
			}
		case "H":
			_, next := readText(lines, i)
			i = next
		default:
			i++
		}
	}

	for _, sd := range symByRef {
		g.Symbols = append(g.Symbols, sd)
	}
	sheet.Size = gedaSheetBounds(sheet)
	g.Sheets = []*geom.SheetGeometry{sheet}
	return g
}

func findWire(sheet *geom.SheetGeometry, net string) *geom.WireGeometry {
	for _, w := range sheet.Wires {
		if w.Net == net {
			return w
		}
	}
	w := &geom.WireGeometry{Net: net}
	sheet.Wires = append(sheet.Wires, w)
	return w
}

// loadSymbol builds a SymbolDef from a resolved .sym, or a placeholder box if it cannot resolve.
func loadSymbol(symref, base string, open SymbolOpener) *geom.SymbolDef {
	sd := &geom.SymbolDef{
		CellRef: base,
		Asset:   &geom.Asset{Kind: geom.Asset_KIND_SYMBOL, Id: base, Prov: &geom.Provenance{SourceFile: symref}},
	}
	var data []byte
	var err error
	if open != nil {
		data, err = open(symref)
	}
	if open == nil || err != nil {
		sd.Shapes = []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 400, Y: 400}}}}
		sd.Bbox = &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 400, Y: 400}}
		sd.Asset.Placeholder = true
		return sd
	}
	lines := splitLines(data)
	sd.Shapes = symbolShapes(lines)
	for _, sp := range symbolPins(lines) {
		sd.Pins = append(sd.Pins, &geom.PinPoint{PortRef: sp.number, Name: sp.name, Loc: gedaPt(sp.x, sp.y)})
	}
	sd.Bbox = geomath.SymbolBBox(sd)
	return sd
}

// symbolShapes turns a gEDA .sym's drawing objects into geom Shapes: L->polyline, B->rect (gEDA
// boxes are x,y + width,height), V->circle, A->arc. P (pins) are captured as PinPoints elsewhere.
func symbolShapes(lines []string) []*geom.Shape {
	var shapes []*geom.Shape
	i := 0
	for i < len(lines) {
		f := strings.Fields(lines[i])
		if len(f) == 0 {
			i++
			continue
		}
		switch f[0] {
		case "L":
			x1, _ := atof(field(f, 1))
			y1, _ := atof(field(f, 2))
			x2, _ := atof(field(f, 3))
			y2, _ := atof(field(f, 4))
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{gedaPt(x1, y1), gedaPt(x2, y2)}})
			i++
		case "B":
			x, _ := atof(field(f, 1))
			y, _ := atof(field(f, 2))
			w, _ := atof(field(f, 3))
			h, _ := atof(field(f, 4))
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{gedaPt(x, y), gedaPt(x+w, y+h)}})
			i++
		case "V":
			x, _ := atof(field(f, 1))
			y, _ := atof(field(f, 2))
			rad, _ := atof(field(f, 3))
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_CIRCLE, Points: []*geom.Point{gedaPt(x, y)}, Radius: int64(math.Round(rad))})
			i++
		case "A":
			cx, _ := atof(field(f, 1))
			cy, _ := atof(field(f, 2))
			rad, _ := atof(field(f, 3))
			a, _ := atof(field(f, 4))
			b, _ := atof(field(f, 5))
			shapes = append(shapes, geomath.ArcShape(cx, cy, rad, a, b, gedaPt))
			i++
		case "P", "H", "T":
			// pin (captured as PinPoint), path, or text: consume any attached block / body.
			_, next := gedaConsume(lines, i)
			i = next
		default:
			i++
		}
	}
	return shapes
}

// gedaConsume skips an object that may own an attribute block ({...}) or a multi-line text body,
// returning the index of the next object.
func gedaConsume(lines []string, i int) (map[string]string, int) {
	f := strings.Fields(lines[i])
	if len(f) > 0 && f[0] == "T" {
		_, next := readText(lines, i)
		return nil, next
	}
	attrs, _, next := readAttrBlock(lines, i+1)
	return attrs, next
}

func firstLine(s string) string {
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		return s[:nl]
	}
	return s
}

func gedaSheetBounds(sheet *geom.SheetGeometry) *geom.BBox {
	var b geomath.Bounds
	for _, w := range sheet.Wires {
		for _, pl := range w.Polylines {
			for _, p := range pl.Points {
				b.Add(p)
			}
		}
	}
	for _, pl := range sheet.Placements {
		if pl.Transform != nil {
			b.Add(pl.Transform.Origin)
		}
	}
	for _, im := range sheet.Images {
		if im.Bbox != nil {
			b.Add(im.Bbox.Min)
			b.Add(im.Bbox.Max)
		}
	}
	return b.BBox()
}
