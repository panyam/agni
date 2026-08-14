package edif

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// readFixture loads a testdata file, failing the test if it is missing.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// TestReadSchematic_MultiSheet asserts a design with two (page ...) nodes yields two sheets,
// in document order, with each page's id/name and its own placements.
func TestReadSchematic_MultiSheet(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "multisheet.eds")), "multisheet.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Sheets) != 2 {
		t.Fatalf("sheets = %d, want 2", len(g.Sheets))
	}
	if g.Sheets[0].GetId() != "P1" || g.Sheets[0].GetName() != "Sheet 1" {
		t.Errorf("sheet 0 = (%q,%q), want (P1, Sheet 1)", g.Sheets[0].GetId(), g.Sheets[0].GetName())
	}
	if g.Sheets[1].GetId() != "P2" || g.Sheets[1].GetName() != "Sheet 2" {
		t.Errorf("sheet 1 = (%q,%q), want (P2, Sheet 2)", g.Sheets[1].GetId(), g.Sheets[1].GetName())
	}
	// The pages differ (1 vs 2 placements), so the sheets are distinct, not duplicated.
	if a, b := len(g.Sheets[0].Placements), len(g.Sheets[1].Placements); a != 1 || b != 2 {
		t.Errorf("placements = (%d,%d), want (1,2)", a, b)
	}
}

func TestReadSchematic_Sample(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "sample.eds")), "sample.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}

	if g.DesignRef != "TopDesign" {
		t.Errorf("design_ref = %q, want TopDesign", g.DesignRef)
	}
	if g.UnitNm != 10 {
		t.Errorf("unit_nm = %d, want 10", g.UnitNm)
	}

	// Symbol library.
	if len(g.Symbols) != 1 {
		t.Fatalf("symbols = %d, want 1", len(g.Symbols))
	}
	sym := g.Symbols[0]
	if sym.CellRef != "PartA" || sym.LibraryRef != "Lib1" {
		t.Errorf("symbol key = %q/%q, want PartA/Lib1", sym.CellRef, sym.LibraryRef)
	}
	if sym.Bbox.GetMax().GetX() != 200 || sym.Bbox.GetMin().GetY() != -100 {
		t.Errorf("symbol bbox = %v", sym.Bbox)
	}
	// BOX rect + ARC + PIN stub path = 3 shapes; the connectLocation dot is a pin.
	if len(sym.Shapes) != 3 {
		t.Fatalf("symbol shapes = %d, want 3", len(sym.Shapes))
	}
	kinds := map[geom.Shape_Kind]int{}
	for _, s := range sym.Shapes {
		kinds[s.Kind]++
	}
	if kinds[geom.Shape_KIND_RECT] != 1 || kinds[geom.Shape_KIND_ARC] != 1 || kinds[geom.Shape_KIND_POLYLINE] != 1 {
		t.Errorf("shape kinds = %v", kinds)
	}
	if len(sym.Pins) != 1 {
		t.Fatalf("symbol pins = %d, want 1", len(sym.Pins))
	}
	if p := sym.Pins[0]; p.PortRef != "1" || p.Loc.GetX() != 200 || p.Loc.GetY() != -50 {
		t.Errorf("pin = %v", p)
	}

	// Sheet.
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	sh := g.Sheets[0]
	if sh.Name != "Sheet 1" {
		t.Errorf("sheet name = %q, want 'Sheet 1'", sh.Name)
	}
	if sh.Size.GetMax().GetX() != 1000 || sh.Size.GetMax().GetY() != 800 {
		t.Errorf("sheet size = %v", sh.Size)
	}

	// Placement + transform.
	if len(sh.Placements) != 1 {
		t.Fatalf("placements = %d, want 1", len(sh.Placements))
	}
	pl := sh.Placements[0]
	if pl.RefDes != "R1" || pl.CellRef != "PartA" || pl.LibraryRef != "Lib1" {
		t.Errorf("placement keys = %q/%q/%q", pl.RefDes, pl.CellRef, pl.LibraryRef)
	}
	tf := pl.Transform
	if !tf.MirrorY || tf.MirrorX || tf.RotationDeg != 90 {
		t.Errorf("transform orientation = %+v, want MirrorY+90", tf)
	}
	if tf.Origin.GetX() != 300 || tf.Origin.GetY() != 400 {
		t.Errorf("transform origin = %v", tf.Origin)
	}
	if tf.ScaleX != 0.5 || tf.ScaleY != 0.5 {
		t.Errorf("transform scale = %v/%v, want 0.5/0.5", tf.ScaleX, tf.ScaleY)
	}

	// Wire (keyed by the outer net name) and label.
	if len(sh.Wires) != 1 {
		t.Fatalf("wires = %d, want 1", len(sh.Wires))
	}
	if w := sh.Wires[0]; w.Net != "GND" || len(w.Polylines) != 1 || len(w.Polylines[0].Points) != 2 {
		t.Errorf("wire = %v", w)
	}
	// Just the comment annotation as a sheet label now; ref-des is a placement field.
	if len(sh.Labels) != 1 {
		t.Fatalf("labels = %d, want 1", len(sh.Labels))
	}
	hello := findLabel(sh, "HELLO")
	if hello == nil || hello.Justify != "left bottom" || hello.RotationDeg != 90 ||
		hello.Origin.GetX() != 5 || hello.Origin.GetY() != 6 {
		t.Errorf("HELLO label = %v", hello)
	}
	// Reference (synthesized) plus the placed Value property (WS1-037); the data-only Tol
	// property carries no display origin, so it is NOT a drawn field.
	if len(pl.Fields) != 2 {
		t.Fatalf("placement fields = %d, want 2 (Reference + placed Value)", len(pl.Fields))
	}
	if f := pl.Fields[0]; f.Name != "Reference" || f.Value != "R1" || !f.Visible {
		t.Errorf("field[0] = %v, want visible Reference=R1", f)
	}
	if f := pl.Fields[1]; f.Name != "Value" || f.Value != "10k" || !f.Visible ||
		f.Justify != "left" || f.Origin.GetX() != 320 || f.Origin.GetY() != 420 {
		t.Errorf("field[1] = %v, want visible Value=10k placed at (320,420) left", f)
	}
	// A free figure in commentGraphics becomes a sheet shape (a rectangle here).
	if len(sh.Shapes) != 1 || sh.Shapes[0].Kind != geom.Shape_KIND_RECT {
		t.Errorf("sheet shapes = %v, want 1 RECT from commentGraphics figure", sh.Shapes)
	}
}

// TestReadSchematic_WireNetNameForm: a net declared with the compound (net (name SIG) ...) form
// names its wire "SIG". The reader previously kept only the display name of that form (empty here)
// and fell back to atom(), which is empty for a compound, so every such wire was left UNNAMED —
// making net-subject findings unlocatable on the .eds canvas (WS1-047: the companion is joined by
// net name). The fix falls back to the id, matching how the netlist read names the same net.
func TestReadSchematic_WireNetNameForm(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "net-name-id.eds")), "net-name-id.eds")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(g.Sheets) != 1 || len(g.Sheets[0].Wires) != 1 {
		t.Fatalf("want one sheet with one wire, got %+v", g.Sheets)
	}
	if w := g.Sheets[0].Wires[0]; w.GetNet() != "SIG" {
		t.Errorf("wire net = %q, want SIG (the (name SIG) id)", w.GetNet())
	}
}

// TestReadSchematic_FieldVisibility asserts placedFields honors the source visibility flag and
// inherits a figureGroup's default text height (WS1-037 follow-up). An OrCAD/Mentor export records
// a display origin for nearly every property but marks most (visible (false)); only the visible
// ones become drawn Fields. A visible field that restates no textHeight inherits its figureGroup's
// default height (so it scales with the sheet), while one with its own textHeight keeps it.
func TestReadSchematic_FieldVisibility(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "visibility.eds")), "visibility.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Sheets) != 1 || len(g.Sheets[0].Placements) != 1 {
		t.Fatalf("sheets/placements = %d/%v, want 1/1", len(g.Sheets), g.Sheets)
	}
	fields := map[string]*geom.Field{}
	for _, f := range g.Sheets[0].Placements[0].Fields {
		fields[f.Name] = f
	}
	// The hidden Part_Number property must NOT be drawn.
	if f, ok := fields["Part_Number"]; ok {
		t.Errorf("hidden property Part_Number drawn as field %v, want dropped", f)
	}
	// Reference (synthesized) + the two visible properties: Value and Rating.
	if len(fields) != 3 {
		t.Fatalf("fields = %v, want 3 (Reference + Value + Rating)", fields)
	}
	// Heights are GLYPH heights, textLineRatio below the source textHeight the fixture states
	// (see glyphHeight): the EDIF value is a line pitch, so it is divided down to an em size.
	// Value overrides ATTRIBUTE without a textHeight, so it inherits the group default 254000.
	if v := fields["Value"]; v == nil || v.Value != "10k" || v.Height != glyphHeight(254000) {
		t.Errorf("Value field = %v, want 10k height %d (inherited figureGroup default 254000)", v, glyphHeight(254000))
	}
	// Rating restates its own textHeight, which wins over the group default.
	if r := fields["Rating"]; r == nil || r.Value != "50V" || r.Height != glyphHeight(100000) {
		t.Errorf("Rating field = %v, want 50V height %d (explicit 100000)", r, glyphHeight(100000))
	}
}

// TestReadSchematic_HiddenFieldFlood is the scaled, redistributable repro (hidden-field-flood.eds,
// no org-specific data) of the WS1-037 field-visibility flood: six parts each carry one visible
// Value and several hidden attribute properties (manufacturer, part number, datasheet URL,
// description, tolerance, footprint). Only the ref-des and the visible Value may be drawn — the
// hidden attributes must not flood the sheet.
func TestReadSchematic_HiddenFieldFlood(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "hidden-field-flood.eds")), "hidden-field-flood.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	var comp *geom.SheetGeometry
	for _, sh := range g.Sheets {
		if sh.Name == "COMPONENTS" {
			comp = sh
		}
	}
	if comp == nil {
		t.Fatalf("COMPONENTS sheet not found in %v", g.Sheets)
	}
	if len(comp.Placements) != 6 {
		t.Fatalf("placements = %d, want 6", len(comp.Placements))
	}
	hidden := map[string]bool{"Tolerance": true, "Manufacturer": true, "Part_Number": true, "Datasheet": true, "Description": true, "Footprint": true}
	for _, pl := range comp.Placements {
		// Exactly the synthesized Reference plus the one visible Value; no hidden attribute leaks.
		if len(pl.Fields) != 2 {
			t.Errorf("%s: fields = %d (%v), want 2 (Reference + Value)", pl.RefDes, len(pl.Fields), fieldNames(pl.Fields))
		}
		for _, f := range pl.Fields {
			if hidden[f.Name] {
				t.Errorf("%s: hidden property %q drawn as a field, want dropped", pl.RefDes, f.Name)
			}
		}
	}
}

// fieldNames is a test helper: the name of each field, for a readable failure message.
func fieldNames(fs []*geom.Field) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}

func findLabel(sh *geom.SheetGeometry, text string) *geom.Label {
	for _, l := range sh.Labels {
		if l.Text == text {
			return l
		}
	}
	return nil
}

// TestReadSchematic_Resolution asserts every placement joins to a SymbolDef: the &id-form
// cell reference is normalized to the display name, and the GRAPHIC-view builtin cell is
// extracted from its (contents ...) figures.
func TestReadSchematic_Resolution(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "resolve.eds")), "resolve.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}

	byKey := map[string]*geom.SymbolDef{}
	for _, s := range g.Symbols {
		byKey[s.CellRef+"|"+s.LibraryRef] = s
	}
	if len(g.Symbols) != 2 {
		t.Fatalf("symbols = %d, want 2 (PartA + gnd)", len(g.Symbols))
	}
	// The GRAPHIC builtin cell must be extracted from (contents ...): two LINE polylines.
	gnd := byKey["gnd|builtin"]
	if gnd == nil {
		t.Fatalf("builtin GRAPHIC symbol gnd|builtin not extracted")
	}
	if len(gnd.Shapes) != 2 || gnd.Bbox == nil {
		t.Errorf("gnd symbol shapes=%d bbox=%v, want 2 shapes + bbox", len(gnd.Shapes), gnd.Bbox)
	}

	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	// Every placement must resolve to a symbol in the library.
	for _, pl := range g.Sheets[0].Placements {
		if byKey[pl.CellRef+"|"+pl.LibraryRef] == nil {
			t.Errorf("placement %s: cell_ref=%q lib=%q does not resolve to a symbol",
				pl.RefDes, pl.CellRef, pl.LibraryRef)
		}
	}
	// The &id reference must be normalized to the display name (not "&c1").
	var r1 *geom.SymbolPlacement
	for _, pl := range g.Sheets[0].Placements {
		if pl.RefDes == "R1" {
			r1 = pl
		}
	}
	if r1 == nil {
		t.Fatalf("placement R1 not found")
	}
	if r1.CellRef != "PartA" {
		t.Errorf("R1 cell_ref = %q, want PartA (normalized from &c1)", r1.CellRef)
	}
	// The library reference must likewise normalize from its id (LibOne) to display name.
	if r1.LibraryRef != "Lib One" {
		t.Errorf("R1 library_ref = %q, want 'Lib One' (normalized from LibOne)", r1.LibraryRef)
	}
}

// TestReadSchematic_MultiView asserts a multi-section cell yields one SymbolDef per view
// and that each placement's view_ref selects the bank it references.
func TestReadSchematic_MultiView(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "multiview.eds")), "mv.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	byView := map[string]*geom.SymbolDef{}
	for _, s := range g.Symbols {
		if s.CellRef == "Conn" && s.LibraryRef == "Conns" {
			byView[s.ViewRef] = s
		}
	}
	if len(byView) != 2 {
		t.Fatalf("Conn symbols = %d, want 2 (Bank A + Bank B)", len(byView))
	}
	if a := byView["Bank A"]; a == nil || len(a.Shapes) != 1 {
		t.Errorf("Bank A symbol = %v, want 1 shape", a)
	}
	if b := byView["Bank B"]; b == nil || len(b.Shapes) != 2 {
		t.Errorf("Bank B symbol = %v, want 2 shapes", b)
	}
	// Each placement's view_ref (normalized from the view id) selects its bank.
	pv := map[string]string{}
	for _, pl := range g.Sheets[0].Placements {
		pv[pl.RefDes] = pl.ViewRef
	}
	if pv["PA"] != "Bank A" {
		t.Errorf("PA view_ref = %q, want 'Bank A'", pv["PA"])
	}
	if pv["PB"] != "Bank B" {
		t.Errorf("PB view_ref = %q, want 'Bank B'", pv["PB"])
	}
}

// TestReadSchematic_PinLabels asserts the visible pin's number label is captured (origin
// + justify) and the hidden pin's label is not.
func TestReadSchematic_PinLabels(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "pinlabel.eds")), "pl.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Symbols) != 1 {
		t.Fatalf("symbols = %d, want 1", len(g.Symbols))
	}
	pins := map[string]*geom.PinPoint{}
	for _, p := range g.Symbols[0].Pins {
		pins[p.PortRef] = p
	}
	a1 := pins["1"] // PortRef is the DESIGNATOR (the number), not the port name
	if a1 == nil || a1.LabelOrigin == nil {
		t.Fatalf("A1 pin label not captured: %v", pins)
	}
	// EDIF splits the two: the portImplementation names the PORT, the cell interface carries the
	// physical designator. PortRef is a join key (proto: ir.Port.designator), so it takes the
	// number and the name goes to Name, matching what the KiCad reader already produces.
	if a1.PortRef != "1" || a1.Name != "A1" {
		t.Errorf("A1 = PortRef %q / Name %q, want \"1\" / \"A1\"", a1.PortRef, a1.Name)
	}
	// The number has its own display, so it does not stack under the name.
	if a1.NumberOrigin.GetX() != 2 || a1.NumberOrigin.GetY() != 46 || a1.NumberJustify != "left top" {
		t.Errorf("A1 number placement = %v / %q, want (2,46) left top", a1.NumberOrigin, a1.NumberJustify)
	}
	// A3 declares a designator EQUAL to its port name, so Name stays empty rather than drawing
	// the same string twice.
	if a3 := pins["A3"]; a3 == nil || a3.Name != "" {
		t.Errorf("A3 = %v, want Name empty (designator equals the port name)", a3)
	}
	if a1.LabelOrigin.GetX() != 10 || a1.LabelOrigin.GetY() != 20 || a1.Justify != "right" {
		t.Errorf("A1 label = origin(%d,%d) justify %q, want (10,20) right",
			a1.LabelOrigin.GetX(), a1.LabelOrigin.GetY(), a1.Justify)
	}
	if a2 := pins["A2"]; a2 == nil || a2.LabelOrigin != nil {
		t.Errorf("A2 label should be hidden (nil origin), got %v", a2)
	}
	// A pin label carries a text height like any other run, so it scales with the sheet instead
	// of being sized from a renderer constant. A1 restates none and inherits the LABEL group's;
	// A3 states its own, which wins. Both are converted from a line pitch (see glyphHeight).
	if a1.Height != glyphHeight(12) {
		t.Errorf("A1 height = %d, want %d (inherited from figureGroup LABEL's textHeight 12)",
			a1.Height, glyphHeight(12))
	}
	if a3 := pins["A3"]; a3 == nil || a3.Height != glyphHeight(6) {
		t.Errorf("A3 = %v, want height %d (its own textHeight 6, overriding the group)",
			a3, glyphHeight(6))
	}
	// A hidden label has no height to carry, so the renderer's own default still applies.
	if a2 := pins["A2"]; a2 != nil && a2.Height != 0 {
		t.Errorf("A2 is hidden but carries height %d, want 0", a2.Height)
	}
}

// TestReadSchematic_UpsideDown documents the input to the upside-down-text bug (a
// real-corpus headers sheet, reproduced by the small upsidedown.eds fixture): the reader
// faithfully captures a source R180 on both a placement transform and a label's own text
// orientation. That the reader carries R180 through is correct; keeping such text readable is
// the render layer's job (see render.TestSheetSVG_UprightText).
func TestReadSchematic_UpsideDown(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "upsidedown.eds")), "upsidedown.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	byRef := map[string]*geom.SymbolPlacement{}
	for _, p := range g.Sheets[0].Placements {
		byRef[p.RefDes] = p
	}
	if x4 := byRef["X4"]; x4 == nil || x4.Transform == nil || x4.Transform.RotationDeg != 180 {
		t.Errorf("X4 placement rotation = %v, want R180", x4.GetTransform())
	}
	if x8 := byRef["X8"]; x8 == nil || x8.Transform == nil || x8.Transform.RotationDeg != 0 {
		t.Errorf("X8 placement rotation = %v, want R0", x8.GetTransform())
	}
	byText := map[string]*geom.Label{}
	for _, l := range g.Sheets[0].Labels {
		byText[l.Text] = l
	}
	if nf := byText["NET_FLIP"]; nf == nil || nf.RotationDeg != 180 {
		t.Errorf("NET_FLIP label rotation = %v, want R180", nf)
	}
}

// TestReadSchematic_OffPageLabels asserts off-page connector signal names (page-level
// portImplementation name displays) are captured as sheet labels at their placed origins.
func TestReadSchematic_OffPageLabels(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "offpage.eds")), "offpage.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	byText := map[string]*geom.Label{}
	for _, l := range g.Sheets[0].Labels {
		byText[l.Text] = l
	}
	if len(byText) != 2 {
		t.Fatalf("labels = %d, want 2 (NET_A + NET_B)", len(byText))
	}
	if a := byText["NET_A"]; a == nil || a.Origin.GetX() != 200 || a.Origin.GetY() != 800 || a.Justify != "left" {
		t.Errorf("NET_A label = %v, want origin(200,800) justify left", a)
	}
	if byText["NET_B"] == nil {
		t.Errorf("NET_B label missing")
	}
}

// TestReadSchematic_Annotations asserts static free-text annotations on a cell view (e.g.
// a title block's field labels) are captured as symbol-local Labels.
func TestReadSchematic_Annotations(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "annot.eds")), "annot.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Symbols) != 1 {
		t.Fatalf("symbols = %d, want 1", len(g.Symbols))
	}
	byText := map[string]*geom.Label{}
	for _, a := range g.Symbols[0].Annotations {
		byText[a.Text] = a
	}
	if len(byText) != 2 {
		t.Fatalf("annotations = %d, want 2 (SHEET + REV)", len(byText))
	}
	if s := byText["SHEET"]; s == nil || s.Origin.GetX() != 20 || s.Origin.GetY() != 150 || s.Justify != "left" {
		t.Errorf("SHEET annotation = %v, want origin(20,150) justify left", s)
	}
	if byText["REV"] == nil {
		t.Errorf("REV annotation missing")
	}
}

// TestReadSchematic_TitleBlock asserts the drawing-sheet border/title-block instance is
// promoted into the sheet's TitleBlock (WS7-019): the field-name'd properties map to
// title/rev/date/company, the first non-empty of a REV_n/DATE_n sequence wins, an all-dashes
// placeholder is treated as empty, and the border instance itself is dropped from placements
// (the worksheet frame is synthesized) instead of double-drawing the raw border symbol.
func TestReadSchematic_TitleBlock(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "titleblock.eds")), "titleblock.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	sh := g.Sheets[0]

	tb := sh.TitleBlock
	if tb == nil {
		t.Fatal("sheet TitleBlock is nil, want it populated from the border instance")
	}
	if tb.Title != "My Board" {
		t.Errorf("title = %q, want %q", tb.Title, "My Board")
	}
	if tb.Rev != "C3" { // REV_1 is empty, REV_2 = "C3" wins
		t.Errorf("rev = %q, want %q (first non-empty of REV_n)", tb.Rev, "C3")
	}
	if tb.Date != "2026-07-09" { // DATE is the "---" placeholder, DATE_1 wins
		t.Errorf("date = %q, want %q (placeholder skipped)", tb.Date, "2026-07-09")
	}
	if tb.Company != "Acme" {
		t.Errorf("company = %q, want %q", tb.Company, "Acme")
	}

	// Fields with no typed slot land in ExtraFields in source order, keyed by base name so
	// DV_1 (empty, skipped) / DV_2 collapse to DV = first non-empty.
	wantExtra := []struct{ key, value string }{
		{"DRAWING", "DEMO-100-SCH"},
		{"DESIGNER", "A ENGINEER"},
		{"PROTOTYPE", "PROTO1"},
		{"DV", "J DOE"},
	}
	if len(tb.ExtraFields) != len(wantExtra) {
		t.Fatalf("extra_fields = %d (%v), want %d", len(tb.ExtraFields), tb.ExtraFields, len(wantExtra))
	}
	for i, w := range wantExtra {
		if got := tb.ExtraFields[i]; got.Key != w.key || got.Value != w.value {
			t.Errorf("extra_fields[%d] = {%q,%q}, want {%q,%q}", i, got.Key, got.Value, w.key, w.value)
		}
	}

	// The border instance is dropped: only the real part (R1) remains a placement.
	if len(sh.Placements) != 1 || sh.Placements[0].RefDes != "R1" {
		t.Errorf("placements = %d (%v), want 1 (R1) with the border instance dropped",
			len(sh.Placements), refDesList(sh.Placements))
	}
	// The promoted field text must not also leak into loose sheet labels.
	for _, l := range sh.Labels {
		if l.Text == "My Board" {
			t.Errorf("title text %q also drawn as a loose label (double-draw)", l.Text)
		}
	}
}

// TestReadSchematic_RefDesDisplay asserts the ref-des is placed from its designator's own
// (display ...) rather than anchored on the symbol origin. Anchoring on the origin put the ref-des
// a line or two off its field column, at the renderer's fixed fallback size and center-anchored,
// so it overran the neighbouring component's fields. The display also carries the visibility flag,
// so an instance that hides its designator must draw nothing rather than falling back to the
// origin. sample.eds covers the fallback for a designator with no display at all.
func TestReadSchematic_RefDesDisplay(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "refdes-display.eds")), "refdes-display.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	byRef := map[string]*geom.Field{}
	for _, pl := range g.Sheets[0].Placements {
		for _, f := range pl.Fields {
			if f.Name == "Reference" {
				byRef[f.Value] = f
			}
		}
	}
	// R1 takes origin/justify from its display and inherits the ATTRIBUTE group's textHeight.
	f := byRef["R1"]
	if f == nil {
		t.Fatalf("R1 has no Reference field; got %v", byRef)
	}
	if f.Origin.GetX() != 100 || f.Origin.GetY() != 176 {
		t.Errorf("R1 origin = %v, want (100,176) from the designator display, not the symbol origin", f.Origin)
	}
	if f.Justify != "left bottom" || f.RotationDeg != 0 || f.Height != glyphHeight(10) {
		t.Errorf("R1 = justify %q, rot %d, height %d; want left bottom / 0 / %d (glyph height of the inherited textHeight 10)",
			f.Justify, f.RotationDeg, f.Height, glyphHeight(10))
	}
	// R2 restates its own textHeight, which wins over the group default, and rotates.
	if f := byRef["R2"]; f == nil || f.Height != glyphHeight(6) || f.RotationDeg != 90 || f.Justify != "right top" {
		t.Errorf("R2 = %v, want height %d (glyph height of the restated textHeight 6), rot 90, justify right top", f, glyphHeight(6))
	}
	// R3's display is marked hidden, so it must not be drawn at all.
	if f, ok := byRef["R3"]; ok {
		t.Errorf("R3 designator is (visible (false)) but was drawn as %v", f)
	}
}

// TestGlyphHeight pins the textHeight -> glyph height conversion with LITERAL expectations, so a
// change to textLineRatio fails here rather than silently rescaling every EDIF render. The inputs
// are real line pitches an EDIF technology declares (100 mil and 70 mil at this file's 10nm unit).
//
// The second assertion is the property that motivates the conversion at all: EDIF's textHeight is
// the pitch consecutive field rows are stacked at, so a glyph height EQUAL to it leaves zero
// leading and rows touch. The converted height must leave real space between lines.
func TestGlyphHeight(t *testing.T) {
	for _, tc := range []struct {
		pitch, want int64
		why         string
	}{
		{254000, 193185, "100 mil line pitch"},
		{177800, 135230, "70 mil line pitch"},
		{0, 0, "unspecified stays unspecified, so the renderer's fallback still applies"},
		{-5, -5, "a negative height is passed through rather than sign-flipped"},
	} {
		if got := glyphHeight(tc.pitch); got != tc.want {
			t.Errorf("glyphHeight(%d) = %d, want %d (%s)", tc.pitch, got, tc.want, tc.why)
		}
	}
	// Leading: a row of text must not fill its own line pitch.
	const pitch = 177800
	if leading := pitch - glyphHeight(pitch); leading < pitch/8 {
		t.Errorf("glyph height %d in a %d pitch leaves only %d of leading; rows would nearly touch",
			glyphHeight(pitch), pitch, leading)
	}
}

// TestReadSchematic_CellNameField asserts the part name an instance references becomes a drawn
// field when the source places and shows it. It rides on the cellRef rather than on a property:
// (viewRef V (cellRef (name PART (display ...)))). Most instances mark it hidden, so honoring the
// visibility flag is what keeps this to the few a schematic actually captions.
func TestReadSchematic_CellNameField(t *testing.T) {
	g, err := ReadSchematic(bytes.NewReader(readFixture(t, "pinlabel.eds")), "pl.eds")
	if err != nil {
		t.Fatalf("ReadSchematic: %v", err)
	}
	var cell *geom.Field
	for _, f := range g.Sheets[0].Placements[0].Fields {
		if f.Name == "Cell" {
			cell = f
		}
	}
	if cell == nil {
		t.Fatalf("no Cell field; got %v", g.Sheets[0].Placements[0].Fields)
	}
	if cell.Value != "Part" || cell.Origin.GetX() != 70 || cell.Origin.GetY() != 90 || cell.Justify != "left bottom" {
		t.Errorf("Cell field = %v, want Part at (70,90) left bottom", cell)
	}
}

// refDesList is a test helper: the ref-des of each placement, for a readable failure message.
func refDesList(pls []*geom.SymbolPlacement) []string {
	out := make([]string, len(pls))
	for i, pl := range pls {
		out[i] = pl.RefDes
	}
	return out
}
