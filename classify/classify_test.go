package classify

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestClassifyPlaceholderPrefix: a part's declared designator_prefix arrives as printed, and capture
// tools print the annotation-placeholder form ("C?", "REF**"); the tail is trimmed before the table
// lookup — without it every part-typed component on the Mentor EDIF corpus classified unknown.
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

// TestStampFillsDeviceClasses is the left-shift proof: Stamp classifies every component once over the
// read IR and writes the device_classes set, resolving part-type sections through the shared index. A
// component the classifier cannot place (unknown) carries no tag, so the set stays honest.
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
		got[c.RefDes] = c.DeviceClasses
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

// TestClassesOf: a class with a subtype family carries the family tag too; test_connector does NOT
// carry connector (WS3-066 split); unknown is the empty set.
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
		{ClassTestConnector, []string{"test_connector"}}, // NOT connector — the split is deliberate
		{ClassConnector, []string{"connector"}},
	}
	for _, tc := range cases {
		if got := ClassesOf(tc.in); !equal(got, tc.want) {
			t.Errorf("ClassesOf(%s) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestMostSpecific: the specific class wins over its family tag so the Model's single component.class
// stays stable as the set widens; a non-token-hint class (ic) still resolves; an empty set is unknown.
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
