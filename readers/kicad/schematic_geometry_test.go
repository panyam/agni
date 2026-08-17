package kicad

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestReadSchematicHierarchy asserts a root .kicad_sch with a (sheet ...) sub-sheet reference
// yields the root plus the referenced child as a two-node tree: hierarchical path ids, the
// child's parent set to the root, each sheet's own placements, and one deduped symbol library.
func TestReadSchematicHierarchy(t *testing.T) {
	open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", rel)) }
	g, err := ReadSchematicHierarchy("hier.kicad_sch", readFixture(t, "hier.kicad_sch"), open)
	if err != nil {
		t.Fatalf("ReadSchematicHierarchy: %v", err)
	}
	if len(g.Sheets) != 2 {
		t.Fatalf("sheets = %d, want 2", len(g.Sheets))
	}
	root, sub := g.Sheets[0], g.Sheets[1]
	if root.GetId() != "/" || root.GetName() != "Hier Root" || root.GetParentId() != "" {
		t.Errorf("root sheet = (id=%q name=%q parent=%q), want (/, Hier Root, \"\")", root.GetId(), root.GetName(), root.GetParentId())
	}
	if sub.GetId() != "/Sub A" || sub.GetName() != "Sub A" || sub.GetParentId() != "/" {
		t.Errorf("sub sheet = (id=%q name=%q parent=%q), want (/Sub A, Sub A, /)", sub.GetId(), sub.GetName(), sub.GetParentId())
	}
	if len(root.Placements) != 1 || len(sub.Placements) != 2 {
		t.Errorf("placements = (root %d, sub %d), want (1, 2)", len(root.Placements), len(sub.Placements))
	}
	// Both files declare test:R (one unit); merged and deduped to a single SymbolDef.
	if len(g.Symbols) != 1 {
		t.Errorf("symbols = %d, want 1 (merged+deduped test:R)", len(g.Symbols))
	}
}

// TestSubSheetBoxOnParent asserts each (sheet ...) instance draws its box on the PARENT page:
// a RECT at (at)/(size) plus a "Sheetname" label, independent of following the Sheetfile into
// the child (WS7-022). KiCad draws hier.kicad_sch/Hier Root with a "Sub A" rectangle.
func TestSubSheetBoxOnParent(t *testing.T) {
	open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", rel)) }
	g, err := ReadSchematicHierarchy("hier.kicad_sch", readFixture(t, "hier.kicad_sch"), open)
	if err != nil {
		t.Fatalf("ReadSchematicHierarchy: %v", err)
	}
	root := g.Sheets[0]

	// The box: a RECT whose corners span (at 100 100)/(size 40 30) -> geom (Y negated)
	// (100,-100)..(140,-130) in mm*1e6.
	var box *geom.Shape
	for _, s := range root.Shapes {
		if s.Kind == geom.Shape_KIND_RECT {
			box = s
			break
		}
	}
	if box == nil {
		t.Fatalf("parent sheet has no RECT sub-sheet box; shapes = %+v", root.Shapes)
	}
	x0, y0 := box.Points[0].X, box.Points[0].Y
	x1, y1 := box.Points[1].X, box.Points[1].Y
	if x0 != 100_000_000 || y0 != -100_000_000 || x1 != 140_000_000 || y1 != -130_000_000 {
		t.Errorf("box corners = (%d,%d)-(%d,%d), want (100000000,-100000000)-(140000000,-130000000)", x0, y0, x1, y1)
	}

	// The Sheetname label "Sub A" and the hierarchical pin "IN" on the parent page.
	want := map[string]bool{"Sub A": false, "IN": false}
	for _, l := range root.Labels {
		if _, ok := want[l.Text]; ok {
			want[l.Text] = true
		}
	}
	for text, got := range want {
		if !got {
			t.Errorf("parent sheet missing %q label; labels = %+v", text, root.Labels)
		}
	}
}

func readGeom(t *testing.T) *geom.SchematicGeometry {
	t.Helper()
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "geom.kicad_sch")), "geom.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	return g
}

func TestReadSchematicGeometryShape(t *testing.T) {
	g := readGeom(t)
	if g.UnitNm != 1 {
		t.Errorf("unit_nm = %d, want 1", g.UnitNm)
	}
	if g.DesignRef != "Geo Test" {
		t.Errorf("design_ref = %q, want %q", g.DesignRef, "Geo Test")
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	sh := g.Sheets[0]
	if len(sh.Placements) != 2 {
		t.Errorf("placements = %d, want 2", len(sh.Placements))
	}
	if len(sh.Wires) != 1 || len(sh.Wires[0].Polylines) != 1 || len(sh.Wires[0].Polylines[0].Points) != 2 {
		t.Errorf("want 1 wire with 1 polyline of 2 points, got %+v", sh.Wires)
	}
	// A4 paper, Y-up: width 297mm on +X, height 210mm on -Y.
	if sh.Size == nil || sh.Size.Max.X != 297_000_000 || sh.Size.Min.Y != -210_000_000 {
		t.Errorf("paper size = %+v, want A4 (297x210mm)", sh.Size)
	}
}

func TestReadSchematicGeometryMmToNmAndYFlip(t *testing.T) {
	g := readGeom(t)
	r1 := placement(g, "R1")
	if r1 == nil {
		t.Fatal("placement R1 not found")
	}
	// 50.8mm -> 50_800_000nm on X; 40.64mm -> negated on Y (KiCad Y-down -> geom Y-up).
	if r1.Transform.Origin.X != 50_800_000 || r1.Transform.Origin.Y != -40_640_000 {
		t.Errorf("R1 origin = (%d,%d), want (50800000,-40640000)", r1.Transform.Origin.X, r1.Transform.Origin.Y)
	}
}

func TestReadSchematicGeometryTransform(t *testing.T) {
	g := readGeom(t)
	u1 := placement(g, "U1")
	if u1 == nil {
		t.Fatal("placement U1 not found")
	}
	// KiCad angle 90 -> geom 270: the Y-down->Y-up conversion inverts rotation direction.
	if u1.Transform.RotationDeg != 270 {
		t.Errorf("U1 rotation = %d, want 270 (KiCad 90 negated for Y-up)", u1.Transform.RotationDeg)
	}
	if !u1.Transform.MirrorX {
		t.Error("U1 mirror_x should be set (mirror x)")
	}
	if u1.CellRef != "test:DUAL" || u1.ViewRef != "2" {
		t.Errorf("U1 cell_ref=%q view_ref=%q, want test:DUAL / 2", u1.CellRef, u1.ViewRef)
	}
}

func TestReadSchematicGeometryPerUnitSymbols(t *testing.T) {
	g := readGeom(t)
	// test:R is single-unit -> one SymbolDef exposed as view "1". test:DUAL is two units ->
	// two SymbolDefs (view "1" and "2"), each carrying only its unit's pin.
	if d := symbolDef(g, "test:R", "1"); d == nil {
		t.Error("test:R view 1 SymbolDef missing")
	}
	u1 := symbolDef(g, "test:DUAL", "1")
	u2 := symbolDef(g, "test:DUAL", "2")
	if u1 == nil || u2 == nil {
		t.Fatalf("test:DUAL should have view 1 and 2, got %v / %v", u1, u2)
	}
	if !hasPin(u1, "1") || hasPin(u1, "2") {
		t.Error("test:DUAL view 1 should carry pin 1 only")
	}
	if !hasPin(u2, "2") || hasPin(u2, "1") {
		t.Error("test:DUAL view 2 should carry pin 2 only")
	}
}

func TestReadSchematicGeometryLabels(t *testing.T) {
	g := readGeom(t)
	sh := g.Sheets[0]
	net := label(sh, "NET1")
	if net == nil {
		t.Fatal("label NET1 not found")
	}
	if net.Origin.X != 60_960_000 || net.Origin.Y != -40_640_000 {
		t.Errorf("NET1 origin = (%d,%d), want (60960000,-40640000)", net.Origin.X, net.Origin.Y)
	}
	if net.Height != 1_270_000 {
		t.Errorf("NET1 height = %d, want 1270000 (1.27mm)", net.Height)
	}
	if label(sh, "a note") == nil {
		t.Error("free-text note label missing")
	}
}

func TestReadSchematicGeometryFields(t *testing.T) {
	g := readGeom(t)
	r1 := placement(g, "R1")
	if r1 == nil {
		t.Fatal("placement R1 not found")
	}
	// Reference + Value are visible fields at their own positions; the hidden Footprint is
	// present but not visible.
	ref, val, fp := field(r1, "Reference"), field(r1, "Value"), field(r1, "Footprint")
	if ref == nil || !ref.Visible || ref.Value != "R1" {
		t.Errorf("R1 Reference field = %v, want visible R1", ref)
	}
	if val == nil || !val.Visible || val.Value != "10k" || val.Origin.X != 53_340_000 || val.Origin.Y != -39_370_000 {
		t.Errorf("R1 Value field = %v, want visible 10k at (53340000,-39370000)", val)
	}
	if fp == nil || fp.Visible {
		t.Errorf("R1 Footprint field should be present but hidden, got %v", fp)
	}
}

func field(pl *geom.SymbolPlacement, name string) *geom.Field {
	for _, f := range pl.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func TestReadSchematicGeometryPinNumbers(t *testing.T) {
	g := readGeom(t)
	// test:R hides pin numbers (pin_numbers hide yes) -> no label origin; test:DUAL shows them.
	r := symbolDef(g, "test:R", "1")
	for _, p := range r.Pins {
		if p.LabelOrigin != nil {
			t.Errorf("test:R pin %q should have no number label (hidden)", p.PortRef)
		}
	}
	d := symbolDef(g, "test:DUAL", "2")
	shown := false
	for _, p := range d.Pins {
		if p.LabelOrigin != nil {
			shown = true
		}
	}
	if !shown {
		t.Error("test:DUAL pin numbers should be shown")
	}
}

func TestReadSchematicGeometryPinLegs(t *testing.T) {
	g := readGeom(t)
	r := symbolDef(g, "test:R", "1")
	if r == nil {
		t.Fatal("test:R view 1 missing")
	}
	// Two pins -> two leg polylines. The top pin (at 0 3.81 270)(length 1.27) legs INTO the
	// body: end y = 3.81 - 1.27 = 2.54mm, the rectangle's top edge. A flipped direction would
	// put it at 5.08mm (sticking outward) — this is the direction check.
	legs := 0
	var topLeg *geom.Shape
	for _, s := range r.Shapes {
		if s.Kind != geom.Shape_KIND_POLYLINE || len(s.Points) != 2 {
			continue
		}
		legs++
		if s.Points[0].X == 0 && s.Points[0].Y == 3_810_000 {
			topLeg = s
		}
	}
	if legs != 2 {
		t.Errorf("test:R leg polylines = %d, want 2 (one per pin)", legs)
	}
	if topLeg == nil {
		t.Fatal("top-pin leg (starting at 0,3810000) not found")
	}
	if topLeg.Points[1].X != 0 || topLeg.Points[1].Y != 2_540_000 {
		t.Errorf("top leg end = (%d,%d), want (0,2540000) into the body (not outward)", topLeg.Points[1].X, topLeg.Points[1].Y)
	}
}

func TestReadSchematicGeometryFill(t *testing.T) {
	g := readGeom(t)
	// test:R's body rectangle is (fill (type background)); DUAL's polylines are unfilled.
	r := symbolDef(g, "test:R", "1")
	if r == nil || len(r.Shapes) == 0 || r.Shapes[0].Fill != geom.Shape_FILL_BACKGROUND {
		t.Errorf("test:R body should be background-filled, got %v", r.Shapes)
	}
	d := symbolDef(g, "test:DUAL", "1")
	for _, s := range d.Shapes {
		if s.Fill != geom.Shape_FILL_UNSPECIFIED {
			t.Errorf("test:DUAL shapes should be unfilled, got fill %v", s.Fill)
		}
	}
}

func TestReadSchematicGeometrySheetShapes(t *testing.T) {
	sh := readGeom(t).Sheets[0]
	// A junction (1 DOT), a no-connect (2 polyline arms of an X), and a sheet polyline.
	dots, polylines := 0, 0
	for _, s := range sh.Shapes {
		switch s.Kind {
		case geom.Shape_KIND_DOT:
			dots++
		case geom.Shape_KIND_POLYLINE:
			polylines++
		}
	}
	if dots != 1 {
		t.Errorf("junction dots = %d, want 1", dots)
	}
	if polylines != 3 { // 2 for the no-connect X + 1 sheet polyline
		t.Errorf("sheet polylines = %d, want 3 (no-connect X + 1 graphic)", polylines)
	}
}

func TestReadSchematicGeometryTitleBlock(t *testing.T) {
	tb := readGeom(t).Sheets[0].TitleBlock
	if tb == nil || tb.Title != "Geo Test" {
		t.Errorf("title block = %v, want title 'Geo Test'", tb)
	}
}

func TestReadSchematicGeometryRejectsNonSchematic(t *testing.T) {
	if _, err := ReadSchematicGeometry(bytes.NewReader([]byte("(kicad_pcb)")), "x.kicad_pcb"); err == nil {
		t.Error("want error for a non-.kicad_sch root")
	}
}

func placement(g *geom.SchematicGeometry, ref string) *geom.SymbolPlacement {
	for _, p := range g.Sheets[0].Placements {
		if p.RefDes == ref {
			return p
		}
	}
	return nil
}

func symbolDef(g *geom.SchematicGeometry, cell, view string) *geom.SymbolDef {
	for _, s := range g.Symbols {
		if s.CellRef == cell && s.ViewRef == view {
			return s
		}
	}
	return nil
}

func hasPin(d *geom.SymbolDef, portRef string) bool {
	for _, p := range d.Pins {
		if p.PortRef == portRef {
			return true
		}
	}
	return false
}

func label(sh *geom.SheetGeometry, text string) *geom.Label {
	for _, l := range sh.Labels {
		if l.Text == text {
			return l
		}
	}
	return nil
}

// TestWireNetNames (WS1-022): the geometry reader stamps each wire with its solved net
// name — a labeled wire gets the label, an unlabeled pinned wire gets its N$ stub — so
// the viewer can highlight/badge KiCad wires by net.
func TestWireNetNames(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "wirenet.kicad_sch")), "wirenet.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	nets := map[string]string{} // we can't key by uuid (dropped in geom), so collect the name set
	var names []string
	for _, sh := range g.Sheets {
		for _, w := range sh.Wires {
			names = append(names, w.Net)
			nets[w.Net] = w.Net
		}
	}
	if nets["SIG"] == "" {
		t.Errorf("the labeled wire must carry net SIG; got wire nets %v", names)
	}
	// The unlabeled stub wire off R1.2 gets a synthesized N$ name (not empty).
	hasStub := false
	for _, n := range names {
		if len(n) >= 2 && n[:2] == "N$" {
			hasStub = true
		}
	}
	if !hasStub {
		t.Errorf("the unlabeled stub wire must carry an N$ name; got %v", names)
	}
}

// TestWireNetNamesHierarchy (WS1-022): in a hierarchy the combined solve qualifies
// sub-sheet local names ("/amp1/SIG"), matching the netlist read exactly, so a
// sub-sheet's labeled wires carry the qualified name a net-subject finding would use.
func TestWireNetNamesHierarchy(t *testing.T) {
	open := func(rel string) ([]byte, error) { return readFixture(t, rel), nil }
	g, err := ReadSchematicHierarchy("hier_root.kicad_sch", readFixture(t, "hier_root.kicad_sch"), open)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, sh := range g.Sheets {
		for _, w := range sh.Wires {
			if w.Net != "" {
				got[w.Net] = true
			}
		}
	}
	// hier_child's SIG label is instantiated on amp1 and amp2, so its wires qualify.
	for _, want := range []string{"/amp1/SIG", "/amp2/SIG"} {
		if !got[want] {
			t.Errorf("missing qualified wire net %q; have %v", want, got)
		}
	}
}

// TestReadBusGeometry asserts the geometry reader draws KiCad bus constructs (WS7-042): a `bus`
// trunk and a `bus_entry` stub become WireGeometry tagged KIND_BUS / KIND_BUS_ENTRY, carrying the
// KiCad uuid on Prov.SourceId (the WS7-042b highlight join key) and no net, while a plain wire
// stays an untagged (KIND_UNSPECIFIED) wire. bus_alias is a declaration and is not drawn.
func TestReadBusGeometry(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "bus-render.kicad_sch")), "bus-render.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	byKind := map[geom.WireGeometry_Kind]*geom.WireGeometry{}
	for _, w := range g.Sheets[0].Wires {
		byKind[w.GetKind()] = w
	}
	// One plain wire, one bus trunk, one bus entry — three drawable wires, no more (the bus_alias
	// must not add geometry).
	if got := len(g.Sheets[0].Wires); got != 3 {
		t.Fatalf("drawn wires = %d, want 3 (wire + bus + bus_entry)", got)
	}
	if byKind[geom.WireGeometry_KIND_UNSPECIFIED] == nil {
		t.Error("plain wire missing / not left untagged")
	}

	bus := byKind[geom.WireGeometry_KIND_BUS]
	if bus == nil {
		t.Fatal("no KIND_BUS wire emitted")
	}
	// The bus is NAMED by the range label on its wire (WS7-042b), the key a bus finding joins on; it
	// still has no net_id (its member nets are unmodeled).
	if bus.GetNet() != "DATA[7:0]" {
		t.Errorf("bus name = %q, want %q (the range label on the bus)", bus.GetNet(), "DATA[7:0]")
	}
	if bus.GetNetId() != "" {
		t.Errorf("bus net_id = %q, want none (members unmodeled)", bus.GetNetId())
	}
	if bus.GetProv().GetSourceId() != "bus-1" {
		t.Errorf("bus Prov.SourceId = %q, want %q (the uuid, kept for provenance)", bus.GetProv().GetSourceId(), "bus-1")
	}
	// (xy 100 120) -> (xy 150 120), Y negated to the geom frame.
	if pts := bus.GetPolylines()[0].GetPoints(); len(pts) != 2 ||
		pts[0].X != 100_000_000 || pts[0].Y != -120_000_000 || pts[1].X != 150_000_000 || pts[1].Y != -120_000_000 {
		t.Errorf("bus points = %+v, want (100,-120)->(150,-120) in Mnm", pts)
	}

	entry := byKind[geom.WireGeometry_KIND_BUS_ENTRY]
	if entry == nil {
		t.Fatal("no KIND_BUS_ENTRY wire emitted")
	}
	if entry.GetProv().GetSourceId() != "be-1" {
		t.Errorf("bus_entry Prov.SourceId = %q, want %q", entry.GetProv().GetSourceId(), "be-1")
	}
	// (at 150 120) + (size 2.54 2.54): start (150,-120), end start + (dx, -dy) = (152.54, -122.54).
	if pts := entry.GetPolylines()[0].GetPoints(); len(pts) != 2 ||
		pts[0].X != 150_000_000 || pts[0].Y != -120_000_000 || pts[1].X != 152_540_000 || pts[1].Y != -122_540_000 {
		t.Errorf("bus_entry points = %+v, want (150,-120)->(152.54,-122.54) in Mnm", pts)
	}
}

// A #-prefixed symbol (#PWR, #FLG) is drawn but is not a component, and the geometry used to blank
// its reference and stop there — leaving the glyph anonymous, so the thing that NAMES a rail was the
// one thing on a sheet nothing could address. Its Value is the net name (the same fact sch_nets.go
// turns into a rank-0 anchor), so it carries that as net_anchor instead.
func TestPowerSymbolCarriesItsNetAnchor(t *testing.T) {
	open := func(rel string) ([]byte, error) { return readFixture(t, rel), nil }
	g, err := ReadSchematicHierarchy("hier_root.kicad_sch", readFixture(t, "hier_root.kicad_sch"), open)
	if err != nil {
		t.Fatal(err)
	}

	anchors := map[string]bool{}
	for _, sh := range g.Sheets {
		for _, pl := range sh.Placements {
			if a := pl.GetNetAnchor(); a != "" {
				anchors[a] = true
				// An anchor is not a component: a ref_des here would join to nothing.
				if pl.GetRefDes() != "" {
					t.Errorf("anchor %q also carries ref_des %q", a, pl.GetRefDes())
				}
			}
		}
	}
	if !anchors["VCC"] {
		t.Errorf("the VCC power symbol carries no net anchor; have %v", anchors)
	}
}
