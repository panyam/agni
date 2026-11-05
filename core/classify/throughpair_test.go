package classify

import (
	"reflect"
	"strings"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

func TestThroughPartnersReadTheBuiltInPairs(t *testing.T) {
	v := DefaultRoleVocab()
	for name, want := range map[string][]string{
		"A2":   {"B2"},
		"b12":  {"A12"},
		"1A":   {"1Y"},
		"2Y":   {"2A"},
		"SDA1": {"SDA2"},
		"scl2": {"SCL1"},
		"VCCA": nil,
		"A":    nil,
		"OE":   nil,
	} {
		if got := v.ThroughPartners(name); !reflect.DeepEqual(got, want) {
			t.Errorf("ThroughPartners(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestThroughPairsTakeAProjectsOverride(t *testing.T) {
	add, err := BuildRoleVocab(&configpb.NamingLexicon{Pin: &configpb.PinNameVocab{ThroughPair: &configpb.PinPairVocab{
		Pairs: []*configpb.PinPair{{A: "IO{n}_L", B: "IO{n}_H"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := add.ThroughPartners("IO3_L"); !reflect.DeepEqual(got, []string{"IO3_H"}) {
		t.Errorf("a project's pair: %v", got)
	}
	if got := add.ThroughPartners("A2"); len(got) != 1 {
		t.Errorf("adding a pair dropped the built-ins: %v", got)
	}
	off, err := BuildRoleVocab(&configpb.NamingLexicon{Pin: &configpb.PinNameVocab{ThroughPair: &configpb.PinPairVocab{Replace: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := off.ThroughPartners("A2"); got != nil {
		t.Errorf("replace with no pairs left %v", got)
	}
	for _, bad := range []*configpb.PinPair{{A: "A{n}", B: "B"}, {A: "", B: "B1"}} {
		_, err := BuildRoleVocab(&configpb.NamingLexicon{Pin: &configpb.PinNameVocab{ThroughPair: &configpb.PinPairVocab{Pairs: []*configpb.PinPair{bad}}}})
		if err == nil || !strings.Contains(err.Error(), "through_pair") {
			t.Errorf("pair %v: err = %v, want a through_pair error", bad, err)
		}
	}
}

func TestThroughPairsAreListedInTheLexicon(t *testing.T) {
	n := 0
	for _, e := range EffectiveEntries(nil) {
		if e.Vocabulary == "pin.through_pair" && e.Kind == "pair" {
			n++
		}
	}
	if n != 4 {
		t.Errorf("the lexicon lists %d through pairs, want the 4 built-ins", n)
	}
	if got := EffectiveLexicon(nil).GetPin().GetThroughPair().GetPairs(); len(got) != 4 || PinPairEntry(got[0]) != "A{n}=B{n}" {
		t.Errorf("the effective lexicon's pairs = %v", got)
	}
}
