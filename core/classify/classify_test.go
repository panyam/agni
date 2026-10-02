package classify

import (
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestClassifyPlaceholderPrefix covers a part's declared designator_prefix, which arrives as
// printed, and capture tools print the annotation-placeholder form ("C?", "REF**"). The tail is
// trimmed before the table lookup. Without it every part-typed component on the Mentor EDIF corpus
// classified unknown.
func TestClassifyPlaceholderPrefix(t *testing.T) {
	c := &ir.Component{RefDes: "C3154"}
	pt := &ir.PartType{Name: "GCJ188R71H224KA01D", DesignatorPrefix: "C?"}
	if got := Classify(c, pt); got != ClassCapacitor {
		t.Errorf("classify with C? prefix = %s, want capacitor", got)
	}
	if got := Classify(&ir.Component{RefDes: "X1"}, &ir.PartType{DesignatorPrefix: "REF**"}); got != ClassUnknown {
		t.Errorf("REF** prefix = %s, want unknown (trimmed REF is unmapped)", got)
	}
}

// TestStampFillsDeviceClasses is the left-shift proof. Stamp classifies every component once over
// the read IR and writes the device_classes set, resolving part-type sections through the shared
// index. A component the classifier cannot place (unknown) carries no tag, so the set stays
// accurate.
func TestStampFillsDeviceClasses(t *testing.T) {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "LED_0805", DesignatorPrefix: "D"},
		}}},
		Components: []*ir.Component{
			{RefDes: "R1"},
			{RefDes: "D2", Sections: []*ir.ComponentSection{{LibraryRef: "lib", PartRef: "LED_0805"}},
				Attributes: map[string]string{"Value": "LED"}},
			{RefDes: "W9"}, // unmapped prefix -> unknown -> empty set
		},
	}
	Stamp(d)
	got := map[string][]string{}
	for _, c := range d.Components {
		got[c.RefDes] = ClassNames(c)
	}
	if want := []string{string(ClassResistor)}; !equal(got["R1"], want) {
		t.Errorf("R1 device_classes = %v, want %v", got["R1"], want)
	}
	// an LED carries its diode family tag too (WS3-071 set expansion)
	if want := []string{string(ClassLED), string(ClassDiode)}; !equal(got["D2"], want) {
		t.Errorf("D2 device_classes = %v, want %v", got["D2"], want)
	}
	if len(got["W9"]) != 0 {
		t.Errorf("W9 (unknown) device_classes = %v, want empty", got["W9"])
	}
}

// TestClassesOf checks that a class with a subtype family carries the family tag too;
// test_connector does NOT carry connector (WS3-066 split); unknown is the empty set.
func TestClassesOf(t *testing.T) {
	cases := []struct {
		in   ComponentClass
		want []string
	}{
		{ClassUnknown, nil},
		{ClassResistor, []string{"resistor"}},
		{ClassTVS, []string{"tvs", "diode"}},
		{ClassLED, []string{"led", "diode"}},
		{ClassZener, []string{"zener", "diode"}},
		{ClassFerrite, []string{"ferrite", "inductor"}},
		{ClassTestConnector, []string{"test_connector"}},         // NOT connector, and the split is deliberate
		{ClassInternalConnector, []string{"internal_connector"}}, // NOT connector either (agni issue 815)
		{ClassConnector, []string{"connector"}},
	}
	for _, tc := range cases {
		if got := ClassesOf(tc.in); !equal(got, tc.want) {
			t.Errorf("ClassesOf(%s) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestMostSpecific checks that the specific class wins over its family tag so the Model's single
// component.class stays stable as the set widens; a non-token-hint class (ic) still resolves; an
// empty set is unknown.
func TestMostSpecific(t *testing.T) {
	cases := []struct {
		in   []string
		want ComponentClass
	}{
		{[]string{"tvs", "diode"}, ClassTVS},
		{[]string{"diode", "tvs"}, ClassTVS},
		{[]string{"ic"}, ClassIC},
		{nil, ClassUnknown},
	}
	for _, tc := range cases {
		if got := MostSpecific(tc.in); got != tc.want {
			t.Errorf("MostSpecific(%v) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A thermistor classified UNKNOWN, so component.class emitted no row for it and it fell out of every
// class-scoped rule and query. Silently, because an absent row and a row that did not match look
// identical downstream. Found reconciling per-part coverage against a second tool on a real board,
// where the whole residual in one direction was 15 thermistors, present in the netlist and invisible
// (agni issue 627).
func TestClassifyThermistor(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    *ir.Component
		pt   *ir.PartType
	}{
		{"the RT prefix", &ir.Component{RefDes: "RT1"}, nil},
		// A project whose ref-des convention differs still resolves from the part text, without
		// declaring a prefix in its lexicon (agni 677).
		{"an NTC in the part text", &ir.Component{RefDes: "X9"}, &ir.PartType{Name: "NTC 10K 0603"}},
		{"a PTC in the part text", &ir.Component{RefDes: "X9"}, &ir.PartType{Name: "PTC resettable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.c, tc.pt); got != ClassThermistor {
				t.Errorf("Classify = %s, want thermistor", got)
			}
		})
	}
}

// The family tag is what makes the fix reach the analyses that motivated it. A thermistor is a
// two-terminal resistor for every topological question, so test-point coverage and divider topology
// must see it without each learning a new class name.
func TestThermistorCarriesTheResistorFamily(t *testing.T) {
	got := ClassesOf(ClassThermistor)
	want := map[string]bool{"thermistor": true, "resistor": true}
	if len(got) != len(want) {
		t.Fatalf("ClassesOf(thermistor) = %v, want thermistor and resistor", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected tag %q in %v", g, got)
		}
	}
}

// THE CONTROL, and the direction that is easy to get backwards. A family points from the specific to
// the general, so a thermistor is a resistor and a plain resistor is NOT a thermistor. Declaring the
// family the other way round would pass every other test here while tagging 1110 resistors on a real
// board as temperature sensors.
func TestAPlainResistorIsNotAThermistor(t *testing.T) {
	if got := Classify(&ir.Component{RefDes: "R1"}, nil); got != ClassResistor {
		t.Fatalf("Classify(R1) = %s, want resistor", got)
	}
	for _, tag := range ClassesOf(ClassResistor) {
		if tag == string(ClassThermistor) {
			t.Error("a plain resistor carries the thermistor tag, so the family points the wrong way")
		}
	}
}

// A mezzanine connector is a board-to-board joint, refined out of the J-prefix connector base as a
// debug header is (agni issue 815), and a project's lexicon can put its own connectors in the class,
// which is how a part number with no telling word in it gets there.
func TestInternalConnectorClassification(t *testing.T) {
	comp := func(ref, value string) *ir.Component {
		return &ir.Component{RefDes: ref, Attributes: map[string]string{"Value": value}}
	}
	if got := Classify(comp("J5", "Mezzanine 80-pin"), nil); got != ClassInternalConnector {
		t.Errorf("a mezzanine connector classifies as %s, want internal_connector", got)
	}
	if got := Classify(comp("J6", "USB-C receptacle"), nil); got != ClassConnector {
		t.Errorf("a USB receptacle classifies as %s, want connector", got)
	}
	cv, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassInternalConnector: {Patterns: []string{"^218650$"}}})
	if err != nil {
		t.Fatal(err)
	}
	lex := &Lexicon{Class: cv}
	if got := lex.Classify(comp("J18", "ASP-218650-01"), nil); got != ClassInternalConnector {
		t.Errorf("a project-declared internal connector classifies as %s, want internal_connector", got)
	}
}
