package graph

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestDefaultClassify covers the built-in classifier (WS7-030): the source symbol/part-type
// name wins (so odd designators like RE1/Cout still classify), the ref-des prefix matches on a
// startswith (so RE1/RC1/CE1 are not lost to the box), multi-letter LED beats the single-letter
// L rule, designator_prefix overrides the ref-des guess, and unknown parts are ClassOther.
func TestDefaultClassify(t *testing.T) {
	reg := DefaultRegistry()
	parts := map[string]*ir.PartType{
		"res":     {Name: "res"},                           // xschem symbol name
		"capa":    {Name: "capa"},                          // xschem symbol name
		"npn":     {Name: "npn"},                           // xschem symbol name
		"esdprot": {Name: "esdprot"},                       // esd* symbol -> tvs
		"WIDGET":  {Name: "WIDGET", DesignatorPrefix: "C"}, // odd name, but declares prefix C
	}
	sec := func(part string) []*ir.ComponentSection { return []*ir.ComponentSection{{PartRef: part}} }

	cases := []struct {
		name string
		comp *ir.Component
		want string
	}{
		{"res symbol beats odd refdes", &ir.Component{RefDes: "RE1", Sections: sec("res")}, ClassResistor},
		{"capa symbol", &ir.Component{RefDes: "Cout", Sections: sec("capa")}, ClassCapacitor},
		{"npn symbol", &ir.Component{RefDes: "Q1", Sections: sec("npn")}, ClassTransistor},
		{"multi-letter R prefix (no part)", &ir.Component{RefDes: "RE1"}, ClassResistor},
		{"multi-letter C prefix (no part)", &ir.Component{RefDes: "CE2"}, ClassCapacitor},
		{"Cout by prefix", &ir.Component{RefDes: "Cout"}, ClassCapacitor},
		{"LED beats inductor", &ir.Component{RefDes: "LED3"}, ClassLED},
		{"plain inductor", &ir.Component{RefDes: "L1"}, ClassInductor},
		{"ic by prefix", &ir.Component{RefDes: "U5"}, ClassIC},
		{"IC multi-letter prefix", &ir.Component{RefDes: "IC2"}, ClassIC},
		{"ground by refdes", &ir.Component{RefDes: "#PWR01"}, ClassGround},
		{"designator_prefix overrides refdes", &ir.Component{RefDes: "X1", Sections: sec("WIDGET")}, ClassCapacitor},
		{"ferrite bead FB", &ir.Component{RefDes: "FB1"}, ClassFerrite},
		{"fuse", &ir.Component{RefDes: "F1"}, ClassFuse},
		{"tvs prefix", &ir.Component{RefDes: "TVS1"}, ClassTVS},
		{"esd part name", &ir.Component{RefDes: "D9", Sections: sec("esdprot")}, ClassTVS},
		{"CR is a diode not a capacitor", &ir.Component{RefDes: "CR2"}, ClassDiode},
		{"CN connector beats capacitor", &ir.Component{RefDes: "CN1"}, ClassConnector},
		{"J connector", &ir.Component{RefDes: "J1"}, ClassConnector},
		{"P connector", &ir.Component{RefDes: "P3"}, ClassConnector},
		{"test point", &ir.Component{RefDes: "TP7"}, ClassTestPoint},
		{"crystal Y", &ir.Component{RefDes: "Y1"}, ClassCrystal},
		{"unknown stays box", &ir.Component{RefDes: "W9"}, ClassOther},
	}
	for _, tc := range cases {
		if got := reg.Classify(tc.comp, parts); got != tc.want {
			t.Errorf("%s: Classify(%s) = %q, want %q", tc.name, tc.comp.RefDes, got, tc.want)
		}
	}
}

// TestUserRuleOverrides asserts a user rule prepended with With wins over the defaults, and that
// an open (user-defined) class id with its own glyph is honored.
func TestUserRuleOverrides(t *testing.T) {
	// Remap the "res" symbol to capacitor, and add a brand-new "crystal" class + glyph.
	crystal := &geom.SymbolDef{CellRef: "__node:crystal__"}
	reg := DefaultRegistry().With(
		ClassRule{Class: ClassCapacitor, Symbol: "res"},
		ClassRule{Class: "crystal", Prefix: "Y"},
	)
	reg.Glyphs["crystal"] = crystal

	parts := map[string]*ir.PartType{"res": {Name: "res"}}
	if got := reg.Classify(&ir.Component{RefDes: "R1", Sections: []*ir.ComponentSection{{PartRef: "res"}}}, parts); got != ClassCapacitor {
		t.Errorf("user rule should remap res -> capacitor, got %q", got)
	}
	if got := reg.Classify(&ir.Component{RefDes: "Y1"}, nil); got != "crystal" {
		t.Errorf("user class crystal not matched, got %q", got)
	}
	if reg.cellFor("crystal") != "__node:crystal__" {
		t.Errorf("custom glyph cell = %q, want __node:crystal__", reg.cellFor("crystal"))
	}
}

// TestAssembleDrawsClassGlyphs asserts assemble points each placement at its device-class glyph
// (not the shared box) and ships one glyph per used class with the right pin count; an IC gets
// the ic body glyph. It exercises the multi-letter designators the old classifier lost.
func TestAssembleDrawsClassGlyphs(t *testing.T) {
	d := &ir.Design{
		Name: "mix",
		Components: []*ir.Component{
			{RefDes: "R1"}, {RefDes: "RE1"}, {RefDes: "C1"}, {RefDes: "Cout"}, {RefDes: "Q1"}, {RefDes: "U1"},
		},
		Nets: []*ir.Net{{Name: "N", Connections: []*ir.Connection{
			{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "C1", PinRef: "1"},
		}}},
	}
	g := layout(d)

	cellByRef := map[string]string{}
	for _, pl := range g.Sheets[0].Placements {
		cellByRef[pl.RefDes] = pl.CellRef
	}
	reg := DefaultRegistry()
	wantCell := map[string]string{
		"R1":   reg.cellFor(ClassResistor),
		"RE1":  reg.cellFor(ClassResistor), // multi-letter, previously a box
		"C1":   reg.cellFor(ClassCapacitor),
		"Cout": reg.cellFor(ClassCapacitor), // previously a box
		"Q1":   reg.cellFor(ClassTransistor),
		"U1":   reg.cellFor(ClassIC),
	}
	for ref, want := range wantCell {
		if cellByRef[ref] != want {
			t.Errorf("%s cell = %q, want %q", ref, cellByRef[ref], want)
		}
	}

	symByCell := map[string]*geom.SymbolDef{}
	for _, s := range g.Symbols {
		symByCell[s.CellRef] = s
	}
	if got := len(symByCell); got != 4 {
		t.Errorf("shipped %d distinct glyphs, want 4 (resistor, capacitor, transistor, ic)", got)
	}
	if r := symByCell[reg.cellFor(ClassResistor)]; r == nil || len(r.Pins) != 2 {
		t.Errorf("resistor glyph pins = %v, want 2", pinCount(r))
	}
	if q := symByCell[reg.cellFor(ClassTransistor)]; q == nil || len(q.Pins) != 3 {
		t.Errorf("transistor glyph pins = %v, want 3", pinCount(q))
	}
}

func pinCount(s *geom.SymbolDef) int {
	if s == nil {
		return -1
	}
	return len(s.Pins)
}
