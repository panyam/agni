package kicad

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG for image.DecodeConfig
	_ "image/png"  // register PNG for image.DecodeConfig
	"io"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/netgraph"
)

// ReadSchematicGeometry parses a KiCad .kicad_sch into the geometry sidecar
// (geom.SchematicGeometry), the render contract shared with the EDIF .eds reader. It is a
// separate extractor from ReadSchematic, which produces the netlist IR in this package; this
// produces render geometry keyed to that IR by ref_des, never the IR itself. (In edif,
// ReadSchematic is the geometry reader.)
//
// Fidelity is lossy-bounded (render subset, CONSTRAINTS C6). It extracts the symbol library
// per unit, so multi-unit parts draw the right bank, plus placements, pin connect-points, wire
// polylines with their solved nets (solveWireNets), labels, and the paper size. Hierarchical
// sub-sheet instances draw as a labeled box on the parent page (WS7-022). De Morgan alternate
// body styles are dropped. Coordinates convert from KiCad mm (Y-down) to the geom contract's
// nanometers (Y-up). sourceFile is provenance only (CONSTRAINTS C1).
func ReadSchematicGeometry(r io.Reader, sourceFile string) (*geom.SchematicGeometry, error) {
	return ReadSchematicGeometryWithSymbols(r, sourceFile, nil)
}

// ReadSchematicGeometryWithSymbols is ReadSchematicGeometry plus external
// symbol-library resolution (WS1-016). Placements whose lib_id has no embedded
// lib_symbols entry get their artwork from openSym-resolved .kicad_sym libraries, so a
// stripped file renders faithfully instead of as placeholder boxes. Embedded definitions
// always win; nil openSym resolves nothing.
func ReadSchematicGeometryWithSymbols(r io.Reader, sourceFile string, openSym func(lib string) ([]byte, error)) (*geom.SchematicGeometry, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_sch" {
		return nil, fmt.Errorf("kicad: not a .kicad_sch file (root is %q)", root.Head())
	}
	g := extractSchGeom(root, sourceFile)
	g.Symbols = append(g.Symbols, externalSymbolDefs(root, g.Symbols, newSymLibCache(openSym), sourceFile)...)
	return g, nil
}

// externalSymbolDefs resolves artwork for placement lib_ids missing from the collected
// symbol defs, one library parse per read (WS1-016). Order and dedup are the caller's
// concern only across files (the hierarchy walk's addSymbols); within one file each
// missing lib_id resolves once.
func externalSymbolDefs(root *node, have []*geom.SymbolDef, syms *symLibCache, src string) []*geom.SymbolDef {
	if syms == nil {
		return nil
	}
	embedded := map[string]bool{}
	for _, sd := range have {
		embedded[sd.GetCellRef()] = true
	}
	var out []*geom.SymbolDef
	done := map[string]bool{}
	for _, ps := range root.Children("symbol") {
		libID := symbolKey(ps)
		if libID == "" || embedded[libID] || done[libID] {
			continue
		}
		done[libID] = true
		if sym := syms.symbol(libID); sym != nil {
			out = append(out, symbolDefsOfAs(sym, libID, src)...)
		}
	}
	return out
}

// extractSchGeom walks one schematic file into the geometry sidecar, with the per-unit symbol
// library from lib_symbols and one sheet (the file is one page). unit_nm is 1 because
// coordinates are stored directly in nanometers. The single sheet is the top-level "root" with
// no parent; ReadSchematicHierarchy assigns hierarchical ids.
func extractSchGeom(root *node, src string) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		UnitNm:     1,
		TextMarkup: "kicad",
		DesignRef:  titleOf(root),
		Symbols:    symbolsOf(root, src),
		Prov:       &geom.Provenance{SourceFile: src},
	}
	single := solveWireNets(root, sheetScope{src: src})
	sh := sheetOf(root, src, busAliases(root), func(uuid string) netgraph.NetRef { return single[uuid] })
	sh.Id = "root"
	sh.Name = g.DesignRef
	g.Sheets = []*geom.SheetGeometry{sh}
	return g
}

// titleOf reads the schematic's title-block title, used as the root sheet's display name.
func titleOf(root *node) string {
	if tb := root.Child("title_block"); tb != nil {
		if t := tb.Child("title"); t != nil {
			return atomOf(t.Arg(1))
		}
	}
	return ""
}

// symbolsOf extracts the per-unit symbol library from a file's lib_symbols.
func symbolsOf(root *node, src string) []*geom.SymbolDef {
	var out []*geom.SymbolDef
	if ls := root.Child("lib_symbols"); ls != nil {
		for _, libSym := range ls.Children("symbol") {
			out = append(out, symbolDefsOf(libSym, src)...)
		}
	}
	return out
}

// solveWireNets runs the net solver over one sheet (the same collectSheetNets + Build that the
// netlist reader uses) and returns each wire's uuid -> solved net (name + deterministic
// per-instance id, WS1-022 / WS9). KiCad wires carry no inline net name, and running the SAME
// Build as the netlist read makes a wire's id match its ir.Net's id by construction. sc.prefix
// qualifies sub-sheet local names ("/amp1/SIG") to match the netlist read.
func solveWireNets(root *node, sc sheetScope) map[string]netgraph.NetRef {
	var in netInputs
	collectSheetNets(root, sc, &in)
	_, _, wireNets := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals, kicadNaming)
	return wireNets
}

// sheetOf builds one page's drawable geometry (placements, wires, labels, free shapes, paper
// size) from a parsed file. The caller assigns the sheet's id/name/parent_id. wireNetOf maps
// a wire uuid to its solved net, or a zero NetRef for an unnamed wire. It is a closure rather
// than a map because the hierarchy caller namespaces the lookup by sheet instance (a reused
// sub-sheet's wires share uuids across instances), while the single-sheet caller looks up the
// bare uuid.
func sheetOf(root *node, src string, aliases map[string][]string, wireNetOf func(uuid string) netgraph.NetRef) *geom.SheetGeometry {
	sh := &geom.SheetGeometry{Size: paperSize(root), Prov: &geom.Provenance{SourceFile: src}}
	for _, ps := range root.Children("symbol") {
		sh.Placements = append(sh.Placements, placementOf(ps, src))
	}
	for _, w := range root.Children("wire") {
		if pts := xyPoints(w.Child("pts"), sheetPt); len(pts) > 0 {
			nr := wireNetOf(uuidOf(w))
			sh.Wires = append(sh.Wires, &geom.WireGeometry{Net: nr.Name, NetId: nr.ID, Polylines: []*geom.Polyline{{Points: pts}}, Prov: &geom.Provenance{SourceFile: src}})
		}
	}
	// Bus trunks and entries (WS7-042) are drawn as wires tagged with a Kind so the renderers style
	// them apart, each named for the bus it carries (drawnBuses, agni issue 1435). A bus_alias is a
	// declaration and is not drawn.
	sh.Wires = append(sh.Wires, drawnBuses(root, aliases, src)...)
	sh.Labels = append(sh.Labels, labelsOf(root)...)
	sh.Shapes = append(sheetShapes(root), hierLabelArrows(root)...)
	for _, im := range root.Children("image") {
		if img := kicadImage(im, src); img != nil {
			sh.Images = append(sh.Images, img)
		}
	}
	// Hierarchical sub-sheet instances draw as a labeled box on THIS (the parent) page,
	// independent of whether the hierarchy walk later follows the Sheetfile into the child.
	for _, sub := range root.Children("sheet") {
		appendSubSheet(sh, sub)
	}
	sh.TitleBlock = titleBlockOf(root)
	return sh
}

// busWire builds a bus trunk or entry WireGeometry (WS7-042). name is the bus's range-label
// name ("" for an unlabeled entry stub), stored in Net because a bus-not-modeled finding's
// subject is that name and the viewer highlights the drawn bus by it (WS7-042b,
// docsite/content/architecture/geometry-and-rendering.md). NetId is left empty since a bus's
// member nets are unmodeled. The KiCad uuid goes on Prov.SourceId, as kicadImage does.
func busWire(kind geom.WireGeometry_Kind, pts []*geom.Point, uuid, name, src string) *geom.WireGeometry {
	return &geom.WireGeometry{
		Kind:      kind,
		Net:       name,
		Polylines: []*geom.Polyline{{Points: pts}},
		Prov:      &geom.Provenance{SourceFile: src, SourceId: uuid},
	}
}

// busEntryPoints synthesizes a bus_entry's drawn segment. KiCad writes it as an (at x y) origin
// plus a (size dx dy) delta, so the stub runs from (x,y) to (x+dx, y+dy) in Y-down mm; the origin
// converts to the geom frame via sheetPt (Y negated) and the delta subtracts dy, matching the
// same corner+delta convention that appendSubSheet uses for a sub-sheet box.
func busEntryPoints(e *node) []*geom.Point {
	at, size := e.Child("at"), e.Child("size")
	if at == nil || size == nil {
		return nil
	}
	start := sheetPt(at)
	if start == nil {
		return nil
	}
	dx, dy := mmToNm(atomOf(size.Arg(1))), mmToNm(atomOf(size.Arg(2)))
	end := &geom.Point{X: start.X + dx, Y: start.Y - dy}
	return []*geom.Point{start, end}
}

// appendSubSheet draws one KiCad (sheet ...) hierarchical instance onto its parent page: the
// bounding rectangle at (at)/(size), the Sheetname/Sheetfile property text, and the sheet's
// hierarchical pins, the way KiCad renders a sub-sheet reference (WS7-022). (at x y) is the
// top-left corner in Y-down mm and (size w h) extends right and down, so the far corner is
// (x+w, y+h). Both corners convert to geom via Y negation, as sheetPt does.
func appendSubSheet(sh *geom.SheetGeometry, sub *node) {
	at, size := sub.Child("at"), sub.Child("size")
	if at == nil || size == nil {
		return
	}
	x0, y0 := mmToNm(atomOf(at.Arg(1))), mmToNm(atomOf(at.Arg(2)))
	w, h := mmToNm(atomOf(size.Arg(1))), mmToNm(atomOf(size.Arg(2)))
	sh.Shapes = append(sh.Shapes, &geom.Shape{
		Kind:   geom.Shape_KIND_RECT,
		Points: []*geom.Point{{X: x0, Y: -y0}, {X: x0 + w, Y: -(y0 + h)}},
	})
	for _, key := range []string{"Sheetname", "Sheetfile"} {
		if p := propNode(sub, key); p != nil {
			if origin := sheetPt(p.Child("at")); origin != nil {
				sh.Labels = append(sh.Labels, &geom.Label{
					Text: atomOf(p.Arg(2)), Origin: origin, Height: fontSize(p), Justify: kicadJustify(p),
				})
			}
		}
	}
	// Hierarchical pins, (pin "NAME" <type> (at x y ang) (effects ...)) on the box border, draw
	// as KiCad's arrow inside the box plus a net-name label. A pin of a type KiCad has no arrow for
	// keeps a connect dot.
	for _, pin := range sub.Children("pin") {
		loc := sheetPt(pin.Child("at"))
		if loc == nil {
			continue
		}
		if arrow := sheetPinArrow(pin); arrow != nil {
			sh.Shapes = append(sh.Shapes, arrow)
		} else {
			sh.Shapes = append(sh.Shapes, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{loc}})
		}
		if name := atomOf(pin.Arg(1)); name != "" {
			l := &geom.Label{Text: name, Origin: loc, Height: fontSize(pin), Justify: kicadJustify(pin)}
			deg, _ := strconv.ParseFloat(atomOf(pin.Child("at").Arg(3)), 64)
			l.RotationDeg = geomRotation(readableLabelAngle(deg))
			shiftHierLabel(l)
			sh.Labels = append(sh.Labels, l)
		}
	}
}

// kicadImage reads a KiCad (image (at x y) (scale s) (data "b64"...)) into a geom.Image. KiCad
// stores a PNG/JPEG as base64 chunks; (at) is the image centre in Y-down mm and (scale) scales
// its native pixels (KiCad's default is 300 px/in). The bytes are decoded for their pixel size
// to build the bbox; a decode failure drops the image rather than mis-sizing it.
func kicadImage(n *node, src string) *geom.Image {
	at, data := n.Child("at"), n.Child("data")
	if at == nil || data == nil || len(data.Kids) < 2 {
		return nil
	}
	var b64 strings.Builder
	for _, k := range data.Kids[1:] {
		b64.WriteString(k.Atom)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64.String()))
	if err != nil || len(raw) == 0 {
		return nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	scale := 1.0
	if s := n.Child("scale"); s != nil {
		if v, perr := strconv.ParseFloat(atomOf(s.Arg(1)), 64); perr == nil && v > 0 {
			scale = v
		}
	}
	const nmPerInch = 25_400_000.0
	nmPerPx := nmPerInch / 300.0 * scale
	halfW := int64(float64(cfg.Width) * nmPerPx / 2)
	halfH := int64(float64(cfg.Height) * nmPerPx / 2)
	cx := mmToNm(atomOf(at.Arg(1)))
	cy := -mmToNm(atomOf(at.Arg(2))) // Y-down mm -> Y-up geom
	mime := "image/png"
	if len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xD8 {
		mime = "image/jpeg"
	}
	return &geom.Image{
		Bbox: &geom.BBox{Min: &geom.Point{X: cx - halfW, Y: cy - halfH}, Max: &geom.Point{X: cx + halfW, Y: cy + halfH}},
		Mime: mime,
		Data: raw,
		Asset: &geom.Asset{
			Kind: geom.Asset_KIND_IMAGE, Id: uuidOf(n),
			Prov: &geom.Provenance{SourceFile: src, SourceId: uuidOf(n)},
		},
	}
}

// ReadSchematicHierarchy reads a KiCad design's full sheet tree into one geom.SheetGeometry
// per sheet instance, with parent_id set to its containing sheet. It is the geometry twin of
// ReadSchematicHierarchyNets. It shares the traversal, the opener contract (open takes a relative
// Sheetfile path), the cycle guard, and the hierarchical sheet ids ("/", "/<Sheetname>", ...). A child
// that fails to open is skipped and the rest of the tree still renders. Symbols from every file
// are merged, deduped by (cell_ref, view_ref).
func ReadSchematicHierarchy(rootName string, rootContent []byte, open func(relPath string) ([]byte, error)) (*geom.SchematicGeometry, error) {
	return ReadSchematicHierarchyWithSymbols(rootName, rootContent, open, nil)
}

// ReadSchematicHierarchyWithSymbols is the geometry walk plus external symbol-library
// resolution (WS1-016); one cache serves every sheet. See
// ReadSchematicGeometryWithSymbols for the openSym contract.
func ReadSchematicHierarchyWithSymbols(rootName string, rootContent []byte, open, openSym func(string) ([]byte, error)) (*geom.SchematicGeometry, error) {
	w := &hierGeomWalker{
		g:        &geom.SchematicGeometry{UnitNm: 1, TextMarkup: "kicad", Prov: &geom.Provenance{SourceFile: rootName}},
		symByKey: map[string]bool{},
		syms:     newSymLibCache(openSym),
		// One combined net solve for the whole tree (WS1-022), so every sheet's wires get the
		// names that the netlist read produces. A per-sheet solve would mis-number N$ stubs and
		// mis-qualify some names.
		wireNets: hierWireNets(rootName, rootContent, open, openSym),
		aliases:  projectBusAliases(rootContent, open),
		open:     open,
	}
	if err := w.walk(rootContent, rootName, "/", "", "", map[string]bool{}); err != nil {
		return nil, err
	}
	return w.g, nil
}

// hierGeomWalker holds the accumulator state for the hierarchical geometry walk. One walk
// call runs per sheet instance.
type hierGeomWalker struct {
	g        *geom.SchematicGeometry
	symByKey map[string]bool
	syms     *symLibCache
	wireNets map[string]netgraph.NetRef // wire uuid -> solved net (name + id), per-instance keyed "<id>\x00<uuid>"
	// aliases is the project-wide bus_alias table, so a sheet names a group bus whose alias another
	// sheet declares, as the net solve does (projectBusAliases).
	aliases map[string][]string
	open    func(relPath string) ([]byte, error)
}

// addSymbols merges symbol defs into the design, skipping any (cell_ref, view_ref) already seen.
func (w *hierGeomWalker) addSymbols(defs []*geom.SymbolDef) {
	for _, sd := range defs {
		key := sd.GetCellRef() + "|" + sd.GetViewRef()
		if !w.symByKey[key] {
			w.symByKey[key] = true
			w.g.Symbols = append(w.g.Symbols, sd)
		}
	}
}

// walk reads one sheet instance (content) into a SheetGeometry with the given hierarchical id
// and parentID, merges its symbols, then recurses into referenced sub-sheets. ancestors is the
// source-file set on the path from the root, for cycle breaking.
func (w *hierGeomWalker) walk(content []byte, src, id, name, parentID string, ancestors map[string]bool) error {
	if len(ancestors) > 64 {
		return nil // depth backstop, in addition to the ancestor-cycle guard
	}
	root, err := parse(bytes.NewReader(content))
	if err != nil {
		return err
	}
	if root.Head() != "kicad_sch" {
		return fmt.Errorf("kicad: %q is not a .kicad_sch file (root is %q)", src, root.Head())
	}
	title := titleOf(root)
	if parentID == "" {
		name = title // the root sheet is named for the design, not a Sheetname
		w.g.DesignRef = title
	}
	w.addSymbols(symbolsOf(root, src))
	w.addSymbols(externalSymbolDefs(root, w.g.Symbols, w.syms, src))
	// Combined solve, keyed per instance ("<sheetId>\x00<uuid>") so a reused sub-sheet's
	// shared wire uuids don't collapse across instances (WS1-022). The key mirrors
	// hierWireNets's sheetScope.wirePfx = id.
	sh := sheetOf(root, src, w.aliases, func(uuid string) netgraph.NetRef { return w.wireNets[id+"\x00"+uuid] })
	sh.Id, sh.Name, sh.ParentId = id, name, parentID
	w.g.Sheets = append(w.g.Sheets, sh)

	for _, sub := range root.Children("sheet") {
		file := propValue(sub, "Sheetfile")
		if file == "" || ancestors[file] {
			continue // no reference, or a cycle back to an ancestor
		}
		childBytes, err := w.open(file)
		if err != nil {
			continue // a missing/unreadable sub-sheet is skipped, not fatal
		}
		childAnc := map[string]bool{src: true}
		for a := range ancestors {
			childAnc[a] = true
		}
		sheetName := propValue(sub, "Sheetname")
		if err := w.walk(childBytes, file, path.Join(id, sheetName), sheetName, id, childAnc); err != nil {
			return err
		}
	}
	return nil
}

// symbolDefsOfAs is symbolDefsOf with the cell_ref overridden. An EXTERNAL library file
// names its symbols bare ("R"), but placements reference them qualified ("Device:R"), so
// resolved defs must be keyed under the qualified id that the placements use (WS1-016).
func symbolDefsOfAs(libSym *node, libID, src string) []*geom.SymbolDef {
	defs := symbolDefsOf(libSym, src)
	for _, sd := range defs {
		sd.CellRef = libID
		if sd.Prov != nil {
			sd.Prov.SourceId = libID
		}
	}
	return defs
}

// symbolDefsOf turns one lib_symbols entry ("lib:Name") into one SymbolDef per unit. A KiCad
// lib symbol nests sub-symbols named "Name_<unit>_<bodyStyle>"; unit 0 is graphics common to
// every unit. Each emitted SymbolDef holds unit 0 plus that unit's graphics, and its view_ref
// is the unit number. So a placement's (unit N) selects the right bank via view_ref (WS1-007).
func symbolDefsOf(libSym *node, src string) []*geom.SymbolDef {
	libID := atomOf(libSym.Arg(1))
	text := pinTextOf(libSym)
	type bucket struct {
		shapes []*geom.Shape
		pins   []*geom.PinPoint
	}
	byUnit := map[int]*bucket{}
	unitsSeen := map[int]bool{}
	for _, sub := range libSym.Children("symbol") {
		u := unitOfSubSymbol(atomOf(sub.Arg(1)))
		b := byUnit[u]
		if b == nil {
			b = &bucket{}
			byUnit[u] = b
		}
		b.shapes = append(b.shapes, kicadShapes(sub, libPt)...)
		for _, pn := range sub.Children("pin") {
			if p := kicadPin(pn, text); p != nil {
				b.pins = append(b.pins, p)
			}
			if leg := kicadPinLeg(pn); leg != nil {
				b.shapes = append(b.shapes, leg)
			}
		}
		if u != 0 {
			unitsSeen[u] = true
		}
	}

	mkDef := func(view string, u int) *geom.SymbolDef {
		sd := &geom.SymbolDef{CellRef: libID, ViewRef: view, Prov: &geom.Provenance{SourceFile: src, SourceId: libID}}
		if b := byUnit[0]; b != nil {
			sd.Shapes = append(sd.Shapes, b.shapes...)
			sd.Pins = append(sd.Pins, b.pins...)
		}
		if u != 0 {
			if b := byUnit[u]; b != nil {
				sd.Shapes = append(sd.Shapes, b.shapes...)
				sd.Pins = append(sd.Pins, b.pins...)
			}
		}
		sd.Shapes = backgroundFirst(sd.Shapes)
		return sd
	}

	// No numbered units means a single-unit part (only the common bucket); expose it as unit 1,
	// since placements default to (unit 1).
	if len(unitsSeen) == 0 {
		return []*geom.SymbolDef{mkDef("1", 0)}
	}
	units := make([]int, 0, len(unitsSeen))
	for u := range unitsSeen {
		units = append(units, u)
	}
	sort.Ints(units)
	out := make([]*geom.SymbolDef, 0, len(units))
	for _, u := range units {
		out = append(out, mkDef(strconv.Itoa(u), u))
	}
	return out
}

// unitOfSubSymbol reads the unit number from a lib sub-symbol id "Name_<unit>_<bodyStyle>".
// The two trailing underscore-separated integers are unit and body style; the base name may
// itself contain underscores. Returns 0 (common) when the suffix is not two integers.
func unitOfSubSymbol(id string) int {
	parts := strings.Split(id, "_")
	if len(parts) < 3 {
		return 0
	}
	unit, err1 := strconv.Atoi(parts[len(parts)-2])
	if _, err2 := strconv.Atoi(parts[len(parts)-1]); err1 != nil || err2 != nil {
		return 0
	}
	return unit
}

// kicadShapes converts a node's graphic primitives (rectangle/polyline/circle/arc) into geom
// shapes in file order, using conv to place points. Lib symbol graphics pass libPt (Y-up, no
// flip); sheet graphics pass sheetPt (Y-down -> flip).
func kicadShapes(n *node, conv func(*node) *geom.Point) []*geom.Shape {
	var out []*geom.Shape
	shape := func(kind geom.Shape_Kind, src *node, pts []*geom.Point, radius int64) *geom.Shape {
		fill, color := kicadFill(src)
		return &geom.Shape{Kind: kind, Fill: fill, FillColor: color, Points: pts, Radius: radius}
	}
	for _, k := range n.Kids {
		switch k.Head() {
		case "rectangle":
			out = append(out, shape(geom.Shape_KIND_RECT, k, []*geom.Point{conv(k.Child("start")), conv(k.Child("end"))}, 0))
		case "polyline":
			s := shape(geom.Shape_KIND_POLYLINE, k, xyPoints(k.Child("pts"), conv), 0)
			// A two-point line encloses nothing, so KiCad draws only its stroke, whatever fill it
			// names. Q5's channel segments say background, and kept as fills they sort under the
			// circle's fill again.
			if len(s.Points) < 3 {
				s.Fill, s.FillColor = geom.Shape_FILL_UNSPECIFIED, ""
			}
			out = append(out, s)
		case "circle":
			var radius int64
			if rn := k.Child("radius"); rn != nil {
				radius = mmToNm(atomOf(rn.Arg(1)))
			}
			out = append(out, shape(geom.Shape_KIND_CIRCLE, k, []*geom.Point{conv(k.Child("center"))}, radius))
		case "arc":
			out = append(out, shape(geom.Shape_KIND_ARC, k, []*geom.Point{conv(k.Child("start")), conv(k.Child("mid")), conv(k.Child("end"))}, 0))
		}
	}
	return out
}

// backgroundFirst moves every background-filled shape ahead of the rest, keeping file order
// within each group. KiCad paints a symbol's body fills on a layer under its lines, so a
// transistor's filled circle sits beneath the channel drawn inside it (agni 1344). Drawn in file
// order, the circle's fill covers them.
func backgroundFirst(shapes []*geom.Shape) []*geom.Shape {
	out := make([]*geom.Shape, 0, len(shapes))
	for _, s := range shapes {
		if s.Fill == geom.Shape_FILL_BACKGROUND {
			out = append(out, s)
		}
	}
	for _, s := range shapes {
		if s.Fill != geom.Shape_FILL_BACKGROUND {
			out = append(out, s)
		}
	}
	return out
}

// kicadFill maps a graphic's (fill (type ...)) onto the geom fill style. background/outline
// map to their named fills. An explicit (type color) yields FILL_COLOR + #rrggbb. A fully
// transparent color (alpha 0, KiCad's default) is treated as unfilled.
func kicadFill(n *node) (geom.Shape_Fill, string) {
	f := n.Child("fill")
	if f == nil {
		return geom.Shape_FILL_UNSPECIFIED, ""
	}
	t := f.Child("type")
	if t == nil {
		return geom.Shape_FILL_UNSPECIFIED, ""
	}
	switch atomOf(t.Arg(1)) {
	case "outline":
		return geom.Shape_FILL_OUTLINE, ""
	case "background":
		return geom.Shape_FILL_BACKGROUND, ""
	case "color":
		c := f.Child("color")
		if c == nil {
			return geom.Shape_FILL_BACKGROUND, ""
		}
		if a := c.Arg(4); a != nil {
			if af, _ := strconv.ParseFloat(atomOf(a), 64); af == 0 {
				return geom.Shape_FILL_UNSPECIFIED, "" // transparent
			}
		}
		return geom.Shape_FILL_COLOR, rgbHex(atomOf(c.Arg(1)), atomOf(c.Arg(2)), atomOf(c.Arg(3)))
	}
	return geom.Shape_FILL_UNSPECIFIED, ""
}

// rgbHex formats three 0-255 component strings as "#rrggbb".
func rgbHex(r, g, b string) string {
	ri, _ := strconv.Atoi(r)
	gi, _ := strconv.Atoi(g)
	bi, _ := strconv.Atoi(b)
	return fmt.Sprintf("#%02x%02x%02x", ri&0xff, gi&0xff, bi&0xff)
}

// sheetShapes collects the sheet's free graphics not owned by a symbol: junction dots,
// no-connect X markers, and stand-alone drawing primitives. All in sheet coordinates (sheetPt).
func sheetShapes(root *node) []*geom.Shape {
	var out []*geom.Shape
	for _, j := range root.Children("junction") {
		if p := sheetPt(j.Child("at")); p != nil {
			out = append(out, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{p}})
		}
	}
	// A no-connect is drawn as a small X centered on its point.
	for _, nc := range root.Children("no_connect") {
		c := sheetPt(nc.Child("at"))
		if c == nil {
			continue
		}
		const arm = 635000 // 0.635mm
		out = append(out,
			&geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: c.X - arm, Y: c.Y - arm}, {X: c.X + arm, Y: c.Y + arm}}},
			&geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: c.X - arm, Y: c.Y + arm}, {X: c.X + arm, Y: c.Y - arm}}},
		)
	}
	// Stand-alone graphic primitives drawn directly on the sheet (notes, boxes, lines).
	out = append(out, kicadShapes(root, sheetPt)...)
	return out
}

// pinText is a lib symbol's pin text settings: whether its pins show their numbers and names,
// and how far past a pin's inner end its name sits. The zero value shows nothing, for a read that
// wants connect points alone.
type pinText struct {
	numbers, names bool
	offset         int64 // nm; 0 puts the name above the pin line, as KiCad does
}

// kicadPinNameOffset is KiCad's default (pin_names (offset ...)), 0.508 mm.
const kicadPinNameOffset = 508_000

// pinTextGap is how far a pin's number or name sits off the pin line, in nm: KiCad's 4 mil text
// margin plus a 10 mil pen, where kicad-cli puts them.
const pinTextGap = 355_600

// pinTextOf reads a lib symbol's (pin_numbers ...) and (pin_names ...) settings.
func pinTextOf(libSym *node) pinText {
	pt := pinText{numbers: !hideFlag(libSym.Child("pin_numbers")), offset: kicadPinNameOffset}
	pn := libSym.Child("pin_names")
	pt.names = !hideFlag(pn)
	if pn != nil {
		if o := pn.Child("offset"); o != nil {
			pt.offset = mmToNm(atomOf(o.Arg(1)))
		}
	}
	return pt
}

// kicadPin reads one lib pin's connect point and, as text allows, where its name and number go.
// KiCad lays the text out from the pin itself. With a name offset, the name sits that far past
// the pin's inner end, inside the body, and the number sits above the middle of the pin line.
// With an offset of 0, the name sits above the middle of the line and the number below it. A
// vertical pin's text runs along the pin (text_rotation_deg 90), so "above" is its left.
func kicadPin(pn *node, text pinText) *geom.PinPoint {
	at := pn.Child("at")
	if at == nil {
		return nil
	}
	pp := &geom.PinPoint{Loc: libPt(at)}
	num := pn.Child("number")
	if num != nil {
		pp.PortRef = atomOf(num.Arg(1))
		pp.SourceId = pp.PortRef
	}
	if nm := pn.Child("name"); nm != nil {
		if n := atomOf(nm.Arg(1)); n != "~" { // "~" is KiCad's "no name"
			pp.Name = n
		}
	}
	if hideFlag(pn) {
		return pp
	}
	showNum := text.numbers && pp.PortRef != ""
	showName := text.names && pp.Name != ""
	if !showNum && !showName {
		return pp
	}
	pp.Height = pinFontHeight(pn)
	deg, _ := strconv.ParseFloat(atomOf(at.Arg(3)), 64)
	rad := deg * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	var length float64
	if ln := pn.Child("length"); ln != nil {
		length = float64(mmToNm(atomOf(ln.Arg(1))))
	}
	// The text frame runs along x for a horizontal pin and along y for a vertical one, and its
	// "up" is a quarter turn counter-clockwise from that.
	tx, ty, ux, uy := 1.0, 0.0, 0.0, 1.0
	if math.Abs(dy) > math.Abs(dx) {
		pp.TextRotationDeg = 90
		tx, ty, ux, uy = 0, 1, -1, 0
	}
	at2 := func(x, y float64) *geom.Point {
		return &geom.Point{X: pp.Loc.X + int64(math.Round(x)), Y: pp.Loc.Y + int64(math.Round(y))}
	}
	midX, midY := dx*length/2, dy*length/2
	gap := float64(pinTextGap)
	above := at2(midX+ux*gap, midY+uy*gap)
	below := at2(midX-ux*gap, midY-uy*gap)
	if text.offset > 0 || !showName {
		if showName {
			in := length + float64(text.offset)
			pp.LabelOrigin = at2(dx*in, dy*in)
			pp.Justify = "left middle"
			if dx*tx+dy*ty < 0 {
				pp.Justify = "right middle"
			}
		}
		if showNum {
			pp.NumberOrigin, pp.NumberJustify = above, "center bottom"
		}
	} else {
		pp.LabelOrigin, pp.Justify = above, "center bottom"
		if showNum {
			pp.NumberOrigin, pp.NumberJustify = below, "center top"
		}
	}
	// The renderer draws a pin's text only from its name origin, so a pin showing its number
	// alone anchors the name there too, and the name is empty.
	if pp.LabelOrigin == nil {
		pp.LabelOrigin, pp.Justify = pp.NumberOrigin, pp.NumberJustify
		pp.Name = ""
	}
	return pp
}

// pinFontHeight is the glyph height of a pin's name, or of its number when the name states none.
func pinFontHeight(pn *node) int64 {
	for _, k := range []string{"name", "number"} {
		if e := pn.Child(k); e != nil {
			if sz := e.Child("effects").Child("font").Child("size"); sz != nil {
				return mmToNm(atomOf(sz.Arg(1)))
			}
		}
	}
	return 0
}

// kicadPinLeg builds the pin leg, the stub line from the pin's connection point (at) into the
// symbol body. KiCad's (at x y angle)(length L) puts the connection at (at), and the leg runs L
// in the angle direction toward the body (a resistor's top pin at y=3.81, length 1.27, angle
// 270 puts the body edge at y=2.54). Lib-local (Y-up, libPt), so it renders and transforms with
// the symbol. Returns nil for a zero-length pin (e.g. a GND power pin).
func kicadPinLeg(pn *node) *geom.Shape {
	at := pn.Child("at")
	ln := pn.Child("length")
	if at == nil || ln == nil {
		return nil
	}
	length := mmToNm(atomOf(ln.Arg(1)))
	if length == 0 {
		return nil
	}
	start := libPt(at)
	deg, _ := strconv.ParseFloat(atomOf(at.Arg(3)), 64)
	rad := deg * math.Pi / 180
	end := &geom.Point{
		X: start.X + int64(math.Round(float64(length)*math.Cos(rad))),
		Y: start.Y + int64(math.Round(float64(length)*math.Sin(rad))),
	}
	return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{start, end}}
}

// fieldsOf builds a placement's structured text fields from its (property ...) entries
// (Reference, Value, custom). Each carries its own position/justify/height. visible is false
// for a hidden field or a #-prefixed Reference (a virtual power/flag ref KiCad does not show);
// empty-value fields are dropped. The renderer draws the visible ones at their field position.
//
// KiCad stores a field's angle relative to its symbol, and the renderer adds the placement's
// rotation to it. That placement rotation is KiCad's own angle for an unmirrored symbol
// (kicadTransform), while text angles convert through geomRotation. So each field's angle is
// stored as the angle KiCad displays minus the rotation the renderer will add.
func fieldsOf(ps *node) []*geom.Field {
	var out []*geom.Field
	var symDeg float64
	if a := ps.Child("at").Arg(3); a != nil {
		symDeg, _ = strconv.ParseFloat(atomOf(a), 64)
	}
	placed := kicadTransform(ps).GetRotationDeg()
	for _, p := range ps.Children("property") {
		key, val := atomOf(p.Arg(1)), atomOf(p.Arg(2))
		if val == "" {
			continue
		}
		origin := sheetPt(p.Child("at"))
		if origin == nil {
			continue
		}
		visible := !fieldHidden(p)
		if key == "Reference" && strings.HasPrefix(val, "#") {
			visible = false
		}
		f := &geom.Field{Name: key, Value: val, Origin: origin, Height: fontSize(p), Justify: kicadJustify(p), Visible: visible}
		var deg float64
		if a := p.Child("at").Arg(3); a != nil {
			deg, _ = strconv.ParseFloat(atomOf(a), 64)
		}
		f.RotationDeg = normDeg(float64(geomRotation(deg+symDeg) - placed))
		out = append(out, f)
	}
	return out
}

// fieldHidden reports whether a property's (effects ... (hide yes)) hides it. A bare (hide)
// (legacy) also counts as hidden; (hide no) does not.
func fieldHidden(p *node) bool {
	e := p.Child("effects")
	if e == nil {
		return false
	}
	h := e.Child("hide")
	if h == nil {
		return false
	}
	return atomOf(h.Arg(1)) != "no"
}

// hideFlag reports whether a lib-symbol (pin_numbers ...) / (pin_names ...) node, or a single
// (pin ...), hides its text. KiCad 8 writes (hide yes), and KiCad 6 and 7 a bare hide atom.
func hideFlag(n *node) bool {
	if n == nil {
		return false
	}
	if h := n.Child("hide"); h != nil {
		return atomOf(h.Arg(1)) != "no"
	}
	for _, k := range n.Kids[1:] {
		if !k.IsList && k.Atom == "hide" {
			return true
		}
	}
	return false
}

// placementOf builds a SymbolPlacement from a top-level (symbol ...) instance. cell_ref is
// the full lib_id and view_ref is the unit, so it joins to the per-unit SymbolDef.
//
// A #-prefixed reference (#PWR, #FLG) is a virtual power/flag symbol, drawn like a part but
// not one, so it gets no ref_des. It records the net it names in net_anchor instead, so the
// glyph is still clickable. sch_nets.go turns the same value into a rank-0 anchor. A PWR_FLAG
// names nothing and gets no anchor, as on the netlist side. The contract is in
// docsite/content/architecture/geometry-and-rendering.md#symbols-that-name-a-net.
func placementOf(ps *node, src string) *geom.SymbolPlacement {
	ref := symbolRef(ps)
	anchor := ""
	if strings.HasPrefix(ref, "#") {
		if v := propValue(ps, "Value"); v != "PWR_FLAG" {
			anchor = v
		}
		ref = ""
	}
	pl := &geom.SymbolPlacement{
		RefDes:    ref,
		NetAnchor: anchor,
		ViewRef:   unitStr(ps),
		Transform: kicadTransform(ps),
		Fields:    fieldsOf(ps),
		Prov:      &geom.Provenance{SourceFile: src, SourceId: uuidOf(ps)},
		// The same (dnp yes) token the netlist read keeps in the part's attributes.
		Dnp: atomOf(ps.Child("dnp").Arg(1)) == "yes",
	}
	pl.CellRef = symbolKey(ps)
	pl.PinNames = alternatePins(ps)
	return pl
}

// alternatePins reads the alternate function a placed part picks for a pin,
// `(pin "G39" (uuid ...) (alternate "CAN0_DIN"))`, keyed by the pin's number. kicad-cli draws that
// name where the library's goes (agni issue 1344). It returns nil when the part picks none.
func alternatePins(ps *node) map[string]string {
	var out map[string]string
	for _, p := range ps.Children("pin") {
		alt := atomOf(p.Child("alternate").Arg(1))
		if alt == "" {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[atomOf(p.Arg(1))] = alt
	}
	return out
}

// kicadTransform maps a KiCad (at x y angle) plus optional (mirror x|y) onto the neutral
// Transform. Drawing and connectivity both use it, so a drawn pin sits where it connects (agni
// issue 1322). sheetPt converts the origin (mm to nm, Y flipped). Lib pin coordinates are already
// Y-up, so an unmirrored placement keeps KiCad's own angle.
//
// A mirrored placement takes the inverse angle. KiCad rotates and then mirrors, while
// geomath.ApplyTransform mirrors first, and the inverse angle reconciles the two. The derivation is
// in docsite/content/architecture/net-solving.md#kicad-connection-point-semantics. Without it a
// symbol mirrored at 90 or 270 swaps its two pins onto each other's nets (agni issue 577).
// TestMirroredPinPlacement holds all twelve placements of a two-pin symbol to kicad-cli.
func kicadTransform(ps *node) *geom.Transform {
	t := &geom.Transform{}
	if m := ps.Child("mirror"); m != nil {
		switch atomOf(m.Arg(1)) {
		case "x":
			t.MirrorX = true
		case "y":
			t.MirrorY = true
		}
	}
	if at := ps.Child("at"); at != nil {
		t.Origin = sheetPt(at)
		if a := at.Arg(3); a != nil {
			if deg, err := strconv.ParseFloat(atomOf(a), 64); err == nil {
				if t.MirrorX || t.MirrorY {
					t.RotationDeg = geomRotation(deg)
				} else {
					t.RotationDeg = normDeg(deg)
				}
			}
		}
	}
	return t
}

// labelsOf collects the on-sheet text: net labels, global/hierarchical labels, and free text.
func labelsOf(root *node) []*geom.Label {
	var out []*geom.Label
	for _, tag := range []string{"label", "global_label", "hierarchical_label", "text", "text_box"} {
		for _, n := range root.Children(tag) {
			at := n.Child("at")
			origin := sheetPt(at)
			if origin == nil {
				continue
			}
			l := &geom.Label{Text: atomOf(n.Arg(1)), Origin: origin, Height: fontSize(n), Justify: kicadJustify(n)}
			deg, _ := strconv.ParseFloat(atomOf(at.Arg(3)), 64)
			l.RotationDeg = geomRotation(readableLabelAngle(deg))
			switch tag {
			case "global_label":
				placeGlobalLabel(l, n, deg)
			case "label":
				liftLocalLabel(l)
			case "hierarchical_label":
				shiftHierLabel(l)
			}
			out = append(out, l)
		}
	}
	return out
}

// readableLabelAngle is the angle KiCad draws a sheet label or text at. KiCad keeps schematic
// text readable, so a label at 180 reads left to right and one at 270 reads bottom to top, and
// the file's justify is already the justify of the run as read. So (at x y 180) with (justify
// right) ends at its anchor and runs left. Handing the renderer 180 turned the run upright a
// second time and flipped that justify, so the label ran over its wire (agni 1344).
func readableLabelAngle(deg float64) float64 {
	switch normDeg(deg) {
	case 90, 270:
		return 90
	}
	return 0
}

// liftLocalLabel moves a local label's text off its wire, as kicad-cli draws it. KiCad lifts
// the run by its text offset, 0.15 heights, plus the pen, which on Jetson measured 0.343 mm at
// a 1.27 mm height. The lift is along the run's up, so a vertical label moves left.
func liftLocalLabel(l *geom.Label) {
	h := float64(l.Height)
	if h == 0 {
		h = 1_270_000
	}
	d := int64(math.Round((0.15 + 0.12) * h))
	l.Anchor = l.Origin
	if l.RotationDeg != 0 {
		l.Origin = &geom.Point{X: l.Origin.X - d, Y: l.Origin.Y}
		return
	}
	l.Origin = &geom.Point{X: l.Origin.X, Y: l.Origin.Y + d}
}

// shiftHierLabel moves a hierarchical label's or a sheet pin's text along the way it reads,
// away from its connect point, as kicad-cli draws it. KiCad leaves room for the pin's arrow, one
// text width plus the text offset (1.15 heights, 1.46 mm on Jetson). A left-justified run moves
// forward and a right-justified one back.
func shiftHierLabel(l *geom.Label) {
	h := float64(l.Height)
	if h == 0 {
		h = 1_270_000
	}
	d := int64(math.Round(1.15 * h))
	switch {
	case strings.HasPrefix(l.Justify, "right"):
		d = -d
	case !strings.HasPrefix(l.Justify, "left"):
		return
	}
	l.Anchor = l.Origin
	if l.RotationDeg != 0 {
		l.Origin = &geom.Point{X: l.Origin.X, Y: l.Origin.Y + d}
		return
	}
	l.Origin = &geom.Point{X: l.Origin.X + d, Y: l.Origin.Y}
}

// hierArrows are KiCad's hierarchical label outlines (TemplateIN_HN and its siblings in
// sch_label.cpp), in half text heights, for a label running toward -x from its connect point at
// 0 with y down. kicad-cli draws the input one exactly this way on orient-labels.kicad_sch.
var hierArrows = map[string][][2]float64{
	"input":         {{0, 0}, {-1, -1}, {-2, -1}, {-2, 1}, {-1, 1}, {0, 0}},
	"output":        {{-2, 0}, {-1, 1}, {0, 1}, {0, -1}, {-1, -1}, {-2, 0}},
	"bidirectional": {{0, 0}, {-1, -1}, {-2, 0}, {-1, 1}, {0, 0}},
	"tri_state":     {{0, 0}, {-1, -1}, {-2, 0}, {-1, 1}, {0, 0}},
	"passive":       {{0, -1}, {-2, -1}, {-2, 1}, {0, 1}, {0, -1}},
}

// hierLabelArrows is the arrow KiCad draws at each hierarchical label's connect point, as sheet
// shapes. Its size follows the text height alone, so the reader can place it and no renderer
// has to know it.
func hierLabelArrows(root *node) []*geom.Shape {
	var out []*geom.Shape
	for _, n := range root.Children("hierarchical_label") {
		at := n.Child("at")
		deg, _ := strconv.ParseFloat(atomOf(at.Arg(3)), 64)
		if s := hierArrow(sheetPt(at), atomOf(n.Child("shape").Arg(1)), deg, fontSize(n)); s != nil {
			out = append(out, s)
		}
	}
	return out
}

// sheetPinArrow is the arrow KiCad draws at a sheet pin, inside the sheet's box. KiCad draws a
// sheet pin as the hierarchical label it stands for, seen from the parent sheet. The arrow turns a
// half turn, and input and output swap, since a signal entering the sheet leaves the parent
// (SCH_SHEET_PIN::CreateGraphicShape). kicad-cli draws orient-sheetpins.kicad_sch that way.
func sheetPinArrow(pin *node) *geom.Shape {
	at := pin.Child("at")
	deg, _ := strconv.ParseFloat(atomOf(at.Arg(3)), 64)
	shape := atomOf(pin.Arg(2))
	switch shape {
	case "input":
		shape = "output"
	case "output":
		shape = "input"
	}
	return hierArrow(sheetPt(at), shape, deg+180, fontSize(pin))
}

// hierArrow draws one hierarchical arrow of the named shape at o, for a label at deg, sized by
// the text height. It returns nil for a shape KiCad draws no arrow for.
func hierArrow(o *geom.Point, shape string, deg float64, height int64) *geom.Shape {
	tmpl, ok := hierArrows[shape]
	if o == nil || !ok {
		return nil
	}
	half := float64(height) / 2
	if half == 0 {
		half = 635_000
	}
	// The template runs toward 180. Turn it to the label's own angle, counter-clockwise as
	// seen, in the Y-up sheet. KiCad draws 270 as 90 mirrored along the pin, not turned a
	// further half turn, as kicad-cli's arrows show.
	down := normDeg(deg) == 270
	if down {
		deg = 90
	}
	r := (deg - 180) * math.Pi / 180
	c, sn := math.Cos(r), math.Sin(r)
	pts := make([]*geom.Point, 0, len(tmpl))
	for _, p := range tmpl {
		x, y := p[0]*half, -p[1]*half
		px, py := x*c-y*sn, x*sn+y*c
		if down {
			py = -py
		}
		pts = append(pts, &geom.Point{X: o.X + int64(math.Round(px)), Y: o.Y + int64(math.Round(py))})
	}
	return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts}
}

// globalLabelFlags maps KiCad's (shape ...) on a global label to the flag drawn around it.
var globalLabelFlags = map[string]geom.Label_Flag{
	"input":         geom.Label_FLAG_INPUT,
	"output":        geom.Label_FLAG_OUTPUT,
	"bidirectional": geom.Label_FLAG_BIDIRECTIONAL,
	"tri_state":     geom.Label_FLAG_TRI_STATE,
	"passive":       geom.Label_FLAG_PASSIVE,
}

// kicadLabelMargin is KiCad's default label box margin, as a fraction of the text height.
const kicadLabelMargin = 0.375

// placeGlobalLabel moves a global label's text off its connect point, as kicad-cli draws it, and
// records the flag drawn around it. The text starts a margin past the connect point along the
// direction the label runs, plus three quarters of its height for a flag with an arrow at the
// wire end. deg is the label's angle as written, which says which way it runs from the wire.
func placeGlobalLabel(l *geom.Label, n *node, deg float64) {
	flag, ok := globalLabelFlags[atomOf(n.Child("shape").Arg(1))]
	if !ok {
		flag = geom.Label_FLAG_PASSIVE
	}
	h := float64(l.Height)
	if h == 0 {
		h = 1_270_000
	}
	away := kicadLabelMargin * h
	switch flag {
	case geom.Label_FLAG_INPUT, geom.Label_FLAG_BIDIRECTIONAL, geom.Label_FLAG_TRI_STATE:
		away += 0.75 * h
	}
	// KiCad's angle is counter-clockwise as seen, and the sheet is Y-up here.
	rad := deg * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)
	l.Flag = flag
	l.Anchor = l.Origin
	l.Origin = &geom.Point{X: l.Origin.X + int64(math.Round(dx*away)), Y: l.Origin.Y + int64(math.Round(dy*away))}
}

// kicadJustify maps a node's (effects (justify ...)) onto the canonical geom convention,
// "<h> <v>" with h in {left,right} and v in {top,bottom}, each omitted when centered (so
// "" means centered both ways). KiCad's tokens (left/right/top/bottom) are already the
// canonical spellings.
func kicadJustify(n *node) string {
	e := n.Child("effects")
	if e == nil {
		return ""
	}
	j := e.Child("justify")
	if j == nil {
		return ""
	}
	h, v := "", ""
	for i := 1; j.Arg(i) != nil; i++ {
		switch atomOf(j.Arg(i)) {
		case "left":
			h = "left"
		case "right":
			h = "right"
		case "top":
			v = "top"
		case "bottom":
			v = "bottom"
		}
	}
	return strings.TrimSpace(h + " " + v)
}

// fontSize reads a text node's (effects (font (size h h))) height in nanometers, or 0.
func fontSize(n *node) int64 {
	e := n.Child("effects")
	if e == nil {
		return 0
	}
	f := e.Child("font")
	if f == nil {
		return 0
	}
	if s := f.Child("size"); s != nil {
		return mmToNm(atomOf(s.Arg(1)))
	}
	return 0
}

// titleBlockOf reads the (title_block (title ..)(rev ..)(date ..)(company ..)(comment N ..))
// fields. Returns nil when the schematic has no title block.
func titleBlockOf(root *node) *geom.TitleBlock {
	tb := root.Child("title_block")
	if tb == nil {
		return nil
	}
	out := &geom.TitleBlock{
		Title:   atomOf(tb.Child("title").Arg(1)),
		Rev:     atomOf(tb.Child("rev").Arg(1)),
		Date:    atomOf(tb.Child("date").Arg(1)),
		Company: atomOf(tb.Child("company").Arg(1)),
	}
	for _, c := range tb.Children("comment") {
		// (comment N "text"): the text is the second arg.
		if t := atomOf(c.Arg(2)); t != "" {
			out.Comments = append(out.Comments, t)
		}
	}
	return out
}

// paperSize maps the (paper ...) setting to a sheet bounding box. Named sizes resolve to their
// mm dimensions; a "User" paper has explicit width/height. The box is Y-up (Y flipped),
// so it spans x in [0, w] and y in [-h, 0].
func paperSize(root *node) *geom.BBox {
	p := root.Child("paper")
	if p == nil {
		return nil
	}
	w, h := paperMM(atomOf(p.Arg(1)))
	if w == 0 {
		w, _ = strconv.ParseFloat(atomOf(p.Arg(2)), 64)
		h, _ = strconv.ParseFloat(atomOf(p.Arg(3)), 64)
	}
	if w == 0 || h == 0 {
		return nil
	}
	return &geom.BBox{
		Min: &geom.Point{X: 0, Y: -int64(math.Round(h * 1e6))},
		Max: &geom.Point{X: int64(math.Round(w * 1e6)), Y: 0},
	}
}

// paperMM returns the width and height in mm for a named KiCad paper size, or (0,0) if the
// name is not a standard size (e.g. "User", which has explicit dimensions).
func paperMM(name string) (float64, float64) {
	switch name {
	case "A0":
		return 1189, 841
	case "A1":
		return 841, 594
	case "A2":
		return 594, 420
	case "A3":
		return 420, 297
	case "A4":
		return 297, 210
	case "A5":
		return 210, 148
	case "USLetter", "Letter":
		return 279.4, 215.9
	case "USLegal", "Legal":
		return 355.6, 215.9
	case "USLedger", "Ledger", "Tabloid":
		return 431.8, 279.4
	}
	return 0, 0
}

// unitStr returns a placement's unit number as a string (default "1"), used as view_ref.
func unitStr(ps *node) string {
	if u := ps.Child("unit"); u != nil {
		return atomOf(u.Arg(1))
	}
	return "1"
}

// xyPoints reads every (xy X Y) child of a (pts ...) node into geom points, using conv so the
// caller picks lib (no flip) or sheet (Y flip) coordinates.
func xyPoints(pts *node, conv func(*node) *geom.Point) []*geom.Point {
	if pts == nil {
		return nil
	}
	var out []*geom.Point
	for _, xy := range pts.Children("xy") {
		out = append(out, conv(xy))
	}
	return out
}

// KiCad uses two coordinate systems. Library symbol graphics are Y-up (like the geom
// contract), while the schematic SHEET is Y-down. So lib-local points convert straight
// (libPt, no flip) and sheet-level points negate Y (sheetPt). Flipping lib points too
// vertically mirrors every symbol (an upside-down GND).

// libPt reads a node's first two args as KiCad millimeters into a geom point WITHOUT flipping
// Y, for library-symbol-local coordinates (shapes, pins), which are already Y-up.
func libPt(n *node) *geom.Point {
	if n == nil {
		return nil
	}
	return &geom.Point{X: mmToNm(atomOf(n.Arg(1))), Y: mmToNm(atomOf(n.Arg(2)))}
}

// sheetPt reads a node's first two args as KiCad millimeters into a geom point with Y negated,
// for sheet-level coordinates (placement origin, wires, labels), which are Y-down.
func sheetPt(n *node) *geom.Point {
	if n == nil {
		return nil
	}
	return &geom.Point{X: mmToNm(atomOf(n.Arg(1))), Y: -mmToNm(atomOf(n.Arg(2)))}
}

// geomRotation turns a KiCad text or mirrored-placement angle into the geom contract's
// rotation. The Y flip from KiCad's Y-down sheet frame to geom's Y-up frame is a reflection,
// which reverses the direction of a rotation, so the geom angle is -kicad (mod 360). An
// unmirrored placement keeps KiCad's angle instead (normDeg), because its lib pins are already
// Y-up (see kicadTransform).
func geomRotation(deg float64) int32 {
	r := math.Mod(360-deg, 360)
	if r < 0 {
		r += 360
	}
	return int32(r)
}

// normDeg reduces an angle in degrees to [0, 360).
func normDeg(deg float64) int32 {
	r := math.Mod(deg, 360)
	if r < 0 {
		r += 360
	}
	return int32(r)
}

// mmToNm converts a millimeter string to nanometers, exact at KiCad's 0.0001mm grid.
func mmToNm(s string) int64 {
	f, _ := strconv.ParseFloat(s, 64)
	return int64(math.Round(f * 1e6))
}
