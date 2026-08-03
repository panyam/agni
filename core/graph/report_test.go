package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func reportByRef(rep *ConversionReport) map[string]ComponentReport {
	m := map[string]ComponentReport{}
	for _, c := range rep.Components {
		m[c.RefDes] = c
	}
	return m
}

// TestBuildReportGlyphMode asserts the report attributes each component in glyph mode: a
// classified glyph carries its class and Kind glyph, an unclassified part is Kind box, and the
// box call-out lists it.
func TestBuildReportGlyphMode(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "U1"}, {RefDes: "W9"}}}
	rep := BuildReport(d, DefaultRegistry())
	by := reportByRef(rep)

	if by["R1"].Kind != KindGlyph || by["R1"].Class != ClassResistor {
		t.Errorf("R1 = %+v, want glyph/resistor", by["R1"])
	}
	if by["U1"].Kind != KindGlyph || by["U1"].Class != ClassIC {
		t.Errorf("U1 = %+v, want glyph/ic", by["U1"])
	}
	if by["W9"].Kind != KindBox {
		t.Errorf("W9 kind = %q, want box", by["W9"].Kind)
	}
	if box := rep.RefsByKind(KindBox); len(box) != 1 || box[0] != "W9" {
		t.Errorf("box refs = %v, want [W9]", box)
	}
}

// TestBuildReportFaithful asserts the faithful report distinguishes a resolved symbol (provided)
// from an unresolved one — both a placeholder box (the .sym did not load) and a ref the sidecar
// does not cover at all fall to unresolved, which is the "pass --symbol-path" signal.
func TestBuildReportFaithful(t *testing.T) {
	real := &geom.SymbolDef{CellRef: "lib:R"} // no placeholder asset -> a resolved symbol
	ph := &geom.SymbolDef{CellRef: "lib:U", Asset: &geom.Asset{Kind: geom.Asset_KIND_SYMBOL, Placeholder: true}}
	faithful := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{real, ph},
		Sheets: []*geom.SheetGeometry{{Placements: []*geom.SymbolPlacement{
			{RefDes: "R1", CellRef: "lib:R"}, {RefDes: "U1", CellRef: "lib:U"},
		}}},
	}
	d := &ir.Design{Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "U1"}, {RefDes: "C1"}}}
	rep := BuildReport(d, NewFaithfulSource(faithful, DefaultRegistry()))
	by := reportByRef(rep)

	if by["R1"].Kind != KindProvided {
		t.Errorf("R1 kind = %q, want provided", by["R1"].Kind)
	}
	if by["U1"].Kind != KindUnresolved {
		t.Errorf("U1 (placeholder) kind = %q, want unresolved", by["U1"].Kind)
	}
	if by["C1"].Kind != KindUnresolved {
		t.Errorf("C1 (not in sidecar) kind = %q, want unresolved", by["C1"].Kind)
	}
	if un := rep.RefsByKind(KindUnresolved); len(un) != 2 {
		t.Errorf("unresolved refs = %v, want 2 (U1, C1)", un)
	}
}
