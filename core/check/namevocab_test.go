package check

import (
	"testing"

	"github.com/panyam/agni/core/classify"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func lexiconOf(t *testing.T, lx *configpb.NamingLexicon) ModelOption {
	t.Helper()
	rv, err := classify.BuildRoleVocab(lx)
	if err != nil {
		t.Fatal(err)
	}
	return WithLexicon(&classify.Lexicon{Role: rv})
}

// A convention reaches each model question with the process-level vocabulary at its defaults, which is
// the half a vocabulary test in classify cannot show (agni issue 1285).
func TestAModelReadsItsOwnNameVocabularies(t *testing.T) {
	d := func(nets ...string) *ir.Design {
		out := &ir.Design{}
		for _, n := range nets {
			out.Nets = append(out.Nets, &ir.Net{Name: n})
		}
		return out
	}
	open := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{NoConnect: &configpb.VocabPatterns{Patterns: []string{`^OPEN_`}}}}
	pair := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Pairs: []*configpb.DiffSuffix{{Positive: "_PLUS", Negative: "_MINUS"}},
	}}}

	t.Run("no_connect", func(t *testing.T) {
		design := d("OPEN_1")
		if IntentionallyUnconnected(NewModel(design), design.Nets[0]) {
			t.Error("OPEN_1 reads as a no-connect with no convention")
		}
		if !IntentionallyUnconnected(NewModel(design, lexiconOf(t, open)), design.Nets[0]) {
			t.Error("the convention did not make OPEN_1 a no-connect")
		}
	})
	t.Run("diff_pair", func(t *testing.T) {
		design := d("CLK_PLUS", "CLK_MINUS")
		if DiffConventionPresent(NewModel(design)) {
			t.Error("CLK_PLUS pairs with no convention")
		}
		if !DiffConventionPresent(NewModel(design, lexiconOf(t, pair))) {
			t.Error("the convention did not pair CLK_PLUS")
		}
	})
	t.Run("voltage", func(t *testing.T) {
		house := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{Voltage: &configpb.VocabPatterns{Patterns: []string{`^(\d+)P(\d+)$`}}}}
		rail := &ir.Net{Name: "VDD_3P3"}
		if _, ok := NewModel(&ir.Design{}).RailMaxVoltage(rail); ok {
			t.Error("VDD_3P3 states a voltage with no convention")
		}
		if got, ok := NewModel(&ir.Design{}, lexiconOf(t, house)).RailMaxVoltage(rail); !ok || got != 3.3 {
			t.Errorf("VDD_3P3 under the convention = %v, %v, want 3.3", got, ok)
		}
	})
	t.Run("polarity", func(t *testing.T) {
		cathode := &configpb.NamingLexicon{Pin: &configpb.PinNameVocab{Cathode: &configpb.VocabPatterns{Patterns: []string{`^NEG$`}}}}
		base := NewModel(&ir.Design{}).(*irModel)
		if got := classifyPinRole(base, "NEG", ClassDiode); got != RoleUnknown {
			t.Errorf("NEG on a diode reads %v with no convention", got)
		}
		m := NewModel(&ir.Design{}, lexiconOf(t, cathode)).(*irModel)
		if got := classifyPinRole(m, "NEG", ClassDiode); got != RoleCathode {
			t.Errorf("NEG on a diode reads %v under the convention", got)
		}
		if got := classifyPinRole(m, "NEG", ClassResistor); got == RoleCathode {
			t.Error("the class gate let NEG read as a cathode on a resistor")
		}
	})
}

func TestAModelReadsItsOwnDoNotPopulateKeys(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "R1", Attributes: map[string]string{"DNF": "yes"}}}}
	if !NewModel(d).IsFitted("R1") {
		t.Fatal("R1 reads unfitted with no convention")
	}
	house := &configpb.NamingLexicon{Attribute: &configpb.AttributeVocab{Dnp: &configpb.AttributeKeys{Keys: []string{"DNF"}}}}
	if NewModel(d, lexiconOf(t, house)).IsFitted("R1") {
		t.Error("the convention's DNF key did not mark R1 unfitted")
	}
}
