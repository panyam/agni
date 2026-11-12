package classify

import (
	"strings"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestDiffNegative(t *testing.T) {
	house, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Pairs: []*configpb.DiffSuffix{{Positive: "_PLUS", Negative: "_MINUS"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Replace: true, Pairs: []*configpb.DiffSuffix{{Positive: "_PLUS", Negative: "_MINUS"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	overlapping, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Replace: true, Pairs: []*configpb.DiffSuffix{{Positive: "_TXP", Negative: "_TXM"}, {Positive: "P", Negative: "N"}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		v     *RoleVocab
		in    string
		want  string
		found bool
	}{
		{"_P", DefaultRoleVocab(), "CLK_P", "CLK_N", true},
		{"lower case", DefaultRoleVocab(), "clk_p", "clk_n", true},
		{"_DP", DefaultRoleVocab(), "USB_DP", "USB_DN", true},
		{"mixed case", DefaultRoleVocab(), "USB_dP", "USB_dN", true},
		{"plus", DefaultRoleVocab(), "D+", "D-", true},
		{"a negative member", DefaultRoleVocab(), "CLK_N", "", false},
		{"a house pair is unread by default", DefaultRoleVocab(), "CLK_PLUS", "", false},
		{"a house pair", house, "CLK_PLUS", "CLK_MINUS", true},
		{"a longer complement keeps the case", house, "clk_plus", "clk_minus", true},
		{"a merge keeps the built-ins", house, "CLK_P", "CLK_N", true},
		{"replace drops the built-ins", replaced, "CLK_P", "", false},
		{"the longest suffix wins", overlapping, "LANE_TXP", "LANE_TXM", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.v.DiffNegative(tc.in)
			if got != tc.want || ok != tc.found {
				t.Errorf("DiffNegative(%q) = %q, %v, want %q, %v", tc.in, got, ok, tc.want, tc.found)
			}
		})
	}
}

func TestADiffPairNeedsBothSuffixes(t *testing.T) {
	_, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Pairs: []*configpb.DiffSuffix{{Positive: "_PLUS"}},
	}}})
	if err == nil {
		t.Fatal("a pair with no negative suffix built")
	}
}

func TestNameVocabularies(t *testing.T) {
	over := func(set func(n *configpb.NetNameVocab, p *configpb.PinNameVocab)) *RoleVocab {
		t.Helper()
		lx := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{}, Pin: &configpb.PinNameVocab{}}
		set(lx.Net, lx.Pin)
		v, err := BuildRoleVocab(lx)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	add := func(p string) *configpb.VocabPatterns { return &configpb.VocabPatterns{Patterns: []string{p}} }
	for _, tc := range []struct {
		vocabulary string
		ask        func(*RoleVocab, string) bool
		builtin    []string
		not        []string
		house      string
		override   *RoleVocab
	}{
		{"pin.anode", (*RoleVocab).IsAnode, []string{"A", "AN", "ANODE", "+", "anode"}, []string{"K", "CLKA", "A/K"}, "POS",
			over(func(_ *configpb.NetNameVocab, p *configpb.PinNameVocab) { p.Anode = add("^POS$") })},
		{"pin.cathode", (*RoleVocab).IsCathode, []string{"K", "KATHODE", "CATHODE", "CAT", "-"}, []string{"A", "A/K"}, "NEG",
			over(func(_ *configpb.NetNameVocab, p *configpb.PinNameVocab) { p.Cathode = add("^NEG$") })},
		{"pin.feedback", (*RoleVocab).IsFeedbackPin, []string{"FB", "FB2", "VFB", "BUCK1_FB", "SENSE", "SNS"}, []string{"FBK", "SWFB"}, "FDBK",
			over(func(_ *configpb.NetNameVocab, p *configpb.PinNameVocab) { p.Feedback = add("^FDBK$") })},
		{"pin.switching", (*RoleVocab).IsSwitchingPin, []string{"SW", "SW1", "LX", "BOOT", "PHASE", "PH_SW"}, []string{"SWD", "SWCLK"}, "HSD",
			over(func(_ *configpb.NetNameVocab, p *configpb.PinNameVocab) { p.Switching = add("^HSD$") })},
		{"net.no_connect", (*RoleVocab).IsNoConnect, []string{"NC_1", "nc_1", "no_connect_3"}, []string{"NCLK", "unconnected-(J1-A-Pad1)"}, "OPEN_1",
			over(func(n *configpb.NetNameVocab, _ *configpb.PinNameVocab) { n.NoConnect = add("^OPEN_") })},
	} {
		t.Run(tc.vocabulary, func(t *testing.T) {
			def := DefaultRoleVocab()
			for _, n := range tc.builtin {
				if !tc.ask(def, n) {
					t.Errorf("%q is not in the built-ins", n)
				}
			}
			for _, n := range tc.not {
				if tc.ask(def, n) {
					t.Errorf("%q is in the built-ins", n)
				}
			}
			if tc.ask(def, tc.house) {
				t.Errorf("%q reads as built-in, so the override shows nothing", tc.house)
			}
			if !tc.ask(tc.override, tc.house) {
				t.Errorf("the override did not teach %q", tc.house)
			}
		})
	}
}

func TestEveryVocabularyIsListedAndSurvivesTheRoundTrip(t *testing.T) {
	over := &configpb.NamingLexicon{Net: &configpb.NetNameVocab{DiffPair: &configpb.DiffPairVocab{
		Pairs: []*configpb.DiffSuffix{{Positive: "_PLUS", Negative: "_MINUS"}},
	}}}
	es := EffectiveEntries(over)
	for _, want := range [][3]string{
		{"net.no_connect", "pattern", "^nc_"},
		{"net.diff_pair", "pair", "_DP=_DN"},
		{"net.diff_pair", "pair", "_PLUS=_MINUS"},
		{"pin.anode", "pattern", `^(A|AN|ANODE|\+)$`},
		{"pin.cathode", "pattern", "^(K|KATHODE|CATHODE|CAT|-)$"},
		{"pin.feedback", "pattern", "(^|_)(FB|VFB|FEEDBACK|VSENSE|SENSE|SNS)[0-9]*$"},
		{"pin.switching", "pattern", "(^|_)(SW|BOOT|PHASE|LX)[0-9]*$"},
	} {
		if _, ok := entryOf(es, want[0], want[1], want[2]); !ok {
			t.Errorf("%s %s %q is not listed", want[0], want[1], want[2])
		}
	}
	v, err := BuildRoleVocab(EffectiveLexicon(over))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := v.DiffNegative("CLK_PLUS"); !ok || got != "CLK_MINUS" {
		t.Errorf("the house pair did not survive the round trip: %q, %v", got, ok)
	}
	if got, ok := v.DiffNegative("CLK_P"); !ok || got != "CLK_N" {
		t.Errorf("a built-in pair did not survive the round trip: %q, %v", got, ok)
	}
}

func TestNominalVoltage(t *testing.T) {
	cases := []struct {
		name string
		want float64
		ok   bool
	}{
		{"+5V", 5, true},
		{"5V", 5, true},
		{"3V3", 3.3, true},
		{"1V8", 1.8, true},
		{"12V0", 12, true},
		{"3.3V", 3.3, true},
		{"VCC_5V", 5, true},
		{"5V0_RAIL", 5, true},
		{"+24V_IN", 24, true},
		{"vbus_5v", 5, true},
		{"GND", 0, false},
		{"PCIE", 0, false},
		{"USB5V", 0, false},     // embedded, not a delimited voltage token
		{"EVENT5", 0, false},    // trailing digit is not a voltage
		{"12V_TO_5V", 0, false}, // two conflicting nominals: refuse to guess
		{"VCC", 0, false},       // rail-ish but carries no number
		{"5V_AND_5V0", 5, true}, // two tokens, same nominal: unambiguous
	}
	for _, tc := range cases {
		got, ok := DefaultRoleVocab().NominalVoltage(tc.name)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("NominalVoltage(%q) = %v,%v want %v,%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAHouseVoltageSpelling(t *testing.T) {
	house, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{Voltage: &configpb.VocabPatterns{
		Patterns: []string{`^(\d+)P(\d+)$`},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := DefaultRoleVocab().NominalVoltage("VDD_3P3"); ok {
		t.Fatal("3P3 reads as a voltage with no convention, so the override shows nothing")
	}
	if got, ok := house.NominalVoltage("VDD_3P3"); !ok || got != 3.3 {
		t.Errorf("VDD_3P3 under the convention = %v, %v, want 3.3", got, ok)
	}
	if got, ok := house.NominalVoltage("3V3"); !ok || got != 3.3 {
		t.Errorf("the merge dropped the built-ins: 3V3 = %v, %v", got, ok)
	}
	if _, ok := house.NominalVoltage("3P3_TO_5V"); ok {
		t.Error("two disagreeing tokens read as a voltage")
	}
	_, err = BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{Voltage: &configpb.VocabPatterns{
		Patterns: []string{`^\d+P\d+$`},
	}}})
	if err == nil || !strings.Contains(err.Error(), "capture group") {
		t.Errorf("a voltage pattern with no capture group built: %v", err)
	}
}

func TestAHouseValueUnit(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{compWithValue("C1", "100", string(ClassCapacitor))}}
	DefaultLexicon().StampValues(d)
	if u := d.Components[0].GetValue().GetUnit(); u != "" {
		t.Fatalf("a bare capacitor value has unit %q with no convention", u)
	}
	for unit, want := range map[string]float64{"farad": 100, "pF": 1e-10, "µF": 1e-4} {
		vv, err := BuildValueVocab(map[string]string{"capacitor": unit})
		if err != nil {
			t.Fatal(err)
		}
		(&Lexicon{Value: vv}).StampValues(d)
		if q := d.Components[0].GetValue(); q.GetUnit() != "F" || q.GetValue() != want {
			t.Errorf("a bare capacitor 100 declared in %s reads %v %q, want %v F", unit, q.GetValue(), q.GetUnit(), want)
		}
	}
	prefixed := &ir.Design{Components: []*ir.Component{compWithValue("C2", "10u", string(ClassCapacitor))}}
	vv, _ := BuildValueVocab(map[string]string{"capacitor": "pF"})
	(&Lexicon{Value: vv}).StampValues(prefixed)
	if q := prefixed.Components[0].GetValue(); q.GetValue() != 1e-5 {
		t.Errorf("a value that states its own multiplier was rescaled: 10u reads %v", q.GetValue())
	}
	for in, want := range map[string]string{"widget": "unknown component class", "capacitor": "not one the engine reads"} {
		unit := "F"
		if in == "capacitor" {
			unit = "furlong"
		}
		if _, err := BuildValueVocab(map[string]string{in: unit}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("value_unit %s: %s built with %v, want an error saying %q", in, unit, err, want)
		}
	}
}

func TestExcludingABuiltinPrefix(t *testing.T) {
	v, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassDiode: {ExcludePrefixes: []string{"d"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := DefaultLexicon().Classify(&ir.Component{RefDes: "D4"}, &ir.PartType{}); got != ClassDiode {
		t.Fatalf("D4 reads %s with no convention", got)
	}
	if got := (&Lexicon{Class: v}).Classify(&ir.Component{RefDes: "D4"}, &ir.PartType{}); got == ClassDiode {
		t.Error("D4 still reads as a diode with D excluded")
	}
	if got := DefaultLexicon().Classify(&ir.Component{RefDes: "D4"}, &ir.PartType{}); got != ClassDiode {
		t.Errorf("the exclusion leaked into the defaults: D4 reads %s", got)
	}
	for name, over := range map[string]map[ComponentClass]*configpb.ClassVocab{
		"another class's prefix":  {ClassResistor: {ExcludePrefixes: []string{"D"}}},
		"one the convention adds": {ClassDiode: {ExcludePrefixes: []string{"D"}}, ClassLED: {Prefixes: []string{"D"}}},
	} {
		if _, err := BuildClassVocab(over); err == nil {
			t.Errorf("%s: excluding it built", name)
		}
	}
}

func TestUnitsAndExclusionsAreListedAndSurviveTheRoundTrip(t *testing.T) {
	over := &configpb.NamingLexicon{
		ValueUnit: map[string]string{"capacitor": "F", "resistor": "ohm"},
		Class:     map[string]*configpb.ClassVocab{"diode": {ExcludePrefixes: []string{"D"}}},
		Net:       &configpb.NetNameVocab{Voltage: &configpb.VocabPatterns{Patterns: []string{`^(\d+)P(\d+)$`}}},
	}
	es := EffectiveEntries(over)
	for _, want := range []struct {
		vocab, kind, value string
		conv, replaced     bool
	}{
		{"value_unit.resistor", "unit", UnitOhm, false, true},
		{"value_unit.resistor", "unit", "ohm", true, false},
		{"value_unit.capacitor", "unit", "F", true, false},
		{"class.diode", "prefix", "D", false, true},
		{"class.diode", "exclude_prefix", "D", true, false},
		{"net.voltage", "pattern", `^(\d+)P(\d+)$`, true, false},
	} {
		e, ok := entryOf(es, want.vocab, want.kind, want.value)
		if !ok || e.FromConvention != want.conv || e.Replaced != want.replaced {
			t.Errorf("%s %s %q: listed %v as %+v", want.vocab, want.kind, want.value, ok, e)
		}
	}
	eff := EffectiveLexicon(over)
	if got := eff.GetValueUnit(); got["capacitor"] != "F" || got["resistor"] != "ohm" {
		t.Errorf("the units did not survive the round trip: %v", got)
	}
	if got := eff.GetClass()["diode"].GetExcludePrefixes(); len(got) != 1 || got[0] != "D" {
		t.Errorf("the exclusion did not survive the round trip: %v", got)
	}
	if v, err := BuildRoleVocab(eff); err != nil {
		t.Fatal(err)
	} else if got, ok := v.NominalVoltage("3P3"); !ok || got != 3.3 {
		t.Errorf("the voltage pattern did not survive the round trip: %v, %v", got, ok)
	}
}
