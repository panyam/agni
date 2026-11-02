package classify

import (
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
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
