package classify

import (
	"slices"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"google.golang.org/protobuf/proto"
)

func entryOf(es []LexiconEntry, vocab, kind, value string) (LexiconEntry, bool) {
	for _, e := range es {
		if e.Vocabulary == vocab && e.Kind == kind && e.Value == value {
			return e, true
		}
	}
	return LexiconEntry{}, false
}

// Each entry of the effective lexicon says whether it is the engine's or the convention's, and a
// built-in a convention switched off stays listed, marked, so a reader can see it went.
func TestEffectiveEntriesSayWhereEachCameFrom(t *testing.T) {
	over := &configpb.NamingLexicon{
		Net: &configpb.NetNameVocab{
			Rail:                    &configpb.VocabPatterns{Patterns: []string{`^LPC_VCC$`}},
			Ground:                  &configpb.VocabPatterns{Replace: true, Patterns: []string{`^0V$`}},
			InferRailFromSupplyPins: proto.Bool(false),
		},
		Class: map[string]*configpb.ClassVocab{"connector": {Prefixes: []string{"TP"}}},
	}
	es := EffectiveEntries(over)
	for _, c := range []struct {
		vocab, kind, value string
		conv, replaced     bool
	}{
		{"net.rail", "pattern", `^\+`, false, false},
		{"net.rail", "pattern", `^LPC_VCC$`, true, false},
		{"net.ground", "pattern", "GND", false, true},
		{"net.ground", "pattern", `^0V$`, true, false},
		{"net", "switch", "infer_rail_from_supply_pins=false", true, false},
		{"class.test_point", "prefix", "TP", false, true},
		{"class.connector", "prefix", "TP", true, false},
		{"class.tvs", "family", `^pesd`, false, false},
	} {
		e, ok := entryOf(es, c.vocab, c.kind, c.value)
		if !ok {
			t.Errorf("no %s %s %s entry", c.vocab, c.kind, c.value)
			continue
		}
		if e.FromConvention != c.conv || e.Replaced != c.replaced {
			t.Errorf("%s %s %s: convention=%t replaced=%t, want %t and %t", c.vocab, c.kind, c.value, e.FromConvention, e.Replaced, c.conv, c.replaced)
		}
	}
	if e, ok := entryOf(EffectiveEntries(nil), "net", "switch", "infer_rail_from_supply_pins=true"); !ok || e.FromConvention {
		t.Errorf("with no convention the switch is the built-in setting, got %+v", e)
	}
}

// EffectiveLexicon states the merge as one convention, so building a vocabulary from it matches
// building one from the original convention, list by list, including a list a replace emptied.
func TestEffectiveLexiconBuildsTheSameVocabulary(t *testing.T) {
	for name, over := range map[string]*configpb.NamingLexicon{
		"none":        nil,
		"rail added":  {Net: &configpb.NetNameVocab{Rail: &configpb.VocabPatterns{Patterns: []string{`^LPC_VCC$`}}}},
		"ground off":  {Net: &configpb.NetNameVocab{Ground: &configpb.VocabPatterns{Replace: true}}},
		"i2c exclude": {Net: &configpb.NetNameVocab{I2C: &configpb.VocabPatterns{Exclude: []string{`^DDC`}}}},
		"switch off":  {Net: &configpb.NetNameVocab{InferRailFromSupplyPins: proto.Bool(false)}},
		"pin supply":  {Pin: &configpb.PinNameVocab{Supply: &configpb.VocabPatterns{Patterns: []string{`^VLDO`}}}},
	} {
		want, err := BuildRoleVocab(over)
		if err != nil {
			t.Fatal(err)
		}
		got, err := BuildRoleVocab(EffectiveLexicon(over))
		if err != nil {
			t.Fatalf("%s: the stated lexicon does not build: %v", name, err)
		}
		for i, w := range roleVocabs(want.NamingLexicon, nil) {
			g := roleVocabs(got.NamingLexicon, nil)[i]
			if !slices.Equal(w.base.GetPatterns(), g.base.GetPatterns()) || !slices.Equal(w.base.GetExclude(), g.base.GetExclude()) {
				t.Errorf("%s: %s reads %v less %v, want %v less %v", name, w.path,
					g.base.GetPatterns(), g.base.GetExclude(), w.base.GetPatterns(), w.base.GetExclude())
			}
		}
		if got.InfersRails() != want.InfersRails() {
			t.Errorf("%s: infers rails %t, want %t", name, got.InfersRails(), want.InfersRails())
		}
	}
}
