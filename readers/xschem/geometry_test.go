package xschem

import (
	"bytes"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func TestReadSchematicGeometry(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	sheet := g.Sheets[0]

	// The res symbol carries its drawing (two lead polylines) and two pins, marked as a loaded
	// symbol asset. The pin boxes are captured as pins, not rects.
	res := symbolDef(g, "res")
	if res == nil {
		t.Fatal("no res SymbolDef")
	}
	if got := shapeKinds(res.Shapes); got[geom.Shape_KIND_POLYLINE] != 2 {
		t.Errorf("res polylines = %d, want 2", got[geom.Shape_KIND_POLYLINE])
	}
	if len(res.Pins) != 2 {
		t.Errorf("res pins = %d, want 2", len(res.Pins))
	}
	if res.Asset == nil || res.Asset.Kind != geom.Asset_KIND_SYMBOL {
		t.Errorf("res asset = %v, want KIND_SYMBOL", res.Asset)
	}

	// R1 placed at "100 0 0 0": no rotation, no mirror, origin at scaled (100,0) with y negated.
	r1 := findPlacement(sheet, "R1")
	if r1 == nil {
		t.Fatal("no R1 placement")
	}
	if r1.Transform.RotationDeg != 0 || r1.Transform.MirrorY {
		t.Errorf("R1 transform rot=%d mirror=%v, want 0/false", r1.Transform.RotationDeg, r1.Transform.MirrorY)
	}
	if r1.Transform.Origin.X != gx(100) || r1.Transform.Origin.Y != gy(0) {
		t.Errorf("R1 origin = (%d,%d), want (%d,%d)", r1.Transform.Origin.X, r1.Transform.Origin.Y, gx(100), gy(0))
	}

	// The three labelled nets are drawn as wires.
	if len(sheet.Wires) == 0 {
		t.Error("no wires")
	}
}

// Without an opener, an unresolved symbol falls back to a placeholder box so the sheet still
// renders, rather than an empty placement.
func TestGeometryNoOpenerPlaceholder(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", nil)
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	res := symbolDef(g, "res")
	if res == nil || len(res.Shapes) == 0 {
		t.Fatal("expected a placeholder shape for the unresolved symbol")
	}
}

// Reference and Value are placed at the symbol's @name/@value template positions, transformed by
// the placement — not stamped on top of each other at the origin.
func TestFieldPlacement(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	r1 := findPlacement(g.Sheets[0], "R1")
	ref := fieldNamed(r1, "Reference")
	val := fieldNamed(r1, "Value")
	if ref == nil || val == nil {
		t.Fatalf("R1 missing Reference/Value fields: %+v", r1.Fields)
	}
	if ref.Value != "R1" || val.Value != "1k" {
		t.Errorf("field values = %q/%q, want R1/1k", ref.Value, val.Value)
	}
	// @name at (8,-4), @value at (8,4); R1 at (100,0) rot0 flip0 -> gpt(108,-4)/gpt(108,4).
	if ref.Origin.X != gx(108) || ref.Origin.Y != gy(-4) {
		t.Errorf("Reference origin = (%d,%d), want (%d,%d)", ref.Origin.X, ref.Origin.Y, gx(108), gy(-4))
	}
	if ref.Origin.Y == val.Origin.Y {
		t.Errorf("Reference and Value share a Y (%d) — still overlapping", ref.Origin.Y)
	}
	// And not stamped at the placement origin.
	if ref.Origin.X == r1.Transform.Origin.X && ref.Origin.Y == r1.Transform.Origin.Y {
		t.Error("Reference sits at the symbol origin, not the template position")
	}
}

func fieldNamed(pl *geom.SymbolPlacement, name string) *geom.Field {
	for _, f := range pl.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func symbolDef(g *geom.SchematicGeometry, cell string) *geom.SymbolDef {
	for _, s := range g.Symbols {
		if s.CellRef == cell {
			return s
		}
	}
	return nil
}

func findPlacement(sheet *geom.SheetGeometry, ref string) *geom.SymbolPlacement {
	for _, p := range sheet.Placements {
		if p.RefDes == ref {
			return p
		}
	}
	return nil
}

func shapeKinds(shapes []*geom.Shape) map[geom.Shape_Kind]int {
	m := map[geom.Shape_Kind]int{}
	for _, s := range shapes {
		m[s.Kind]++
	}
	return m
}

// TestAnnotationSymbols (WS7-037): the geometry reader draws the rendered-annotation symbols —
// the title block (its @author, the schematic filename, and static text) and the visible SPICE
// code block (@name plus its multi-line @value split into stacked lines) — while a clutter
// annotation (spice_probe) stays skipped.
func TestAnnotationSymbols(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "annotations.sch")), "annotations.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	var refs, vals []string
	for _, pl := range g.Sheets[0].Placements {
		refs = append(refs, pl.CellRef)
		for _, f := range pl.Fields {
			vals = append(vals, f.Value)
		}
	}
	has := func(s []string, want string) bool {
		for _, v := range s {
			if v == want {
				return true
			}
		}
		return false
	}
	if has(refs, "spice_probe") {
		t.Error("spice_probe is clutter; should be skipped in geometry")
	}
	if !has(refs, "title") || !has(refs, "code_shown") {
		t.Fatalf("title/code_shown should be placed, got refs %v", refs)
	}
	for _, want := range []string{
		"Jane Doe",         // title @author from the instance
		"annotations.sch",  // title @schname_ext from the source filename
		"SCHEM",            // title static text
		"CMDS",             // code_shown @name
		".op",              // code_shown @value line 1
		".tran 1n 10u",     // code_shown @value line 2 (multi-line split)
	} {
		if !has(vals, want) {
			t.Errorf("annotation text %q not rendered; got %v", want, vals)
		}
	}
}

// TestReadBusGeometry asserts the geometry reader draws an xschem bus (an `N` wire whose lab is a
// range name) as a KIND_BUS wire named by that lab (WS7-042) — the join key a bus-not-modeled
// finding highlights it on — rather than an undistinguished plain wire.
func TestReadBusGeometry(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "bus.sch")), "bus.sch", nil)
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	if len(g.Sheets) != 1 {
		t.Fatalf("sheets = %d, want 1", len(g.Sheets))
	}
	var bus *geom.WireGeometry
	for _, w := range g.Sheets[0].Wires {
		if w.GetKind() == geom.WireGeometry_KIND_BUS {
			bus = w
		} else if w.GetKind() != geom.WireGeometry_KIND_UNSPECIFIED {
			t.Errorf("unexpected wire kind %v", w.GetKind())
		}
	}
	if bus == nil {
		t.Fatal("no KIND_BUS wire emitted for the bus-labeled N wire")
	}
	if bus.GetNet() != "DATA[7:0]" {
		t.Errorf("bus name = %q, want %q (its lab)", bus.GetNet(), "DATA[7:0]")
	}
	// N 100 -60 100 -30 -> gpt scales by 10 and negates Y: (1000,600)->(1000,300).
	if pts := bus.GetPolylines()[0].GetPoints(); len(pts) != 2 ||
		pts[0].X != 1000 || pts[0].Y != 600 || pts[1].X != 1000 || pts[1].Y != 300 {
		t.Errorf("bus points = %+v, want (1000,600)->(1000,300)", pts)
	}
}

// An xschem label symbol (gnd/vdd/ipin/opin/lab_pin) names the net at its origin rather than being a
// part, and read.go keeps it out of Components for exactly that reason. Its instance name therefore
// joins to nothing, so carrying it as a ref_des let a viewer select a component that does not exist;
// the net it names goes in net_anchor instead.
func TestLabelSymbolCarriesItsNetAnchor(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatal(err)
	}
	anchors, refs := map[string]bool{}, 0
	for _, sh := range g.Sheets {
		for _, pl := range sh.Placements {
			if a := pl.GetNetAnchor(); a != "" {
				anchors[a] = true
				if pl.GetRefDes() != "" {
					t.Errorf("anchor %q also carries ref_des %q", a, pl.GetRefDes())
				}
			}
			if pl.GetRefDes() != "" {
				refs++
			}
		}
	}
	if len(anchors) == 0 {
		t.Error("no label symbol carries a net anchor, so ground stays unclickable")
	}
	if refs == 0 {
		t.Error("no ordinary component kept its ref_des; the anchor rule is too broad")
	}
}
