package graph

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/model"
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

// TestStampedClassChoosesTheGlyph is agni issue 701: the glyph comes from the class the ingestion
// pass stamped, so the picture and the facts cannot disagree about what a part is. Each case is a
// row of the issue's measured table, and the third field is what the built-in rules alone answered
// before the stamp was read — the wrong picture this closes.
func TestStampedClassChoosesTheGlyph(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		refDes    string
		stamped   []string
		wantClass string
		wantGlyph string
		wasDrawn  string // the rule table's answer, for the record
	}{
		{"D1", []string{"tvs", "diode"}, "tvs", ClassTVS, ClassDiode},
		{"TH1", []string{"thermistor", "resistor"}, "thermistor", ClassResistor, ClassOther},
		{"LX1", []string{"ferrite", "inductor"}, "ferrite", ClassFerrite, ClassOther},
		{"Y1", []string{"clock"}, "clock", ClassCrystal, ClassCrystal},
		{"RT1", []string{"thermistor", "resistor"}, "thermistor", ClassResistor, ClassResistor},
		{"D2", []string{"zener", "diode"}, "zener", ClassDiode, ClassDiode},
		{"J9", []string{"test_connector"}, "test_connector", ClassConnector, ClassConnector},
	}
	for _, tc := range cases {
		c := &ir.Component{RefDes: tc.refDes, DeviceClasses: classify.Tags(tc.stamped...)}
		if got := reg.Classify(c, nil); got != tc.wantClass {
			t.Errorf("%s: Classify = %q, want the stamped %q", tc.refDes, got, tc.wantClass)
		}
		if got := reg.Symbol(tc.refDes, c, nil).GetCellRef(); got != reg.cellFor(tc.wantGlyph) {
			t.Errorf("%s: glyph cell = %q, want the %s glyph %q (the rules alone drew %q)",
				tc.refDes, got, tc.wantGlyph, reg.cellFor(tc.wantGlyph), tc.wasDrawn)
		}
	}
}

// TestUserClassRuleBeatsTheStamp keeps the CLI's --class explicit: a user naming a glyph for a
// symbol is saying what to draw, so it wins over the stamped class as it always won over the rules.
func TestUserClassRuleBeatsTheStamp(t *testing.T) {
	reg := DefaultRegistry().With(ClassRule{Class: ClassCapacitor, Symbol: "res*"})
	parts := map[string]*ir.PartType{"res": {Name: "res"}}
	c := &ir.Component{
		RefDes:        "R1",
		Sections:      []*ir.ComponentSection{{PartRef: "res"}},
		DeviceClasses: classify.Tags("resistor"),
	}
	if got := reg.Classify(c, parts); got != ClassCapacitor {
		t.Errorf("user rule = %q, want it to beat the stamped resistor with %q", got, ClassCapacitor)
	}
	// The stamp still wins over the BUILT-IN rules, which is the other half of the ordering.
	plain := DefaultRegistry()
	d1 := &ir.Component{RefDes: "D1", DeviceClasses: classify.Tags("tvs", "diode")}
	if got := plain.Classify(d1, nil); got != ClassTVS {
		t.Errorf("built-in rule won over the stamp: got %q, want %q", got, ClassTVS)
	}
}

// TestUnstampedComponentUsesTheRules pins the fallback: a design read by something that never ran
// the classify pass (a hand-built ir.Design, a test fixture) still classifies by symbol and ref-des.
func TestUnstampedComponentUsesTheRules(t *testing.T) {
	reg := DefaultRegistry()
	if got := reg.Classify(&ir.Component{RefDes: "R1"}, nil); got != ClassResistor {
		t.Errorf("unstamped R1 = %q, want %q", got, ClassResistor)
	}
	// "unknown" is the absence of a class, not a class: it must not shadow the rules.
	c := &ir.Component{RefDes: "R1", DeviceClasses: classify.Tags("unknown")}
	if got := reg.Classify(c, nil); got != ClassResistor {
		t.Errorf("unknown-stamped R1 = %q, want the rules' %q", got, ClassResistor)
	}
}

// TestEveryStampedClassDraws is the ratchet the issue asks for. A class the engine ships but the
// registry cannot draw is a part the viewer shows as a box while every rule and query knows what it
// is, and nothing else would report it. The set is built with classify.ClassesOf so the test reads
// what ingestion actually stamps, family tag and all, rather than a hand-kept list.
func TestEveryStampedClassDraws(t *testing.T) {
	reg := DefaultRegistry()
	for _, cl := range model.ComponentClasses() {
		c := &ir.Component{RefDes: "X1", DeviceClasses: classify.TagsOf(cl, ir.ClassSource_CLASS_SOURCE_CONVENTION)}
		class, glyph := reg.choose(c, nil)
		if class != string(cl) {
			t.Errorf("%s: classified as %q", cl, class)
		}
		if glyph.GetCellRef() == nodeCell {
			t.Errorf("%s draws the generic box: give it a glyph or a glyphAliases entry", cl)
		}
	}
}

// TestDrawableClassesAcceptAStampedName covers the CLI's --class validation: a class that draws
// through an alias is a class a user may name, because the registry can draw it.
func TestDrawableClassesAcceptAStampedName(t *testing.T) {
	have := map[string]bool{}
	for _, c := range DefaultRegistry().GlyphClasses() {
		have[c] = true
	}
	for _, c := range []string{"thermistor", "zener", "clock", "test_connector", "ideal_diode_controller"} {
		if !have[c] {
			t.Errorf("GlyphClasses omits %q, so --class %q is rejected for a class that draws", c, c)
		}
	}
}

// TestDatasheetClassChoosesTheGlyph is agni issue 710 at the drawing. The classes a datasheet
// establishes now reach the IR, so the glyph follows them for the same reason it follows the
// keyword-derived ones. D1 is the case where the PICTURE moves and not only the label: nothing in a
// netlist says a part is a suppressor, so it drew as an ordinary rectifier while every rule and query
// already called it a tvs.
func TestDatasheetClassChoosesTheGlyph(t *testing.T) {
	reg := DefaultRegistry()
	cases := []struct {
		refDes    string
		tags      []*ir.ComponentClassTag
		wantClass string
		wantGlyph string
	}{
		{"D1", append(classify.Tags("diode"), datasheetTag("tvs")), "tvs", ClassTVS},
		{"Y1", append(classify.Tags("clock"), datasheetTag("crystal")), "crystal", ClassCrystal},
		{"Y2", append(classify.Tags("clock"), datasheetTag("ceramic_resonator")), "ceramic_resonator", ClassCrystal},
		// An unranked vendor class must not take the headline from the keyword class, which is what
		// keeps a corpus saying "regulator" from turning every regulator into a box.
		{"U1", append(classify.Tags("ic"), datasheetTag("regulator")), "ic", ClassIC},
	}
	for _, tc := range cases {
		c := &ir.Component{RefDes: tc.refDes, DeviceClasses: tc.tags}
		if got := reg.Classify(c, nil); got != tc.wantClass {
			t.Errorf("%s: Classify = %q, want %q", tc.refDes, got, tc.wantClass)
		}
		if got := reg.Symbol(tc.refDes, c, nil).GetCellRef(); got != reg.cellFor(tc.wantGlyph) {
			t.Errorf("%s: glyph = %q, want the %s glyph %q", tc.refDes, got, tc.wantGlyph, reg.cellFor(tc.wantGlyph))
		}
	}
}

func datasheetTag(class string) *ir.ComponentClassTag {
	return &ir.ComponentClassTag{Class: class, Source: ir.ClassSource_CLASS_SOURCE_DATASHEET}
}
