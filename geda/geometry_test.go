package geda

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

	// The resistor symbol carries its drawing line and two pins as a loaded symbol asset.
	res := symbolDef(g, "resistor")
	if res == nil {
		t.Fatal("no resistor SymbolDef")
	}
	if len(res.Pins) != 2 {
		t.Errorf("resistor pins = %d, want 2", len(res.Pins))
	}
	if res.Asset == nil || res.Asset.Kind != geom.Asset_KIND_SYMBOL {
		t.Errorf("resistor asset = %v, want KIND_SYMBOL", res.Asset)
	}

	// R1 placed at "1000 2000 1 0 0": gEDA is Y-up, so no negation, no rotation/mirror.
	r1 := findPlacement(sheet, "R1")
	if r1 == nil {
		t.Fatal("no R1 placement")
	}
	if r1.Transform.RotationDeg != 0 || r1.Transform.MirrorY {
		t.Errorf("R1 transform rot=%d mirror=%v, want 0/false", r1.Transform.RotationDeg, r1.Transform.MirrorY)
	}
	if r1.Transform.Origin.X != 1000 || r1.Transform.Origin.Y != 2000 {
		t.Errorf("R1 origin = (%d,%d), want (1000,2000)", r1.Transform.Origin.X, r1.Transform.Origin.Y)
	}

	if len(sheet.Wires) == 0 {
		t.Error("no wires")
	}
}

// Reference and Value are placed at each attribute text's own coordinates from the instance
// block, with justify/visibility from the source — not stamped at the symbol origin.
func TestFieldPlacement(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "divider.sch")), "divider.sch", testOpener(t))
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	r2 := findPlacement(g.Sheets[0], "R2")
	ref := fieldNamed(r2, "Reference")
	val := fieldNamed(r2, "Value")
	if ref == nil || val == nil {
		t.Fatalf("R2 missing Reference/Value: %+v", r2.Fields)
	}
	// refdes T at (1100,1400), value T at (1100,1200) in the fixture's instance block.
	if ref.Origin.X != 1100 || ref.Origin.Y != 1400 {
		t.Errorf("Reference origin = (%d,%d), want (1100,1400)", ref.Origin.X, ref.Origin.Y)
	}
	if val.Origin.X != 1100 || val.Origin.Y != 1200 {
		t.Errorf("Value origin = (%d,%d), want (1100,1200)", val.Origin.X, val.Origin.Y)
	}
	if ref.Origin.Y == val.Origin.Y {
		t.Errorf("Reference and Value share a Y (%d) — still overlapping", ref.Origin.Y)
	}
	if ref.Value != "R2" || val.Value != "2k" {
		t.Errorf("field values = %q/%q, want R2/2k", ref.Value, val.Value)
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

// TestAnnotationSymbols (WS7-037): the geometry reader draws gEDA annotation symbols (title
// blocks, the A1/A2/A3 SPICE blocks) with their visible attribute text, instead of skipping them.
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
	placed := false
	for _, r := range refs {
		if r == "spice-model-1" {
			placed = true
		}
	}
	if !placed {
		t.Fatalf("spice-model-1 annotation should be drawn, got refs %v", refs)
	}
	for _, want := range []string{"2N3904", "./models/2N3904.mod"} {
		found := false
		for _, v := range vals {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("annotation attribute %q not rendered; got %v", want, vals)
		}
	}
}

// TestReadBusGeometry asserts the geometry reader draws a gEDA `U` bus object (WS7-042) as a
// KIND_BUS wire named by its inline netname — the join key a bus-not-modeled finding highlights it
// on — and does not leave it as an undistinguished plain-net wire.
func TestReadBusGeometry(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "bus.sch")), "bus.sch", testOpener(t))
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
		t.Fatal("no KIND_BUS wire emitted for the `U` bus object")
	}
	if bus.GetNet() != "DATA[7:0]" {
		t.Errorf("bus name = %q, want %q (its inline netname)", bus.GetNet(), "DATA[7:0]")
	}
	// U 1000 1000 2000 1000 — gEDA is Y-up, so points pass through unscaled.
	if pts := bus.GetPolylines()[0].GetPoints(); len(pts) != 2 ||
		pts[0].X != 1000 || pts[0].Y != 1000 || pts[1].X != 2000 || pts[1].Y != 1000 {
		t.Errorf("bus points = %+v, want (1000,1000)->(2000,1000)", pts)
	}
}
