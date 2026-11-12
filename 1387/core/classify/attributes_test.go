package classify

import (
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func attrLexicon(t *testing.T, a *configpb.AttributeVocab) *Lexicon {
	t.Helper()
	v, err := BuildRoleVocab(&configpb.NamingLexicon{Attribute: a})
	if err != nil {
		t.Fatal(err)
	}
	return &Lexicon{Role: v}
}

func keys(k ...string) *configpb.AttributeKeys { return &configpb.AttributeKeys{Keys: k} }

func TestDoNotPopulate(t *testing.T) {
	house := attrLexicon(t, &configpb.AttributeVocab{Dnp: keys("DNF"), DnpValues: keys("x")})
	for _, tc := range []struct {
		name  string
		attrs map[string]string
		lex   *Lexicon
		want  bool
	}{
		{"KiCad's flag", map[string]string{"dnp": "yes"}, DefaultLexicon(), true},
		{"a DNP field saying DNP", map[string]string{"DNP": "DNP"}, DefaultLexicon(), true},
		{"a DNP field saying y", map[string]string{"DNP": "y"}, DefaultLexicon(), true},
		{"an empty DNP field", map[string]string{"DNP": ""}, DefaultLexicon(), false},
		{"KiCad's no beside a field saying DNP", map[string]string{"dnp": "no", "DNP": "DNP"}, DefaultLexicon(), true},
		{"a house key with no convention", map[string]string{"DNF": "yes"}, DefaultLexicon(), false},
		{"a house key", map[string]string{"DNF": "yes"}, house, true},
		{"a house value", map[string]string{"DNF": "x"}, house, true},
		{"the built-ins survive the merge", map[string]string{"dnp": "yes"}, house, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.lex.RoleVocab().IsUnfitted(&ir.Component{Attributes: tc.attrs}); got != tc.want {
				t.Errorf("IsUnfitted(%v) = %v, want %v", tc.attrs, got, tc.want)
			}
		})
	}
}

func TestPartNumberKeys(t *testing.T) {
	stamp := func(lex *Lexicon, attrs map[string]string) string {
		d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Attributes: attrs}}}
		lex.StampMPN(d)
		return d.Components[0].GetMpn()
	}
	if got := stamp(DefaultLexicon(), map[string]string{"Part Number": "LM1117"}); got != "LM1117" {
		t.Errorf("KiCad's Part Number field reads %q", got)
	}
	if got := stamp(DefaultLexicon(), map[string]string{"part number": "LM1117"}); got != "LM1117" {
		t.Errorf("a key differing in case reads %q", got)
	}
	if got := stamp(DefaultLexicon(), map[string]string{"MPN": "A", "Part Number": "B"}); got != "A" {
		t.Errorf("the first key in order should win, got %q", got)
	}
	house := attrLexicon(t, &configpb.AttributeVocab{Mpn: keys("Manufacturer_No")})
	if got := stamp(DefaultLexicon(), map[string]string{"Manufacturer_No": "TPS62133"}); got != "" {
		t.Fatalf("a house key reads %q with no convention, so the override shows nothing", got)
	}
	if got := stamp(house, map[string]string{"Manufacturer_No": "TPS62133"}); got != "TPS62133" {
		t.Errorf("a house key under the convention reads %q", got)
	}
	replaced := attrLexicon(t, &configpb.AttributeVocab{Mpn: &configpb.AttributeKeys{Keys: []string{"Manufacturer_No"}, Replace: true}})
	if got := stamp(replaced, map[string]string{"MPN": "A"}); got != "" {
		t.Errorf("replace kept a built-in key: %q", got)
	}
}

func TestValueKeys(t *testing.T) {
	house := attrLexicon(t, &configpb.AttributeVocab{Value: keys("Wert")})
	c := func() *ir.Component { return compWithValue("R1", "", string(ClassResistor)) }
	plain := c()
	plain.Attributes = map[string]string{"Wert": "4k7"}
	DefaultLexicon().StampValues(&ir.Design{Components: []*ir.Component{plain}})
	if plain.GetValue() != nil {
		t.Fatalf("a house value key reads %v with no convention", plain.GetValue())
	}
	taught := c()
	taught.Attributes = map[string]string{"Wert": "4k7"}
	house.StampValues(&ir.Design{Components: []*ir.Component{taught}})
	if got := taught.GetValue().GetValue(); got != 4700 {
		t.Errorf("a house value key under the convention reads %v", got)
	}
}

func TestAttributeKeysAreListedAndSurviveTheRoundTrip(t *testing.T) {
	over := &configpb.NamingLexicon{Attribute: &configpb.AttributeVocab{
		Dnp: keys("DNF"), Mpn: &configpb.AttributeKeys{Keys: []string{"Manufacturer_No"}, Replace: true},
	}}
	es := EffectiveEntries(over)
	for _, want := range []struct {
		vocab, value   string
		conv, replaced bool
	}{
		{"attribute.mpn", "Part Number", false, true},
		{"attribute.mpn", "Manufacturer_No", true, false},
		{"attribute.dnp", "dnp", false, false},
		{"attribute.dnp", "DNF", true, false},
		{"attribute.dnp_values", "y", false, false},
		{"attribute.value", "value", false, false},
	} {
		e, ok := entryOf(es, want.vocab, "key", want.value)
		if !ok || e.FromConvention != want.conv || e.Replaced != want.replaced {
			t.Errorf("%s %q: listed %v as %+v", want.vocab, want.value, ok, e)
		}
	}
	v, err := BuildRoleVocab(EffectiveLexicon(over))
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsUnfitted(&ir.Component{Attributes: map[string]string{"DNF": "yes"}}) {
		t.Error("the house dnp key did not survive the round trip")
	}
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Attributes: map[string]string{"MPN": "A", "Manufacturer_No": "B"}}}}
	(&Lexicon{Role: v}).StampMPN(d)
	if got := d.Components[0].GetMpn(); got != "B" {
		t.Errorf("the replaced mpn list did not survive the round trip: %q", got)
	}
}

func TestASectionsAttributesCount(t *testing.T) {
	c := &ir.Component{RefDes: "J1", Sections: []*ir.ComponentSection{{Attributes: map[string]string{"DNP": "DNP", "Part Number": "X1"}}}}
	if !DefaultLexicon().RoleVocab().IsUnfitted(c) {
		t.Error("a section's DNP field did not mark the part unfitted")
	}
	d := &ir.Design{Components: []*ir.Component{c}}
	DefaultLexicon().StampMPN(d)
	if got := c.GetMpn(); got != "X1" {
		t.Errorf("a section's Part Number reads %q", got)
	}
}
