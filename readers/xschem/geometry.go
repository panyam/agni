package xschem

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/symread"
)

// Faithful schematic geometry: the drawing itself (symbol artwork + wires + labels), for the
// WebGL/SVG renderers, as opposed to the netlist (read.go). A .sym's drawing primitives
// (L/B/A/P) become geom.Shapes; component instances become placements carrying the transform
// the renderer applies to the symbol-local shapes. Resolving symbols needs the .sym library,
// same opener as the netlist; an unresolved symbol falls back to a placeholder box so the sheet
// still renders.
//
// Coordinates: xschem is Y-down (a smaller/more-negative y is higher on the page), while geom
// is Y-up, so every xschem coordinate is negated in Y (and scaled) on the way in. The placement
// transform is mapped onto geom's contract accordingly: an xschem flip is a y-axis mirror, and
// an xschem rotation r (CCW in xschem's frame) becomes a (360-90*r)-degree geom rotation because
// negating Y reverses the sense of rotation. Verified against a real schematic (see transform).

// geomScale converts xschem source units to geom units. xschem uses a half-integer grid, so a
// factor of 10 keeps that (and arc sampling) integer-precise; the renderer fits to the viewport,
// so the absolute magnitude is irrelevant.
const geomScale = 10

// gx / gy convert an xschem coordinate to a geom coordinate (scaled; y negated for Y-up).
func gx(x float64) int64 { return int64(math.Round(x * geomScale)) }
func gy(y float64) int64 { return int64(math.Round(-y * geomScale)) }

func gpt(x, y float64) *geom.Point { return &geom.Point{X: gx(x), Y: gy(y)} }

// ReadSchematicGeometry parses an xschem schematic into faithful geometry. open resolves the
// referenced .sym files for their artwork; a symbol that fails to resolve renders as a
// placeholder box. It never returns nil geometry for a well-formed file.
func ReadSchematicGeometry(r io.Reader, sourceFile string, open SymbolOpener) (*geom.SchematicGeometry, error) {
	br := bufio.NewReader(r)
	head, _ := br.Peek(256)
	if !IsXschem(head) {
		return nil, fmt.Errorf("xschem: not an xschem file (no \"v {xschem\" header)")
	}
	objs, err := parse(br)
	if err != nil {
		return nil, err
	}
	return extractGeometry(objs, sourceFile, open), nil
}

func extractGeometry(objs []object, src string, open SymbolOpener) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		UnitNm: 1,
		Prov:   &geom.Provenance{SourceFile: src},
	}
	sheet := &geom.SheetGeometry{Id: "root", SuppressWorksheet: true, Prov: &geom.Provenance{SourceFile: src}}

	symByRef := map[string]*loadedSym{}
	resolveSym := func(symref string) *loadedSym {
		base := symread.SymbolBase(symref)
		if ls, ok := symByRef[base]; ok {
			return ls
		}
		ls := loadSymbol(symref, base, src, open)
		symByRef[base] = ls
		return ls
	}

	wiresByNet := map[string]*geom.WireGeometry{}
	addWire := func(net string, a, b *geom.Point) {
		w := wiresByNet[net]
		if w == nil {
			w = &geom.WireGeometry{Net: net}
			wiresByNet[net] = w
		}
		w.Polylines = append(w.Polylines, &geom.Polyline{Points: []*geom.Point{a, b}})
	}

	for _, o := range objs {
		switch o.typ {
		case 'N':
			x1, _ := atoi(o.word(0))
			y1, _ := atoi(o.word(1))
			x2, _ := atoi(o.word(2))
			y2, _ := atoi(o.word(3))
			lab := props(lastBrace(o))["lab"]
			if netgraph.IsBusName(lab) {
				// An xschem bus is an `N` wire whose lab is a range name (WS7-042); draw it as a
				// KIND_BUS wire named by that lab (the same name the netlist reader flags as the finding
				// subject), so it styles as a bus and a bus-not-modeled finding highlights it (WS7-042b).
				// Emitted directly, not coalesced into the plain-net pool.
				sheet.Wires = append(sheet.Wires, &geom.WireGeometry{
					Kind: geom.WireGeometry_KIND_BUS, Net: lab,
					Polylines: []*geom.Polyline{{Points: []*geom.Point{gpt(x1, y1), gpt(x2, y2)}}},
					Prov:      &geom.Provenance{SourceFile: src},
				})
			} else {
				addWire(lab, gpt(x1, y1), gpt(x2, y2))
			}
		case 'C':
			symref := o.braceAt(0)
			base := symread.SymbolBase(symref)
			if annotationSymbols[base] && !geomRenderAnnotation[base] {
				continue // clutter annotation (probes/launchers/...): not drawn (WS7-037)
			}
			x, _ := atoi(o.word(1))
			y, _ := atoi(o.word(2))
			rot := atoiInt(o.word(3))
			flip := atoiInt(o.word(4))
			p := props(lastBraceC(o))
			ls := resolveSym(symref)
			// A label symbol (gnd/vdd/ipin/opin/lab_pin) names the net at its origin rather than
			// being a part, and read.go keeps it out of Components for that reason. Its instance
			// name therefore joins to nothing, so carrying it as a ref_des made the glyph selectable
			// as a component that does not exist; the net it names goes in net_anchor instead.
			ref, anchor := p["name"], ""
			if labelSymbols[base] {
				ref, anchor = "", p["lab"]
			}
			pl := &geom.SymbolPlacement{
				RefDes:    ref,
				NetAnchor: anchor,
				CellRef:   base,
				Transform: xschemTransform(x, y, rot, flip),
				Prov:      &geom.Provenance{SourceFile: src, SourceId: p["name"]},
			}
			if geomRenderAnnotation[base] {
				pl.Fields = annotationFields(ls.annotTexts, p, src, x, y, rot, flip)
			} else {
				pl.Fields = templateFields(ls.templates, p, x, y, rot, flip)
			}
			sheet.Placements = append(sheet.Placements, pl)
			// A label symbol (gnd/vdd/ipin/opin/lab_pin) also stamps its net name at its origin.
			if labelSymbols[base] {
				if lab := p["lab"]; lab != "" {
					sheet.Labels = append(sheet.Labels, &geom.Label{Text: lab, Origin: gpt(x, y)})
				}
			}
		case 'T':
			// Sheet-level free text (T {text} x y rot flip hsize vsize {props}).
			text := o.braceAt(0)
			if text == "" {
				continue
			}
			tx, _ := atoi(o.word(1))
			ty, _ := atoi(o.word(2))
			sheet.Labels = append(sheet.Labels, &geom.Label{Text: text, Origin: gpt(tx, ty), Height: textHeight(o)})
		}
	}

	for _, ls := range symByRef {
		g.Symbols = append(g.Symbols, ls.def)
	}
	for _, w := range wiresByNet {
		sheet.Wires = append(sheet.Wires, w)
	}
	sheet.Size = sheetBounds(sheet, g.Symbols)
	g.Sheets = []*geom.SheetGeometry{sheet}
	return g
}

// xschemTransform maps an xschem instance (origin x,y; rotation rot 0-3; flip 0/1) onto the geom
// Transform contract (scale, mirror, rotate-CCW, translate, in Y-up coordinates). Because Y is
// negated for Y-up, an xschem flip becomes a y-axis mirror and an xschem CCW rotation r becomes a
// (360-90*r)-degree geom rotation. Verified: res.sym pin (0,-30) under "150 -460 rot=3 flip=1"
// lands at geom (1200,4600) = xschem (120,-460), on its wire.
func xschemTransform(x, y float64, rot, flip int) *geom.Transform {
	return &geom.Transform{
		Origin:      gpt(x, y),
		RotationDeg: int32(((360 - 90*(rot%4)) % 360)),
		MirrorY:     flip == 1,
	}
}

// loadedSym is a resolved symbol: its drawing (SymbolDef) plus the field-text templates read
// from the .sym, kept so each placement can position its ref-des/value where the symbol says.
type loadedSym struct {
	def        *geom.SymbolDef
	templates  []fieldTemplate
	annotTexts []annotText // every text object, for the annotation symbols we render (WS7-037)
}

// annotText is a raw text object of a rendered annotation symbol (title/code/code_shown): its
// template string (which may embed @attr references and span lines) at a symbol-local point.
type annotText struct {
	text   string
	x, y   float64
	height float64
}

// fieldTemplate is a .sym "T {@key} x y ..." placeholder: an instance attribute drawn at a
// symbol-local position. key is the attribute name (e.g. "name", "value").
type fieldTemplate struct {
	key    string
	x, y   float64 // symbol-local
	height float64 // from the text's vsize
}

// loadSymbol builds a loadedSym for a resolved .sym, or a placeholder box when it cannot be
// resolved so the placement still draws something.
func loadSymbol(symref, base, src string, open SymbolOpener) *loadedSym {
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
		sd.Shapes = []*geom.Shape{placeholderBox()}
		sd.Asset.Placeholder = true
		sd.Bbox = &geom.BBox{Min: &geom.Point{X: -200, Y: -200}, Max: &geom.Point{X: 200, Y: 200}}
		return &loadedSym{def: sd}
	}
	objs, perr := parse(bytes.NewReader(data))
	if perr != nil {
		sd.Shapes = []*geom.Shape{placeholderBox()}
		sd.Asset.Placeholder = true
		return &loadedSym{def: sd}
	}
	sd.Shapes = symbolShapes(objs)
	for _, sp := range symbolPins(objs) {
		sd.Pins = append(sd.Pins, &geom.PinPoint{PortRef: sp.number, Name: sp.name, Loc: gpt(sp.x, sp.y)})
	}
	sd.Bbox = geomath.SymbolBBox(sd)
	return &loadedSym{def: sd, templates: symbolTemplates(objs), annotTexts: symbolAnnotTexts(objs)}
}

// symbolAnnotTexts collects every text object of a .sym verbatim, for the annotation symbols the
// geometry reader renders (WS7-037): their drawn fields are @author/@path/... and static labels,
// not just the @name/@value that symbolTemplates handles.
func symbolAnnotTexts(objs []object) []annotText {
	var out []annotText
	for _, o := range objs {
		if o.typ != 'T' {
			continue
		}
		x, _ := atoi(o.word(1))
		y, _ := atoi(o.word(2))
		vsize, _ := atoi(o.word(6))
		out = append(out, annotText{text: o.braceAt(0), x: x, y: y, height: vsize})
	}
	return out
}

// atToken matches an @attr reference inside an annotation template.
var atToken = regexp.MustCompile(`@[A-Za-z_][A-Za-z0-9_.]*`)

// annotationFields renders a rendered-annotation symbol's text: each template's @attr references
// are substituted from the instance props (or a derived value), and the result is emitted as one
// or more stacked geom.Fields at the transformed position. Static text passes through; global/
// derived @attrs not resolved here (e.g. @time_last_modified) become empty. Multi-line values
// stack downward, matching xschem's Y-down layout.
func annotationFields(texts []annotText, p map[string]string, src string, x, y float64, rot, flip int) []*geom.Field {
	var fields []*geom.Field
	for _, t := range texts {
		resolved := atToken.ReplaceAllStringFunc(t.text, func(m string) string { return annotAttr(m[1:], src, p) })
		lines := strings.Split(strings.TrimRight(resolved, "\n"), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue // blank/empty-after-substitution line keeps its slot, draws nothing
			}
			ty := t.y + float64(i)*t.height*textLineStep
			ax, ay := transform(t.x, ty, x, y, rot, flip)
			fields = append(fields, &geom.Field{
				Name:    "Annotation",
				Value:   line,
				Origin:  gpt(ax, ay),
				Height:  int64(t.height * xschemTextScale),
				Visible: true,
				Justify: "left bottom",
			})
		}
	}
	return fields
}

// annotAttr resolves an @attr reference in an annotation template: instance attributes by name,
// the schematic filename for @schname_ext, and empty for global/derived fields left out of scope.
func annotAttr(key, src string, p map[string]string) string {
	switch key {
	case "schname_ext":
		return filepath.Base(src)
	case "path", "schname", "time_last_modified":
		return "" // global/derived title fields, blank for now (WS7-037)
	default:
		return p[key]
	}
}

// symbolTemplates extracts the drawn field placeholders from a .sym: text objects whose text is
// exactly "@name" or "@value" (the reference designator and value that xschem draws by default),
// with their symbol-local position and size. Other "@..." templates (pin numbers, spice
// annotations) are not treated as fields.
func symbolTemplates(objs []object) []fieldTemplate {
	var out []fieldTemplate
	for _, o := range objs {
		if o.typ != 'T' {
			continue
		}
		var key string
		switch o.braceAt(0) {
		case "@name":
			key = "name"
		case "@value":
			key = "value"
		default:
			continue
		}
		x, _ := atoi(o.word(1))
		y, _ := atoi(o.word(2))
		vsize, _ := atoi(o.word(6))
		out = append(out, fieldTemplate{key: key, x: x, y: y, height: vsize})
	}
	return out
}

// templateFields positions an instance's drawn fields: for each symbol template, transform its
// symbol-local point by the placement (the same flip/rotate/translate as a pin) and emit a
// geom.Field carrying the instance's attribute value at that sheet position.
func templateFields(templates []fieldTemplate, p map[string]string, x, y float64, rot, flip int) []*geom.Field {
	var fields []*geom.Field
	for _, t := range templates {
		val := p[t.key]
		if val == "" {
			continue
		}
		// A multi-line value (an embedded SPICE-code block, WS7-037) draws one line per row,
		// stacked downward from the anchor. xschem is Y-down, so each successive line sits at a
		// larger symbol-local y; the transform + Y-flip carry that to the right geom position.
		lines := strings.Split(strings.TrimRight(val, "\n"), "\n")
		for i, line := range lines {
			if line == "" {
				continue // blank line: keep its vertical slot (i), draw nothing
			}
			ty := t.y + float64(i)*t.height*textLineStep
			ax, ay := transform(t.x, ty, x, y, rot, flip)
			fields = append(fields, &geom.Field{
				Name:    fieldName(t.key),
				Value:   line,
				Origin:  gpt(ax, ay),
				Height:  int64(t.height * xschemTextScale),
				Visible: true,
				Justify: "left bottom",
			})
		}
	}
	return fields
}

// textLineStep is the symbol-local line pitch for a multi-line field: the geom text height
// (height * xschemTextScale) mapped back through geomScale so successive lines are one line
// tall on the page.
const textLineStep = xschemTextScale / geomScale

// xschemTextScale converts an xschem text vsize (e.g. 0.2) to a geom text height. Tuned so
// default 0.2 text reads at a legible fraction of a symbol body.
const xschemTextScale = 300

// fieldName maps an xschem instance attribute key to the neutral field name.
func fieldName(key string) string {
	switch key {
	case "name":
		return "Reference"
	case "value":
		return "Value"
	default:
		return key
	}
}

// symbolShapes turns a .sym's drawing objects into geom Shapes: L (line) and P (polygon) become
// polylines, B (box) becomes a rect unless it is a pin box (a pin's connection marker, captured
// as a PinPoint instead), and A (arc) becomes a 3-point arc.
func symbolShapes(objs []object) []*geom.Shape {
	var shapes []*geom.Shape
	for _, o := range objs {
		switch o.typ {
		case 'L':
			x1, _ := atoi(o.word(1))
			y1, _ := atoi(o.word(2))
			x2, _ := atoi(o.word(3))
			y2, _ := atoi(o.word(4))
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{gpt(x1, y1), gpt(x2, y2)}})
		case 'B':
			p := props(lastBrace(o))
			if _, isPin := p["name"]; isPin {
				continue
			}
			if _, isPin := p["pinnumber"]; isPin {
				continue
			}
			x1, _ := atoi(o.word(1))
			y1, _ := atoi(o.word(2))
			x2, _ := atoi(o.word(3))
			y2, _ := atoi(o.word(4))
			shapes = append(shapes, &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{gpt(x1, y1), gpt(x2, y2)}})
		case 'A':
			cx, _ := atoi(o.word(1))
			cy, _ := atoi(o.word(2))
			r, _ := atoi(o.word(3))
			a, _ := atoi(o.word(4))
			b, _ := atoi(o.word(5))
			shapes = append(shapes, geomath.ArcShape(cx, cy, r, a, b, gpt))
		case 'P':
			shapes = append(shapes, polygonShape(o))
		}
	}
	return shapes
}

// polygonShape reads an xschem polygon (P layer npoints x1 y1 x2 y2 ... {props}) as a polyline.
func polygonShape(o object) *geom.Shape {
	n := atoiInt(o.word(1))
	var pts []*geom.Point
	for i := 0; i < n; i++ {
		x, okx := atoi(o.word(2 + 2*i))
		y, oky := atoi(o.word(3 + 2*i))
		if !okx || !oky {
			break
		}
		pts = append(pts, gpt(x, y))
	}
	return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts}
}

func placeholderBox() *geom.Shape {
	return &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: -200, Y: -200}, {X: 200, Y: 200}}}
}

// textHeight reads an xschem text object's height (vsize field, word 6) scaled to geom units,
// defaulting to a legible size.
func textHeight(o object) int64 {
	if v, ok := atoi(o.word(6)); ok && v > 0 {
		return int64(v * 20 * geomScale) // xschem text sizes are ~0.2-0.5; scale up to source units
	}
	return 5 * geomScale
}

// sheetBounds is the extent of everything drawn on the sheet, used as the page size when the
// format carries no explicit one.
func sheetBounds(sheet *geom.SheetGeometry, syms []*geom.SymbolDef) *geom.BBox {
	var b geomath.Bounds
	for _, w := range sheet.Wires {
		for _, pl := range w.Polylines {
			for _, p := range pl.Points {
				b.Add(p)
			}
		}
	}
	for _, l := range sheet.Labels {
		b.Add(l.Origin)
	}
	for _, pl := range sheet.Placements {
		if pl.Transform != nil {
			b.Add(pl.Transform.Origin)
		}
	}
	return b.BBox()
}
