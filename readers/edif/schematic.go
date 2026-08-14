package edif

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ReadSchematic parses an EDIF 2.0.0 SCHEMATIC export (.eds) into the geometry
// sidecar. It reuses the S-expression parser (sexpr.go) but is a separate extractor
// from the netlist reader (reader.go): it produces render geometry keyed to the core
// IR, never the IR itself.
//
// Fidelity: lossy-bounded (render subset), CONSTRAINTS C6. We extract symbol graphics,
// pin coordinates, placements, wire polylines, labels, and sheets. We drop the
// connectivity graph (it lives in the core IR from the .edn), component properties,
// the style palette, display metadata, and back-annotation. See
// docs/edif-schematic-primer.md and docs/16-geometry-and-rendering.md.
//
// This produces the tier-1 logical form (docs/16). The tier-2 columnar GPU blob is a
// downstream projection built by the renderer path, not here. sourceFile is recorded in
// provenance only; the caller owns file I/O so the core stays runtime-agnostic (C1).
func ReadSchematic(r io.Reader, sourceFile string) (*geom.SchematicGeometry, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	ver := edifVersion(root)
	if len(ver) > 0 && ver[0] != "2" {
		return nil, fmt.Errorf("edif: unsupported version %s (schematic reader supports 2.0.0)", strings.Join(ver, "."))
	}
	return extractGeom(root, sourceFile), nil
}

// extractGeom walks the parsed schematic tree and builds the geometry sidecar: the
// symbol library (drawn once per part type) and the sheets (placements, wires, labels).
func extractGeom(root *node, src string) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		Prov:   &geom.Provenance{SourceFile: src},
		UnitNm: distanceUnitNm(root),
	}
	if dn := findFirst(root, "design"); dn != nil {
		if _, disp := nameParts(dn.Arg(1)); disp != "" {
			g.DesignRef = disp
		}
	}
	if g.DesignRef == "" {
		g.DesignRef = atom(root.Arg(1))
	}

	// Symbols: library -> cell -> view -> symbol. Keyed by cell name + library name.
	// cellByID resolves a cell's internal &id to its display name: instances may reference
	// a cell by either form, but the sidecar joins on the display name (docs §8).
	cellByID := map[string]string{}
	// libByID does the same id->display normalization for library references, since a
	// (library (rename Ferrite_Bead "Ferrite Bead")) is keyed by display name but
	// referenced by instances via its id.
	libByID := map[string]string{}
	// viewByID normalizes a view's internal &id to its display name. A multi-section cell
	// defines several views (banks), and an instance selects one by view id.
	viewByID := map[string]string{}
	var libs []*node
	collect(root, "library", &libs)
	for _, lib := range libs {
		libID, libName := nameParts(lib.Arg(1))
		if libName == "" {
			libName = libID
		}
		if libID != "" {
			libByID[libID] = libName
		}
		libByID[libName] = libName
		for _, cell := range lib.Children("cell") {
			id, disp := nameParts(cell.Arg(1))
			name := disp
			if name == "" {
				name = id
			}
			if id != "" {
				cellByID[id] = name
			}
			cellByID[name] = name
			// One symbol per view: a multi-section cell has several views (banks), each
			// its own graphic. Prefer a (symbol ...) node; fall back to a GRAPHIC view's
			// (contents ...), where builtin cells keep their figures directly.
			for _, view := range cell.Children("view") {
				vID, vDisp := nameParts(view.Arg(1))
				vName := vDisp
				if vName == "" {
					vName = vID
				}
				if vID != "" {
					viewByID[vID] = vName
				}
				viewByID[vName] = vName
				gnode := findFirst(view, "symbol")
				if gnode == nil {
					gnode = graphicContents(view)
				}
				if gnode == nil {
					continue
				}
				g.Symbols = append(g.Symbols, symbolOf(gnode, cell, view, libName, vName, src))
			}
		}
	}

	// Technology default text heights per figureGroup: a placed field that overrides a group
	// without restating its height inherits the group default (see placedFields).
	fgh := figureGroupHeights(root)

	// Sheets: each (page ...) holds placements, nets (wires), and annotations.
	var pages []*node
	collect(root, "page", &pages)
	for _, p := range pages {
		g.Sheets = append(g.Sheets, sheetOf(p, src, cellByID, libByID, viewByID, fgh))
	}
	return g
}

// textLineRatio converts an EDIF textHeight into the GLYPH height geom.Field/Label carry.
//
// EDIF's textHeight is a LINE PITCH, not an em size. The authoring tool stacks a component's
// field rows exactly textHeight apart: measured over 2377 instances of one export, 3161 of the
// same-column row gaps were exactly 1.000x that row's textHeight, far ahead of any other value.
// Setting font-size = textHeight therefore leaves ZERO leading, so consecutive rows touch, which
// is what made rendered field columns look cramped against the tool's own printed output.
//
// The ratio is the tool's line height. Recovered from a PDF the same toolchain printed, it is
// 1.3148: every one of nine distinct font sizes in that PDF divided its source textHeight to
// within 0.1%, so it is calibrated against nine points rather than fitted to one. It also lands
// where a line height should — 1.2 to 1.35 is ordinary typesetting — which is the sanity check
// that this is a real quantity and not a fudge factor.
//
// Applied in the READER, not the renderer, because it is a fact about how EDIF spells text size.
// geom's height field means glyph height (the renderer maps it straight to font-size), so
// translating the format's spelling into the contract's meaning is exactly the reader's job.
// KiCad states a glyph height directly and is correctly left alone.
const textLineRatio = 1.3148

// glyphHeight converts one EDIF textHeight (a line pitch) to the glyph height geom carries.
// A zero or negative height passes through unchanged, so "unspecified" stays unspecified and
// the renderer's own fallback still applies.
func glyphHeight(textHeight int64) int64 {
	if textHeight <= 0 {
		return textHeight
	}
	return int64(float64(textHeight)/textLineRatio + 0.5)
}

// figureGroupHeights reads the technology's default textHeight per figureGroup, in source units.
// EDIF records a display's height once on the figureGroup (e.g. (figureGroup ATTRIBUTE (textHeight
// 254000))); a per-instance (figureGroupOverride GROUP) that changes only color/visibility omits
// the height and inherits the group default. Without this map a placed field with no restated
// height falls to the renderer's fixed pixel fallback and does not scale with the sheet.
// If two libraries declare one group name with different heights, the first definition wins,
// so the map is deterministic.
func figureGroupHeights(root *node) map[string]int64 {
	m := map[string]int64{}
	var fgs []*node
	collect(root, "figureGroup", &fgs)
	for _, fg := range fgs {
		name := atom(fg.Arg(1))
		if name == "" {
			continue
		}
		if _, seen := m[name]; seen {
			continue // first definition wins, so a repeated group name is deterministic
		}
		if th := fg.Child("textHeight"); th != nil {
			if h := glyphHeight(parseInt(atom(th.Arg(1)))); h > 0 {
				m[name] = h
			}
		}
	}
	return m
}

// displayGroup returns the figureGroup a (display ...) overrides via (figureGroupOverride GROUP),
// or "" when it overrides none. Used to inherit the group's default text height.
func displayGroup(d *node) string {
	if fg := d.Child("figureGroupOverride"); fg != nil {
		return atom(fg.Arg(1))
	}
	return ""
}

// graphicContents returns a view's (contents ...) node when it holds drawable figures
// directly (no (symbol ...) child), as builtin GRAPHIC cells (GND, no-connect, off-page
// connectors) do. Returns nil for a symbol-bearing or figureless view.
func graphicContents(view *node) *node {
	c := view.Child("contents")
	if c == nil || c.Child("symbol") != nil {
		return nil
	}
	if len(c.Children("figure")) > 0 {
		return c
	}
	return nil
}

// symbolOf builds a SymbolDef from a cell's geometry node (a (symbol ...) view, or a
// GRAPHIC view's (contents ...) for builtin cells): its bounding box, drawn shapes, and
// pin connect-locations (symbol-local coordinates).
func symbolOf(sym, cell, view *node, libName, viewName, src string) *geom.SymbolDef {
	id, disp := nameParts(cell.Arg(1))
	name := disp
	if name == "" {
		name = id
	}
	sd := &geom.SymbolDef{
		CellRef:    name,
		LibraryRef: libName,
		ViewRef:    viewName,
		Bbox:       bboxOf(sym.Child("boundingBox")),
		Prov:       &geom.Provenance{SourceFile: src, SourceId: id},
	}
	// Shapes: figures that are direct children of the symbol.
	for _, f := range sym.Children("figure") {
		sd.Shapes = append(sd.Shapes, shapesOfFigure(f)...)
	}
	// Pins and their stub figures come from each portImplementation.
	for _, pi := range sym.Children("portImplementation") {
		if pin := pinOf(pi); pin != nil {
			sd.Pins = append(sd.Pins, pin)
		}
		for _, f := range pi.Children("figure") {
			sd.Shapes = append(sd.Shapes, shapesOfFigure(f)...)
		}
	}
	// Static free-text annotations on the view (title-block field labels and other baked
	// text). These are symbol-local and drawn transformed by each placement.
	var anns []*node
	collect(view, "annotate", &anns)
	for _, an := range anns {
		if l := labelOf(an); l != nil {
			sd.Annotations = append(sd.Annotations, l)
		}
	}
	return sd
}

// pinOf extracts a pin's connect location (where a wire attaches) from a
// portImplementation node.
func pinOf(pi *node) *geom.PinPoint {
	nm := pi.Child("name")
	portRef := ""
	if nm != nil {
		portRef = strings.TrimPrefix(atom(nm.Arg(1)), "&")
	}
	cl := pi.Child("connectLocation")
	if cl == nil {
		return nil
	}
	var loc *geom.Point
	if f := cl.Child("figure"); f != nil {
		if d := f.Child("dot"); d != nil {
			loc = ptOf(d.Arg(1))
		}
	}
	if loc == nil {
		return nil
	}
	pp := &geom.PinPoint{PortRef: portRef, Loc: loc, SourceId: portRef}
	// The pin-number label is the port name shown at (name X (display (origin ...))),
	// in symbol-local coordinates. Skip it when the source marks the label hidden.
	if nm != nil {
		if d := nm.Child("display"); d != nil && labelVisible(d) {
			if o := d.Child("origin"); o != nil {
				pp.LabelOrigin = ptOf(o.Arg(1))
			}
			if j := d.Child("justify"); j != nil {
				pp.Justify = canonicalJustify(atom(j.Arg(1)))
			}
		}
	}
	return pp
}

// canonicalJustify maps an EDIF justify code ({UPPER,CENTER,LOWER}{LEFT,CENTER,RIGHT}) onto
// the canonical geom convention "<h> <v>" (h in {left,right}, v in {top,bottom}, each omitted
// when centered). E.g. LOWERLEFT -> "left bottom", CENTERLEFT -> "left", CENTERCENTER -> "".
func canonicalJustify(code string) string {
	h, v := "", ""
	switch {
	case strings.Contains(code, "LEFT"):
		h = "left"
	case strings.Contains(code, "RIGHT"):
		h = "right"
	}
	switch {
	case strings.HasPrefix(code, "UPPER"):
		v = "top"
	case strings.HasPrefix(code, "LOWER"):
		v = "bottom"
	}
	return strings.TrimSpace(h + " " + v)
}

// labelVisible reports whether a (display ...) is shown. A nested (visible (false)) (from
// a figureGroupOverride) hides it; absent visibility means visible.
func labelVisible(display *node) bool {
	if v := findFirst(display, "visible"); v != nil && v.Arg(1) != nil && v.Arg(1).Head() == "false" {
		return false
	}
	return true
}

// shapesOfFigure converts an EDIF (figure GROUP shape...) node into geometry shapes.
// The GROUP (BOX/PIN/NET/...) is carried as a style hint. connectLocation dots are
// skipped here (they are captured as pins).
func shapesOfFigure(f *node) []*geom.Shape {
	group := ""
	// The figure's first child may be a bare group atom, e.g. (figure BOX ...).
	if a := f.Arg(1); a != nil && !a.IsList {
		group = a.Atom
	}
	var out []*geom.Shape
	for _, k := range f.Kids {
		if !k.IsList {
			continue
		}
		switch k.Head() {
		case "rectangle":
			out = append(out, &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: ptsOf(k), FigureGroup: group})
		case "path":
			if pl := k.Child("pointList"); pl != nil {
				out = append(out, &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: ptsOf(pl), FigureGroup: group})
			}
		case "circle":
			out = append(out, circleShape(k, group))
		case "dot":
			out = append(out, &geom.Shape{Kind: geom.Shape_KIND_DOT, Points: ptsOf(k), FigureGroup: group})
		case "openShape":
			if c := k.Child("curve"); c != nil {
				for _, a := range c.Children("arc") {
					out = append(out, &geom.Shape{Kind: geom.Shape_KIND_ARC, Points: ptsOf(a), FigureGroup: group})
				}
			}
		}
	}
	return out
}

// circleShape converts an EDIF (circle pt1 pt2) (two diameter-endpoint points) into a
// center+radius CIRCLE shape. Sub-unit rounding is negligible at the 10nm grid.
func circleShape(k *node, group string) *geom.Shape {
	pts := ptsOf(k)
	if len(pts) < 2 {
		return &geom.Shape{Kind: geom.Shape_KIND_CIRCLE, Points: pts, FigureGroup: group}
	}
	cx := (pts[0].X + pts[1].X) / 2
	cy := (pts[0].Y + pts[1].Y) / 2
	r := iabs(pts[1].X-pts[0].X)/2 + iabs(pts[1].Y-pts[0].Y)/2 // one axis is 0 for an axis-aligned diameter
	return &geom.Shape{Kind: geom.Shape_KIND_CIRCLE, Points: []*geom.Point{{X: cx, Y: cy}}, Radius: r, FigureGroup: group}
}

// sheetOf builds a SheetGeometry from a (page ...) node. cellByID/libByID/viewByID
// normalize each placement's cell, library, and view references to display names (see
// placementOf).
func sheetOf(p *node, src string, cellByID, libByID, viewByID map[string]string, fgh map[string]int64) *geom.SheetGeometry {
	id, disp := nameParts(p.Arg(1))
	sh := &geom.SheetGeometry{
		Id:   id,
		Name: disp,
		Prov: &geom.Provenance{SourceFile: src, SourceId: id},
	}
	if ps := p.Child("pageSize"); ps != nil {
		sh.Size = bboxOf(ps)
	}
	// Instances may be direct children of the page or nested inside nets (power/ground
	// and off-page connector symbols are placed within their net), so collect the whole
	// page subtree. Instances never nest, so this does not double-count.
	var insts []*node
	collect(p, "instance", &insts)
	for _, in := range insts {
		// The drawing-sheet border/title-block is a placed symbol whose per-sheet field values
		// (title, rev, date) ride on the instance's (property ...) overrides. Promote those into
		// the sheet's TitleBlock and drop the instance: the worksheet frame is synthesized from
		// the page size, so drawing the raw border cell too would double the frame, and its field
		// captions would double the title-block text (WS7-019).
		if tb := titleBlockFromInstance(in, cellByID, libByID); tb != nil {
			if sh.TitleBlock == nil {
				sh.TitleBlock = tb
			}
			continue
		}
		pl := placementOf(in, src, cellByID, libByID, viewByID)
		// Ref-des is a structured Reference field (the renderer draws placement fields).
		if f := refDesField(in, pl.RefDes, placementOrigin(pl), fgh); f != nil {
			pl.Fields = append(pl.Fields, f)
		}
		// Placed property overrides (Value, tolerance, rating) carry their own display origin
		// on the instance, so a faithful .eds render shows component values, not just symbols
		// (WS1-037).
		pl.Fields = append(pl.Fields, placedFields(in, fgh)...)
		sh.Placements = append(sh.Placements, pl)
	}
	for _, nn := range p.Children("net") {
		if w := wireOf(nn, src); w != nil {
			sh.Wires = append(sh.Wires, w)
		}
	}
	// Labels and free graphics in the page's commentGraphics. Figures here are stand-alone
	// sheet drawings (not owned by a placed symbol), so they go in the sheet's shape list.
	for _, cg := range p.Children("commentGraphics") {
		for _, an := range cg.Children("annotate") {
			if l := labelOf(an); l != nil {
				sh.Labels = append(sh.Labels, l)
			}
		}
		for _, f := range cg.Children("figure") {
			sh.Shapes = append(sh.Shapes, shapesOfFigure(f)...)
		}
	}
	// Net-stub labels: an off-page connector is a page-level portImplementation whose
	// (name X (display ...)) places the signal name near the connector. (Symbol pins live
	// in cells, not pages, so a page's portImplementations are the off-page connectors.)
	var pis []*node
	collect(p, "portImplementation", &pis)
	for _, pi := range pis {
		if l := labelFromNameDisplay(pi.Child("name")); l != nil {
			sh.Labels = append(sh.Labels, l)
		}
	}
	return sh
}

// placementOf builds a SymbolPlacement from an (instance ...) node on a sheet. The cell
// reference is normalized through cellByID: an instance may name its cell by display name
// or by internal &id, but the sidecar always joins on the display name (docs §8), so a
// placement resolves to its SymbolDef regardless of which form the source used.
func placementOf(in *node, src string, cellByID, libByID, viewByID map[string]string) *geom.SymbolPlacement {
	id, _ := nameParts(in.Arg(1))
	pl := &geom.SymbolPlacement{
		RefDes: refDesOf(in),
		Prov:   &geom.Provenance{SourceFile: src, SourceId: id},
	}
	if v := in.Child("viewRef"); v != nil {
		// The viewRef names the view (bank) this instance uses; normalize its id to the
		// display name so it joins to SymbolDef.view_ref.
		viewRef := atom(v.Arg(1))
		if name, ok := viewByID[viewRef]; ok {
			viewRef = name
		}
		pl.ViewRef = viewRef
		if cr := v.Child("cellRef"); cr != nil {
			ref := cellRefName(cr)
			if name, ok := cellByID[ref]; ok {
				ref = name
			}
			pl.CellRef = ref
			if lr := cr.Child("libraryRef"); lr != nil {
				libRef := atom(lr.Arg(1))
				if name, ok := libByID[libRef]; ok {
					libRef = name
				}
				pl.LibraryRef = libRef
			}
		}
	}
	pl.Transform = transformOf(in.Child("transform"))
	return pl
}

// placedFields reads an instance's DRAWN (property ...) overrides into geom.Field entries: the
// component values/tolerances/ratings a faithful .eds render must show (WS1-037). Only a
// property placed with a stringDisplay + display origin becomes a Field — a data-only property
// (no stringDisplay) rides the netlist .edn attributes, not the drawing. The Reference
// designator is synthesized by the caller from the placement, so a Reference/RefDes property is
// skipped here to avoid a doubled ref-des on the sheet.
//
// A property display carries a source visibility flag, the same one pin labels honor: the
// authoring tool records a display origin for nearly every property but marks most (visible
// (false)), and only the few it actually draws (typically Value) visible. A hidden property is
// skipped, so a faithful render shows what the tool shows instead of flooding the sheet with every
// attribute's hidden text.
//
// A visible property that omits its own textHeight inherits the default height of the figureGroup
// its display overrides (fgh, keyed by group name), so a kept field scales with the sheet instead
// of falling to the renderer's fixed pixel fallback.
func placedFields(in *node, fgh map[string]int64) []*geom.Field {
	var out []*geom.Field
	for _, p := range in.Children("property") {
		id, disp := nameParts(p.Arg(1))
		name := disp
		if name == "" {
			name = id
		}
		name = strings.TrimPrefix(name, "&")
		if base := baseFieldKey(name); base == "REFERENCE" || base == "REFDES" {
			continue
		}
		sd := p.Child("string").Child("stringDisplay")
		d := sd.Child("display")
		o := d.Child("origin")
		if o == nil {
			continue
		}
		// Honor the source visibility flag (matches pinOf): the tool draws only a few properties.
		if !labelVisible(d) {
			continue
		}
		if f := fieldFromDisplay(name, atom(sd.Arg(1)), d, fgh); f != nil {
			out = append(out, f)
		}
	}
	return out
}

// fieldFromDisplay builds one placed Field from a (display ...) node: its origin, justify,
// orientation, and text height, inheriting the overridden figureGroup's default height when the
// display does not restate one. Returns nil when the display has no origin (an unplaced value) or
// the source marks it hidden, so every caller drops the same things. Shared by placedFields and
// refDesField, which read the same display grammar off different parents.
func fieldFromDisplay(name, value string, d *node, fgh map[string]int64) *geom.Field {
	o := d.Child("origin")
	if o == nil || !labelVisible(d) {
		return nil
	}
	f := &geom.Field{Name: name, Value: value, Origin: ptOf(o.Arg(1)), Visible: true}
	if j := d.Child("justify"); j != nil {
		f.Justify = canonicalJustify(atom(j.Arg(1)))
	}
	if or := d.Child("orientation"); or != nil {
		f.RotationDeg = orientationDeg(atom(or.Arg(1)))
	}
	if th := findFirst(d, "textHeight"); th != nil {
		f.Height = glyphHeight(parseInt(atom(th.Arg(1))))
	}
	if f.Height == 0 {
		f.Height = fgh[displayGroup(d)]
	}
	return f
}

// refDesField builds the ref-des Reference field for an instance. A schematic designator carries
// its own display — origin, justify, and text height — so the ref-des lands where the authoring
// tool drew it, which is typically a column of fields beside the symbol rather than the symbol
// origin. Anchoring it at the placement origin instead put it a line or two off and at the
// renderer's fixed fallback size, so it collided with the neighbouring component's fields.
//
// A display is authoritative when present, including its visibility flag: an instance that hides
// its designator keeps it hidden rather than falling back. The fallback covers an export whose
// designator is a bare (designator REF) with no display at all, which has no position of its own.
func refDesField(in *node, refDes string, fallback *geom.Point, fgh map[string]int64) *geom.Field {
	if refDes == "" {
		return nil
	}
	if d := in.Child("designator").Child("stringDisplay").Child("display"); d != nil {
		return fieldFromDisplay("Reference", refDes, d, fgh)
	}
	if fallback == nil {
		return nil
	}
	return &geom.Field{Name: "Reference", Value: refDes, Origin: fallback, Visible: true}
}

// placementOrigin is a placement's transform origin, or nil when it has none.
func placementOrigin(pl *geom.SymbolPlacement) *geom.Point {
	if pl == nil || pl.Transform == nil {
		return nil
	}
	return pl.Transform.Origin
}

// titleBlockFromInstance recognizes the drawing-sheet border/title-block instance and pulls
// its field values into a TitleBlock, or returns nil when the instance is an ordinary symbol.
// EDIF has no structured title-block tags; the fields ride on the border cell instance as
// (property KEY (string (stringDisplay VALUE))) overrides (title, rev, date), so extraction is
// inference by field-name (WS7-019). It is lossy-bounded (C6): unrecognized or placeholder
// values are left empty. A recognized border cell returns a non-nil TitleBlock even when every
// field is empty, so the caller still drops the raw border symbol (the frame is synthesized).
func titleBlockFromInstance(in *node, cellByID, libByID map[string]string) *geom.TitleBlock {
	cell, lib := instanceCellLib(in, cellByID, libByID)
	if !isTitleBlockCell(cell, lib) {
		return nil
	}
	tb := &geom.TitleBlock{}
	seenExtra := map[string]bool{}
	for _, p := range in.Children("property") {
		id, disp := nameParts(p.Arg(1))
		key := disp
		if key == "" {
			key = id
		}
		val := strings.TrimSpace(propText(p))
		if val == "" || isTitleBlockPlaceholder(val) {
			continue
		}
		base := baseFieldKey(key)
		switch base {
		case "TITLE":
			if tb.Title == "" {
				tb.Title = val
			}
		case "REV":
			if tb.Rev == "" {
				tb.Rev = val
			}
		case "DATE":
			if tb.Date == "" {
				tb.Date = val
			}
		case "COMPANY":
			if tb.Company == "" {
				tb.Company = val
			}
		default:
			// Any other property on the border cell is a title-block field with no typed slot
			// (Drawing, Designer, Prototype, DV/PV/Checked signatures). Preserve it rather than
			// drop it (C6 lossy-bounded), keyed by base name so DV_1/DV_2 collapse, first
			// non-empty wins, in source order.
			if !seenExtra[base] {
				seenExtra[base] = true
				tb.ExtraFields = append(tb.ExtraFields, &geom.KeyValue{Key: base, Value: val})
			}
		}
	}
	return tb
}

// instanceCellLib resolves an instance's referenced cell and library to their display names
// (normalized through cellByID/libByID like placementOf), or "" when absent.
func instanceCellLib(in *node, cellByID, libByID map[string]string) (cell, lib string) {
	v := in.Child("viewRef")
	if v == nil {
		return "", ""
	}
	cr := v.Child("cellRef")
	if cr == nil {
		return "", ""
	}
	cell = cellRefName(cr)
	if name, ok := cellByID[cell]; ok {
		cell = name
	}
	if lr := cr.Child("libraryRef"); lr != nil {
		lib = atom(lr.Arg(1))
		if name, ok := libByID[lib]; ok {
			lib = name
		}
	}
	return cell, lib
}

// isTitleBlockCell reports whether a cell/library reference names a drawing-sheet border or
// title-block symbol. Cadence/Allegro exports place these from a "Borders" library and name
// the cell after the block (e.g. GM_TitleBlock_D_Org); the match is a heuristic, since EDIF has
// no viewType or tag that marks the drawing frame.
func isTitleBlockCell(cell, lib string) bool {
	c := strings.ToLower(cell)
	return strings.EqualFold(lib, "borders") ||
		strings.Contains(c, "titleblock") || strings.Contains(c, "title_block")
}

// propText extracts a property's scalar value. Since WS1-046 it delegates to propValue, which
// unwraps the schematic (string (stringDisplay "V" ...)) wrapper as well as the plain
// (string "V") / integer / boolean forms; kept as the geometry-side call name.
func propText(p *node) string {
	return propValue(p)
}

// baseFieldKey uppercases a property key and strips a trailing sequence suffix (an optional
// "_" then digits), so REV_1..REV_5 and DATE_1 collapse to REV and DATE. It maps only the base
// name, so a distinct key like CHANGE_DATE_1 stays CHANGE_DATE and is not read as the date.
func baseFieldKey(k string) string {
	u := strings.ToUpper(strings.TrimSpace(k))
	i := len(u)
	for i > 0 && u[i-1] >= '0' && u[i-1] <= '9' {
		i--
	}
	return strings.TrimSuffix(u[:i], "_")
}

// isTitleBlockPlaceholder reports whether a value is an all-dashes placeholder (e.g. "---"),
// which title-block exports use for an unset field. Treated as empty so it does not populate
// TitleBlock.
func isTitleBlockPlaceholder(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	return strings.Trim(v, "-") == ""
}

// wireOf collects the routed wire polylines for one net on a sheet. Nets nest (an outer
// logical net, inner physical net-segment groups carrying the wires); the wires are
// gathered from the whole subtree and keyed by the outer net name.
func wireOf(n *node, src string) *geom.WireGeometry {
	// Name the wire by its net, preferring the display name and falling back to the id, the same
	// resolution the netlist read uses (nm.best) so the geometry wire.Net EQUALS the ir.Net name a
	// finding carries. A bare (name ID) net has no display, so the id is the join key; discarding it
	// (the old code kept only the display, then atom(), which is empty for the (name ...) compound)
	// left every .eds wire unnamed and made net-subject findings unlocatable on the .eds canvas
	// (WS1-047: the .edn is analysis truth, the .eds a companion joined BY NET NAME).
	id, disp := nameParts(n.Arg(1))
	name := disp
	if name == "" {
		name = id
	}
	w := &geom.WireGeometry{Net: name, Prov: &geom.Provenance{SourceFile: src}}
	var figs []*node
	collect(n, "figure", &figs)
	for _, f := range figs {
		if pl := f.Child("path"); pl != nil {
			if pts := pl.Child("pointList"); pts != nil {
				w.Polylines = append(w.Polylines, &geom.Polyline{Points: ptsOf(pts)})
			}
		}
	}
	if len(w.Polylines) == 0 {
		return nil
	}
	return w
}

// labelOf builds a Label from an (annotate (stringDisplay TEXT (display ...))) node.
func labelOf(an *node) *geom.Label {
	sd := an.Child("stringDisplay")
	if sd == nil {
		return nil
	}
	l := &geom.Label{Text: atom(sd.Arg(1))}
	if d := sd.Child("display"); d != nil {
		if o := d.Child("origin"); o != nil {
			l.Origin = ptOf(o.Arg(1))
		}
		if j := d.Child("justify"); j != nil {
			l.Justify = canonicalJustify(atom(j.Arg(1)))
		}
		if or := d.Child("orientation"); or != nil {
			l.RotationDeg = orientationDeg(atom(or.Arg(1)))
		}
		if th := findFirst(d, "textHeight"); th != nil {
			l.Height = glyphHeight(parseInt(atom(th.Arg(1))))
		}
	}
	return l
}

// labelFromNameDisplay builds a Label from a (name X (display ...)) node, as an off-page
// connector uses to place its signal name. The text is the name; position/justify come
// from the display. Returns nil when there is no display with an origin (an unplaced name).
func labelFromNameDisplay(nm *node) *geom.Label {
	if nm == nil {
		return nil
	}
	id, disp := nameParts(nm.Arg(1))
	text := disp
	if text == "" {
		text = id
	}
	text = strings.TrimPrefix(text, "&")
	d := nm.Child("display")
	if text == "" || d == nil {
		return nil
	}
	l := &geom.Label{Text: text}
	if o := d.Child("origin"); o != nil {
		l.Origin = ptOf(o.Arg(1))
	}
	if j := d.Child("justify"); j != nil {
		l.Justify = canonicalJustify(atom(j.Arg(1)))
	}
	if or := d.Child("orientation"); or != nil {
		l.RotationDeg = orientationDeg(atom(or.Arg(1)))
	}
	if th := findFirst(d, "textHeight"); th != nil {
		l.Height = glyphHeight(parseInt(atom(th.Arg(1))))
	}
	if l.Origin == nil {
		return nil
	}
	return l
}

// transformOf builds a format-neutral Transform from an EDIF (transform ...) node,
// mapping the orientation code to rotation/mirror, origin to a point, and the rare
// (scaleX num den)/(scaleY num den) to scale factors.
func transformOf(tn *node) *geom.Transform {
	t := &geom.Transform{}
	if tn == nil {
		return t
	}
	if o := tn.Child("orientation"); o != nil {
		applyOrientation(atom(o.Arg(1)), t)
	}
	if o := tn.Child("origin"); o != nil {
		t.Origin = ptOf(o.Arg(1))
	}
	if sx := tn.Child("scaleX"); sx != nil {
		t.ScaleX = ratioValue(sx)
	}
	if sy := tn.Child("scaleY"); sy != nil {
		t.ScaleY = ratioValue(sy)
	}
	return t
}

// applyOrientation maps an EDIF orientation code onto rotation/mirror. Mirror is
// applied before rotation (matching the EDIF MxR90 composition).
func applyOrientation(code string, t *geom.Transform) {
	switch code {
	case "R90":
		t.RotationDeg = 90
	case "R180":
		t.RotationDeg = 180
	case "R270":
		t.RotationDeg = 270
	case "MX":
		t.MirrorX = true
	case "MY":
		t.MirrorY = true
	case "MXR90":
		t.MirrorX = true
		t.RotationDeg = 90
	case "MYR90":
		t.MirrorY = true
		t.RotationDeg = 90
	}
}

// ratioValue reads an EDIF (scaleX num den) style node as num/den (den defaulting to 1).
func ratioValue(n *node) float64 {
	num := numberValue(n.Arg(1))
	den := numberValue(n.Arg(2))
	if den == 0 {
		return num
	}
	return num / den
}

// refDesOf reads the reference designator from an instance's (designator ...) node, which on a
// schematic wraps a stringDisplay. Delegates to stringDisplayText so both views unwrap identically
// (a nil designator yields "").
func refDesOf(in *node) string {
	return stringDisplayText(in.Child("designator"))
}

// cellRefName resolves a cellRef target, which is either a bare atom or a
// (name X (display ...)) form.
func cellRefName(cr *node) string {
	a := cr.Arg(1)
	if a == nil {
		return ""
	}
	if !a.IsList {
		return a.Atom
	}
	if a.Head() == "name" {
		return atom(a.Arg(1))
	}
	return ""
}

// bboxOf reads a bounding box from a node containing a (rectangle (pt) (pt)) child.
func bboxOf(n *node) *geom.BBox {
	if n == nil {
		return nil
	}
	r := n
	if rc := n.Child("rectangle"); rc != nil {
		r = rc
	}
	pts := ptsOf(r)
	if len(pts) < 2 {
		return nil
	}
	return &geom.BBox{Min: pts[0], Max: pts[1]}
}

// ptsOf returns every (pt X Y) child of a node, in order.
func ptsOf(n *node) []*geom.Point {
	var out []*geom.Point
	for _, k := range n.Kids {
		if k.IsList && k.Head() == "pt" {
			out = append(out, ptOf(k))
		}
	}
	return out
}

// ptOf parses a single (pt X Y) node.
func ptOf(n *node) *geom.Point {
	if n == nil || !n.IsList || n.Head() != "pt" {
		return nil
	}
	return &geom.Point{X: parseInt(atom(n.Arg(1))), Y: parseInt(atom(n.Arg(2)))}
}

// orientationDeg maps an EDIF orientation code to a rotation in degrees CCW, ignoring
// mirror (used for labels, which do not mirror).
func orientationDeg(code string) int32 {
	switch code {
	case "R90", "MXR90", "MYR90":
		return 90
	case "R180":
		return 180
	case "R270":
		return 270
	}
	return 0
}

func parseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func iabs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// distanceUnitNm reads the DISTANCE scale from the technology and returns nanometers per
// source unit (10 for this EDIF's (scale 1 (e 1 -8) (unit DISTANCE))). Returns 0 if
// absent.
func distanceUnitNm(root *node) int64 {
	var scales []*node
	collect(root, "scale", &scales)
	for _, s := range scales {
		u := s.Child("unit")
		if u == nil || atom(u.Arg(1)) != "DISTANCE" {
			continue
		}
		// (scale <numUnits> <meters> (unit DISTANCE)); meters may be an (e m exp) form.
		num := float64(parseInt(atom(s.Arg(1))))
		if num == 0 {
			num = 1
		}
		meters := numberValue(s.Arg(2))
		if meters == 0 {
			continue
		}
		return int64((meters / num) * 1e9)
	}
	return 0
}

// numberValue reads an EDIF number that is either a bare integer or an (e mantissa
// exponent) form (mantissa * 10^exponent).
func numberValue(n *node) float64 {
	if n == nil {
		return 0
	}
	if !n.IsList {
		v, _ := strconv.ParseFloat(n.Atom, 64)
		return v
	}
	if n.Head() == "e" {
		m, _ := strconv.ParseFloat(atom(n.Arg(1)), 64)
		exp, _ := strconv.ParseFloat(atom(n.Arg(2)), 64)
		return m * math.Pow(10, exp)
	}
	return 0
}
