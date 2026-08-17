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
// (geom.SchematicGeometry), the render contract shared with the EDIF .eds reader. It reuses
// the KiCad s-expr parser (sexpr.go) but is a separate extractor from ReadSchematic, which
// produces the netlist IR: this produces render geometry keyed to that IR by ref_des, never
// the IR itself.
//
// Named ReadSchematicGeometry, not ReadSchematic, because ReadSchematic is already taken by
// the netlist reader in this package (unlike edif, where ReadSchematic returns geometry).
//
// Fidelity: lossy-bounded (render subset), CONSTRAINTS C6. Extracts the symbol library
// (per unit, so multi-unit parts draw the right bank), placements, pin connect-points, wire
// polylines, labels, and the paper size. Hierarchical sub-sheet instances draw as a labeled
// box on the parent page (WS7-022). Drops: net attribution of wires (KiCad connectivity is
// implicit in wire/label geometry, so wires carry no net name here; nets come from the
// netlist IR) and De Morgan alternate body styles.
// Coordinates are converted from KiCad mm (Y-down) to the geom contract's nanometers (Y-up).
// sourceFile is recorded in provenance only; the caller owns file I/O (CONSTRAINTS C1).
func ReadSchematicGeometry(r io.Reader, sourceFile string) (*geom.SchematicGeometry, error) {
	return ReadSchematicGeometryWithSymbols(r, sourceFile, nil)
}

// ReadSchematicGeometryWithSymbols is ReadSchematicGeometry plus external
// symbol-library resolution (WS1-016): placements whose lib_id has no embedded
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
		libID := atomOf(ps.Child("lib_id").Arg(1))
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

// extractSchGeom walks one schematic file into the geometry sidecar: the per-unit symbol
// library from lib_symbols, and one sheet (the file is one page) holding placements, wires,
// and labels. unit_nm is 1 because coordinates are stored directly in nanometers. The single
// sheet is the top-level "root" (no parent); ReadSchematicHierarchy assigns hierarchical ids.
func extractSchGeom(root *node, src string) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		UnitNm:    1,
		DesignRef: titleOf(root),
		Symbols:   symbolsOf(root, src),
		Prov:      &geom.Provenance{SourceFile: src},
	}
	single := solveWireNets(root, sheetScope{src: src})
	sh := sheetOf(root, src, func(uuid string) netgraph.NetRef { return single[uuid] })
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

// solveWireNets runs the net solver over one sheet's geometry (the same collectSheetNets +
// Build the netlist reader uses) and returns each wire's uuid -> solved net (name + the
// deterministic per-instance id, WS1-022 / WS9). KiCad wires carry no inline net name, so the
// geometry pass re-derives both from connectivity — the SAME Build the netlist read runs, so the
// id on a wire matches the id on the corresponding ir.Net by construction. sc.prefix qualifies
// sub-sheet local names ("/amp1/SIG") to match what the netlist read calls the net.
func solveWireNets(root *node, sc sheetScope) map[string]netgraph.NetRef {
	var in netInputs
	collectSheetNets(root, sc, &in)
	_, _, wireNets := netgraph.Build(in.wires, in.anchors, in.pins, in.terminals)
	return wireNets
}

// sheetOf builds one page's drawable geometry (placements, wires, labels, free shapes, paper
// size) from a parsed file. The caller assigns the sheet's id/name/parent_id. wireNetOf maps
// a wire uuid to its solved net (name + per-instance id, WS1-022 / WS9); a zero NetRef for an
// unnamed wire. It is a closure, not a map, because the hierarchy caller must namespace the
// lookup by sheet instance (a reused sub-sheet's wires share uuids across instances), while the
// single-sheet caller looks up the bare uuid.
func sheetOf(root *node, src string, wireNetOf func(uuid string) netgraph.NetRef) *geom.SheetGeometry {
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
	// Bus trunks and entries (WS7-042): drawn as wires tagged with a Kind so the renderers style
	// them distinctly (thick, distinct color). A bus is identified by its range-label NAME
	// (`DATA[7:0]`, WS1-034), so the trunk carries that name in Net — the join key a bus-not-modeled
	// finding (whose subject is that name) highlights it on (WS7-042b). The entry stub carries no
	// label of its own, so it is drawn (styled) but unnamed. A bus_alias is a declaration, not drawn.
	for _, b := range root.Children("bus") {
		if pts := xyPoints(b.Child("pts"), sheetPt); len(pts) > 0 {
			sh.Wires = append(sh.Wires, busWire(geom.WireGeometry_KIND_BUS, pts, uuidOf(b), busLabelFor(root, pts), src))
		}
	}
	for _, e := range root.Children("bus_entry") {
		if pts := busEntryPoints(e); len(pts) > 0 {
			sh.Wires = append(sh.Wires, busWire(geom.WireGeometry_KIND_BUS_ENTRY, pts, uuidOf(e), "", src))
		}
	}
	sh.Labels = append(sh.Labels, labelsOf(root)...)
	sh.Shapes = sheetShapes(root)
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

// busWire builds a bus trunk/entry WireGeometry (WS7-042). name is the bus's range-label name
// (its identity under WS1-034; "" for an unlabeled entry stub), carried in Net so a bus-not-modeled
// finding — whose subject is that name — highlights the drawn bus (WS7-042b). NetId stays empty (a
// bus's member nets are unmodeled). The KiCad uuid is kept on Prov.SourceId (the geom-side identity,
// as kicadImage does) for provenance and future per-instance use.
func busWire(kind geom.WireGeometry_Kind, pts []*geom.Point, uuid, name, src string) *geom.WireGeometry {
	return &geom.WireGeometry{
		Kind:      kind,
		Net:       name,
		Polylines: []*geom.Polyline{{Points: pts}},
		Prov:      &geom.Provenance{SourceFile: src, SourceId: uuid},
	}
}

// busLabelFor returns the range-bus label name (e.g. "DATA[7:0]") sitting on this bus polyline, or ""
// if none. A KiCad bus is named by a range label placed on its wire (WS1-034); naming the drawn bus
// with it lets a bus finding join to the geometry by name (WS7-042b). Uses the same name
// normalization as the netlist detector (collectBuses) so the finding subject and the geometry name
// match exactly.
func busLabelFor(root *node, pts []*geom.Point) string {
	for _, tag := range []string{"label", "global_label"} {
		for _, l := range root.Children(tag) {
			name := unescapeName(atomOf(l.Arg(1)))
			if !netgraph.IsBusName(name) {
				continue
			}
			if p := sheetPt(l.Child("at")); p != nil && pointOnPolyline(p, pts) {
				return name
			}
		}
	}
	return ""
}

// pointOnPolyline reports whether p lies on any segment of the polyline, within a sub-grid
// tolerance (KiCad places a bus label's connection point on the bus wire).
func pointOnPolyline(p *geom.Point, pts []*geom.Point) bool {
	const tol = 1000 // 0.001mm, well under KiCad's 0.0001mm grid
	for i := 0; i+1 < len(pts); i++ {
		a, b := pts[i], pts[i+1]
		if p.X < min(a.X, b.X)-tol || p.X > max(a.X, b.X)+tol || p.Y < min(a.Y, b.Y)-tol || p.Y > max(a.Y, b.Y)+tol {
			continue
		}
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		segLen := math.Hypot(dx, dy)
		if segLen == 0 {
			if math.Hypot(float64(p.X-a.X), float64(p.Y-a.Y)) <= tol {
				return true
			}
			continue
		}
		// Perpendicular distance from p to the (infinite) line through a,b.
		if math.Abs(dx*float64(p.Y-a.Y)-dy*float64(p.X-a.X))/segLen <= tol {
			return true
		}
	}
	return false
}

// busEntryPoints synthesizes a bus_entry's drawn segment. KiCad writes it as an (at x y) origin
// plus a (size dx dy) delta, so the stub runs from (x,y) to (x+dx, y+dy) in Y-down mm; the origin
// converts to the geom frame via sheetPt (Y negated) and the delta subtracts dy, matching the
// same corner+delta convention appendSubSheet uses for a sub-sheet box.
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
// hierarchical pins (WS7-022). KiCad renders a sub-sheet reference this way; the reader
// previously read the block only to follow its Sheetfile, dropping the box the parent shows.
// (at x y) is the top-left corner in Y-down mm and (size w h) extends right and down, so the
// far corner is (x+w, y+h); both corners convert to geom via Y negation, as sheetPt does.
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
	// Hierarchical pins: (pin "NAME" <type> (at x y ang) (effects ...)) on the box border. Draw
	// each as a connect dot plus its net-name label, matching KiCad's sheet-pin symbol.
	for _, pin := range sub.Children("pin") {
		loc := sheetPt(pin.Child("at"))
		if loc == nil {
			continue
		}
		sh.Shapes = append(sh.Shapes, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{loc}})
		if name := atomOf(pin.Arg(1)); name != "" {
			sh.Labels = append(sh.Labels, &geom.Label{Text: name, Origin: loc, Height: fontSize(pin), Justify: kicadJustify(pin)})
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

// ReadSchematicHierarchy reads a KiCad design's full sheet tree. It parses rootContent, then
// follows each (sheet ...) instance's Sheetfile reference to a child .kicad_sch, recursively,
// producing one geom.SheetGeometry per sheet instance with parent_id set to its containing
// sheet. Sheet ids are hierarchical paths built from Sheetnames (root "/", child "/<name>",
// ...), a vendor-neutral identity rather than a KiCad UUID.
//
// open fetches a child by its (relative) Sheetfile path; the caller resolves it against the
// root's location, so this package does no file I/O (CONSTRAINTS C1). A child that fails to
// open is skipped (the rest of the tree still renders); a Sheetfile already on the current
// ancestor chain is skipped to break cycles. Symbols from every file are merged, deduped by
// (cell_ref, view_ref).
func ReadSchematicHierarchy(rootName string, rootContent []byte, open func(relPath string) ([]byte, error)) (*geom.SchematicGeometry, error) {
	return ReadSchematicHierarchyWithSymbols(rootName, rootContent, open, nil)
}

// ReadSchematicHierarchyWithSymbols is the geometry walk plus external symbol-library
// resolution (WS1-016); one cache serves every sheet. See
// ReadSchematicGeometryWithSymbols for the openSym contract.
func ReadSchematicHierarchyWithSymbols(rootName string, rootContent []byte, open, openSym func(string) ([]byte, error)) (*geom.SchematicGeometry, error) {
	w := &hierGeomWalker{
		g:        &geom.SchematicGeometry{UnitNm: 1, Prov: &geom.Provenance{SourceFile: rootName}},
		symByKey: map[string]bool{},
		syms:     newSymLibCache(openSym),
		// One combined net solve for the whole tree (WS1-022): wire uuids are globally unique,
		// so this single map names every sheet's wires with the same names the netlist read
		// produces — a per-sheet solve would mis-number N$ stubs and mis-qualify some names.
		wireNets: hierWireNets(rootName, rootContent, open, openSym),
		open:     open,
	}
	if err := w.walk(rootContent, rootName, "/", "", "", map[string]bool{}); err != nil {
		return nil, err
	}
	return w.g, nil
}

// hierGeomWalker carries the accumulator state for the hierarchical geometry walk — the fields
// a recursive closure would otherwise capture: the SchematicGeometry under construction, the
// (cell_ref, view_ref) dedup set for merged symbols, the external symbol-library cache, the
// combined wire uuid -> net-name map, and the sub-sheet opener. One walk call runs per sheet
// instance.
type hierGeomWalker struct {
	g        *geom.SchematicGeometry
	symByKey map[string]bool
	syms     *symLibCache
	wireNets map[string]netgraph.NetRef // wire uuid -> solved net (name + id), per-instance keyed "<id>\x00<uuid>"
	open     func(relPath string) ([]byte, error)
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
	sh := sheetOf(root, src, func(uuid string) netgraph.NetRef { return w.wireNets[id+"\x00"+uuid] })
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

// symbolDefsOfAs is symbolDefsOf with the cell_ref overridden: an EXTERNAL library file
// names its symbols bare ("R"), but placements reference them qualified ("Device:R"), so
// resolved defs must key under the qualified id the placements use (WS1-016).
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
// every unit. Each emitted SymbolDef (view_ref = the unit number) carries unit 0 plus that
// unit's graphics, so a placement's (unit N) selects the right bank via view_ref (WS1-007).
func symbolDefsOf(libSym *node, src string) []*geom.SymbolDef {
	libID := atomOf(libSym.Arg(1))
	showNum := !hideFlag(libSym.Child("pin_numbers"))
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
			if p := kicadPin(pn, showNum); p != nil {
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
// shapes, using conv to place points. Lib symbol graphics pass libPt (Y-up, no flip); sheet
// graphics pass sheetPt (Y-down -> flip).
func kicadShapes(n *node, conv func(*node) *geom.Point) []*geom.Shape {
	var out []*geom.Shape
	shape := func(kind geom.Shape_Kind, src *node, pts []*geom.Point, radius int64) *geom.Shape {
		fill, color := kicadFill(src)
		return &geom.Shape{Kind: kind, Fill: fill, FillColor: color, Points: pts, Radius: radius}
	}
	for _, r := range n.Children("rectangle") {
		out = append(out, shape(geom.Shape_KIND_RECT, r, []*geom.Point{conv(r.Child("start")), conv(r.Child("end"))}, 0))
	}
	for _, pl := range n.Children("polyline") {
		out = append(out, shape(geom.Shape_KIND_POLYLINE, pl, xyPoints(pl.Child("pts"), conv), 0))
	}
	for _, c := range n.Children("circle") {
		var radius int64
		if rn := c.Child("radius"); rn != nil {
			radius = mmToNm(atomOf(rn.Arg(1)))
		}
		out = append(out, shape(geom.Shape_KIND_CIRCLE, c, []*geom.Point{conv(c.Child("center"))}, radius))
	}
	for _, a := range n.Children("arc") {
		out = append(out, shape(geom.Shape_KIND_ARC, a, []*geom.Point{conv(a.Child("start")), conv(a.Child("mid")), conv(a.Child("end"))}, 0))
	}
	return out
}

// kicadFill maps a graphic's (fill (type ...)) onto the geom fill style. background/outline
// map to their named fills; an explicit (type color) yields FILL_COLOR + #rrggbb, except a
// fully transparent color (alpha 0, as KiCad's default) which is treated as unfilled.
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

// kicadPin reads a pin's connect point. KiCad's (pin ... (at x y ang) (number "N")) places
// the connection at (at); the pin number joins to ir.Pin.designator via port_ref. port_ref is
// always set (it is a join key); the pin-number label is placed only when showNum is true
// (the lib symbol does not hide pin numbers), so passives/power that hide numbers stay clean.
func kicadPin(pn *node, showNum bool) *geom.PinPoint {
	at := pn.Child("at")
	if at == nil {
		return nil
	}
	pp := &geom.PinPoint{Loc: libPt(at)}
	if num := pn.Child("number"); num != nil {
		pp.PortRef = atomOf(num.Arg(1))
		pp.SourceId = pp.PortRef
		if showNum {
			pp.LabelOrigin = pp.Loc
		}
	}
	if nm := pn.Child("name"); nm != nil {
		if n := atomOf(nm.Arg(1)); n != "~" { // "~" is KiCad's "no name"
			pp.Name = n
		}
	}
	return pp
}

// kicadPinLeg builds the pin leg: the stub line from the pin's connection point (at) into the
// symbol body. KiCad's (at x y angle)(length L) puts the connection at (at); the leg runs L in
// the angle direction toward the body (verified: a resistor's top pin at y=3.81, length 1.27,
// angle 270 -> body edge at y=2.54). Lib-local (Y-up, libPt), so it renders and transforms with
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
func fieldsOf(ps *node) []*geom.Field {
	var out []*geom.Field
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
		if a := p.Child("at").Arg(3); a != nil {
			if deg, err := strconv.ParseFloat(atomOf(a), 64); err == nil {
				f.RotationDeg = geomRotation(deg)
			}
		}
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

// hideFlag reports whether a lib-symbol (pin_numbers ...) / (pin_names ...) node hides its
// text. Both carry (hide yes) when the part suppresses that text (typical for passives).
func hideFlag(n *node) bool {
	if n == nil {
		return false
	}
	if h := n.Child("hide"); h != nil {
		return atomOf(h.Arg(1)) != "no"
	}
	return false
}

// placementOf builds a SymbolPlacement from a top-level (symbol ...) instance. cell_ref is
// the full lib_id and view_ref is the unit, so it joins to the per-unit SymbolDef. #-prefixed
// references (power/flag virtuals) are kept: they are drawn, they just do not join the IR.
func placementOf(ps *node, src string) *geom.SymbolPlacement {
	// A #-prefixed reference (#PWR, #FLG) is a virtual power/flag symbol: drawn like a part, but not
	// one, so it must not carry a ref_des that a consumer would try to join to a component.
	//
	// What it DOES carry is a name for the net at its pin — the same fact sch_nets.go turns into a
	// rank-0 anchor — so that goes in net_anchor and the glyph becomes addressable. Blanking the ref
	// alone (what this did) left the symbol drawn and anonymous, which made the thing that NAMES a
	// rail the one thing on the sheet a reader could not click.
	//
	// A PWR_FLAG is the exception: it asserts a net is driven and names nothing, exactly as the
	// netlist side treats it, so it gets no anchor.
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
	}
	if lid := ps.Child("lib_id"); lid != nil {
		pl.CellRef = atomOf(lid.Arg(1))
	}
	return pl
}

// kicadTransform maps a KiCad (at x y angle) plus optional (mirror x|y) onto the neutral
// Transform. Coordinates are converted (mm->nm, Y flipped) by pointMM.
func kicadTransform(ps *node) *geom.Transform {
	t := &geom.Transform{}
	if at := ps.Child("at"); at != nil {
		t.Origin = sheetPt(at)
		if a := at.Arg(3); a != nil {
			if deg, err := strconv.ParseFloat(atomOf(a), 64); err == nil {
				t.RotationDeg = geomRotation(deg)
			}
		}
	}
	if m := ps.Child("mirror"); m != nil {
		switch atomOf(m.Arg(1)) {
		case "x":
			t.MirrorX = true
		case "y":
			t.MirrorY = true
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
			if a := at.Arg(3); a != nil {
				if deg, err := strconv.ParseFloat(atomOf(a), 64); err == nil {
					l.RotationDeg = geomRotation(deg)
				}
			}
			out = append(out, l)
		}
	}
	return out
}

// kicadJustify maps a node's (effects (justify ...)) onto the canonical geom convention:
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
// mm dimensions; a "User" paper carries explicit width/height. The box is Y-up (Y flipped),
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
// name is not a standard size (e.g. "User", which carries explicit dimensions).
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

// KiCad uses two coordinate systems: library symbol graphics are Y-up (like the geom
// contract), while the schematic SHEET is Y-down. So lib-local points convert straight
// (libPt, no flip) and sheet-level points negate Y (sheetPt). Flipping lib points too — the
// original bug — vertically mirrors every symbol (e.g. an upside-down GND).

// libPt reads a node's first two args as KiCad millimeters into a geom point WITHOUT flipping
// Y: it is for library-symbol-local coordinates (shapes, pins), which are already Y-up.
func libPt(n *node) *geom.Point {
	if n == nil {
		return nil
	}
	return &geom.Point{X: mmToNm(atomOf(n.Arg(1))), Y: mmToNm(atomOf(n.Arg(2)))}
}

// sheetPt reads a node's first two args as KiCad millimeters into a geom point with Y negated:
// it is for sheet-level coordinates (placement origin, wires, labels), which are Y-down.
func sheetPt(n *node) *geom.Point {
	if n == nil {
		return nil
	}
	return &geom.Point{X: mmToNm(atomOf(n.Arg(1))), Y: -mmToNm(atomOf(n.Arg(2)))}
}

// geomRotation converts a KiCad rotation angle (degrees) to the geom contract's rotation.
// Converting KiCad's Y-down frame to the geom Y-up frame is a reflection, which inverts
// rotation direction, so the geom angle is -kicad (mod 360). Origin translation and mirror
// axes are unaffected by this (mirror conjugated by the Y-flip is itself), so only rotation
// flips. Without this, rotated symbols land mirrored about their origin and overlap.
func geomRotation(deg float64) int32 {
	r := math.Mod(360-deg, 360)
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
