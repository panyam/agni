package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// railFromConnector is a connector J1 whose pin sits on 3V3 beside U1's supply pin, plus whatever
// else the case wires onto the board. U2 is the converter, with a switch pin, a feedback pin and a
// power-output pin, each on whichever net the case gives it.
func railFromConnector(nets map[string][]string) *ir.Design {
	all := map[string][]string{"3V3": {"J1.1", "U1.1"}, "GND": {"U1.2", "J1.2"}}
	for n, pins := range nets {
		all[n] = append(all[n], pins...)
	}
	d := pinDesign(map[string][]supplyPin{
		"J1": twoPin(), "U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}},
		"U2": {{"SW", "1", pass}, {"FB", "2", pass}, {"OUT", "3", pwrOut}, {"VIN", "4", pwrIn}},
		"L1": twoPin(), "R2": twoPin(), "R3": twoPin(),
		"U3": {{"VBUS", "1", pwrIn}, {"LDO", "2", pwrOut}},
		"U4": {{"VIN", "1", pwrIn}, {"VOUT", "2", pwrOut}},
	}, all)
	classify.Stamp(d)
	classify.StampNetRoles(d)
	return d
}

// A connector on a rail the board drives is an outlet, so neither entry rule judges it (agni issue
// 1016). The plain connector is the positive control, and the boost input is the shape that reaches
// a switch node through an inductor while still being where power arrives. So is a dual-role port.
func TestAConnectorOnARailTheBoardDrivesIsAnOutlet(t *testing.T) {
	for _, tc := range []struct {
		name string
		nets map[string][]string
		want check.Outcome
	}{
		{"a connector feeding a load", nil, check.Fail},
		{"a buck's output", map[string][]string{
			"3V3": {"L1.2", "R2.1"}, "3V3_SW": {"L1.1", "U2.1"}, "3V3_FB": {"R2.2", "R3.1", "U2.2"}, "GND": {"R3.2"},
		}, check.NotConsidered},
		{"a boost's input", map[string][]string{
			"3V3": {"L1.1", "U2.4"}, "5V_SW": {"L1.2", "U2.1"}, "5V_FB": {"R2.2", "R3.1", "U2.2"}, "5V": {"R2.1"}, "GND": {"R3.2"},
		}, check.Fail},
		{"an LDO's output", map[string][]string{"3V3": {"U2.3"}}, check.NotConsidered},
		// A dual-role port: a switch sources the net and a power-path controller sinks it, so power
		// can still arrive through the connector.
		{"a dual-role port", map[string][]string{"3V3": {"U2.3", "U3.1"}, "LDO_OUT": {"U3.2"}}, check.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := railFromConnector(tc.nets)
			for _, r := range []*check.Rule{reverseBlockingAbsent, inputProtection} {
				if got := verdictOn(t, r, d, "3V3"); got != string(tc.want) {
					t.Errorf("%s on 3V3 = %q, want %q", r.Name, got, tc.want)
				}
			}
			specFires := len(inputProtectionSpec.Eval(context.Background(), check.NewModel(d))) > 0
			if specFires != (tc.want == check.Fail) {
				t.Errorf("input-protection's spec twin fires = %v, want %v", specFires, tc.want == check.Fail)
			}
		})
	}
}

// Two shapes of a rail the board drives that BoardSource missed (agni issue 1253). A cascade of
// regulators is not a dual-role port, and a fixed-output buck tied in through a net tie names its
// converter only in its pin names.
func TestARegulatorCascadeIsBoardDriven(t *testing.T) {
	for _, tc := range []struct {
		name string
		nets map[string][]string
		want check.Outcome
	}{
		{"the upstream regulator is fed from elsewhere", map[string][]string{
			"5V": {"U2.4"}, "3V3": {"U2.3", "U4.1"}, "LDO_OUT": {"U4.2"},
		}, check.NotConsidered},
		{"the sink feeds the driver back, as a dual-role port does", map[string][]string{
			"3V3": {"U2.3", "U3.1"}, "LDO_OUT": {"U3.2", "U2.4"},
		}, check.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := railFromConnector(tc.nets)
			for _, r := range []*check.Rule{reverseBlockingAbsent, inputProtection} {
				if got := verdictOn(t, r, d, "3V3"); got != string(tc.want) {
					t.Errorf("%s on 3V3 = %q, want %q", r.Name, got, tc.want)
				}
			}
		})
	}
}

// A dual-role USB-C port whose sink is read from its pin names (agni issue 1934). On Jetson a load
// switch fed from +5V sources VBUS, and a PD controller's VBUS1 pin, typed inout, sinks it. The
// controller feeds nothing back to the switch, so only the pin's name says power can arrive.
func TestACableSupplyPinKeepsAPortJudged(t *testing.T) {
	inout := ir.PinDirection_PIN_DIRECTION_INOUT
	for _, tc := range []struct {
		name  string
		parts map[string][]supplyPin
		nets  map[string][]string
		want  check.Outcome
	}{
		{"a PD controller's VBUS pin typed inout", map[string][]supplyPin{
			"U5": {{"VBUS1", "1", inout}, {"LDO_3V3", "2", pwrOut}},
		}, map[string][]string{"5V": {"U2.4"}, "3V3": {"U2.3", "U5.1"}, "LDO_3V3": {"U5.2"}}, check.Fail},
		{"a USB device's VBUS pin typed passive", map[string][]supplyPin{
			"U5": {{"VBUS", "1", pass}},
		}, map[string][]string{"5V": {"U2.4"}, "3V3": {"U2.3", "U5.1"}}, check.Fail},
		{"an LDO's VIN typed passive, fed by the switch", map[string][]supplyPin{
			"U5": {{"VIN", "1", pass}, {"VOUT", "2", pass}},
		}, map[string][]string{"5V": {"U2.4"}, "3V3": {"U2.3", "U5.1"}, "LDO_3V3": {"U5.2"}}, check.NotConsidered},
		{"a VBUS detect pin", map[string][]supplyPin{
			"U5": {{"VBUS_DET", "1", inout}},
		}, map[string][]string{"5V": {"U2.4"}, "3V3": {"U2.3", "U5.1"}}, check.NotConsidered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := railFromConnector(tc.nets)
			extra := pinDesign(tc.parts, nil)
			d.Libraries[0].Parts = append(d.Libraries[0].Parts, extra.Libraries[0].Parts...)
			d.Components = append(d.Components, extra.Components...)
			classify.Stamp(d)
			classify.StampNetRoles(d)
			for _, r := range []*check.Rule{reverseBlockingAbsent, inputProtection} {
				if got := verdictOn(t, r, d, "3V3"); got != string(tc.want) {
					t.Errorf("%s on 3V3 = %q, want %q", r.Name, got, tc.want)
				}
			}
			specFires := len(inputProtectionSpec.Eval(context.Background(), check.NewModel(d))) > 0
			if specFires != (tc.want == check.Fail) {
				t.Errorf("input-protection's spec twin fires = %v, want %v", specFires, tc.want == check.Fail)
			}
		})
	}
}

// A USB TVS array names a pin VBUS too, and a clamp draws no power (agni issue 1257). A project that
// takes VBUS out of its cable_supply vocabulary gets the board-driven read back.
func TestACableSupplyPinIsAPowerSink(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mpn   string
		cable *configpb.VocabPatterns
		want  bool
	}{
		{"a PD controller's VBUS pin", "", nil, false},
		{"a TVS array's VBUS pin", "TPD4E05U06", nil, true},
		{"a convention whose cable_supply leaves VBUS out", "", &configpb.VocabPatterns{Patterns: []string{`^VCABLE$`}, Replace: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := pinDesign(map[string][]supplyPin{
				"J1": twoPin(), "U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}},
				"U2": {{"OUT", "1", pwrOut}, {"IN", "2", pwrIn}},
				"U5": {{"VBUS", "1", ir.PinDirection_PIN_DIRECTION_INOUT}, {"GND", "2", pass}},
			}, map[string][]string{
				"3V3": {"J1.1", "U1.1", "U2.1", "U5.1"}, "5V": {"U2.2"}, "GND": {"U1.2", "J1.2", "U5.2"},
			})
			for _, c := range d.Components {
				if c.RefDes == "U5" {
					c.Attributes = map[string]string{"MPN": tc.mpn}
				}
			}
			classify.Stamp(d)
			classify.StampNetRoles(d)
			rv, err := classify.BuildRoleVocab(&configpb.NamingLexicon{Pin: &configpb.PinNameVocab{CableSupply: tc.cable}})
			if err != nil {
				t.Fatalf("BuildRoleVocab: %v", err)
			}
			m := check.NewModel(d, check.WithLexicon(&classify.Lexicon{Role: rv}))
			if isTVS := m.HasClass("U5", check.ClassTVS); isTVS != (tc.mpn != "") {
				t.Fatalf("U5 reads as a TVS = %v, want %v", isTVS, tc.mpn != "")
			}
			var rail *ir.Net
			for _, n := range d.Nets {
				if n.Name == "3V3" {
					rail = n
				}
			}
			if got := check.BoardSource(m, rail); (got != "") != tc.want {
				t.Errorf("BoardSource = %q, want board driven = %v", got, tc.want)
			}
		})
	}
}

func TestAFixedBuckBehindANetTieIsBoardDriven(t *testing.T) {
	for _, tc := range []struct {
		name string
		fb   string
		want check.Outcome
	}{
		{"its feedback pin sits on the output", "REG_OUT", check.NotConsidered},
		{"another part's feedback pin is not this converter's", "", check.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nets := map[string][]string{
				"3V3": {"J1.1", "U1.1", "NT1.1"}, "GND": {"U1.2", "J1.2"},
				"REG_OUT": {"NT1.2", "L1.2"}, "Net-(U2-SW1)": {"L1.1", "U2.1"},
			}
			if tc.fb != "" {
				nets[tc.fb] = append(nets[tc.fb], "U2.2")
			} else {
				nets["REG_OUT"] = append(nets["REG_OUT"], "U3.1")
			}
			d := pinDesign(map[string][]supplyPin{
				"J1": twoPin(), "U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}},
				"U2": {{"SW1", "1", pass}, {"FB1", "2", pass}}, "U3": {{"FB", "1", pass}},
				"L1": twoPin(), "NT1": twoPin(),
			}, nets)
			classify.Stamp(d)
			classify.StampNetRoles(d)
			if got := verdictOn(t, inputProtection, d, "3V3"); got != string(tc.want) {
				t.Errorf("input-protection on 3V3 = %q, want %q", got, tc.want)
			}
		})
	}
}

// A project's pin vocabularies decide which pin names BoardSource reads, in both directions, with the
// process-level vocabulary left at its defaults (agni issues 1284, 1285).
func TestBoardSourceReadsTheModelsVocabulary(t *testing.T) {
	houseOnly := &configpb.VocabPatterns{Patterns: []string{`^FDBK$`}, Replace: true}
	added := &configpb.VocabPatterns{Patterns: []string{`^FDBK$`}}
	addedSwitch := &configpb.VocabPatterns{Patterns: []string{`^HSD$`}}
	for _, tc := range []struct {
		name      string
		fb, sw    string
		feedback  *configpb.VocabPatterns
		switching *configpb.VocabPatterns
		want      bool
	}{
		{"a built-in pin name with no convention", "FB", "SW", nil, nil, true},
		{"a house pin name with no convention", "FDBK", "SW", nil, nil, false},
		{"a house pin name the convention adds", "FDBK", "SW", added, nil, true},
		{"a built-in pin name the convention replaces", "FB", "SW", houseOnly, nil, false},
		{"a house switch pin with no convention", "FB", "HSD", nil, nil, false},
		{"a house switch pin the convention adds", "FB", "HSD", nil, addedSwitch, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := pinDesign(map[string][]supplyPin{
				"J1": twoPin(), "U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}},
				"U2": {{tc.sw, "1", pass}, {tc.fb, "2", pass}}, "L1": twoPin(),
			}, map[string][]string{
				"3V3": {"J1.1", "U1.1", "L1.2", "U2.2"}, "NODE": {"L1.1", "U2.1"}, "GND": {"U1.2", "J1.2"},
			})
			classify.Stamp(d)
			classify.StampNetRoles(d)
			rv, err := classify.BuildRoleVocab(&configpb.NamingLexicon{Pin: &configpb.PinNameVocab{Feedback: tc.feedback, Switching: tc.switching}})
			if err != nil {
				t.Fatalf("BuildRoleVocab: %v", err)
			}
			m := check.NewModel(d, check.WithLexicon(&classify.Lexicon{Role: rv}))
			var rail *ir.Net
			for _, n := range d.Nets {
				if n.Name == "3V3" {
					rail = n
				}
			}
			if rail == nil {
				t.Fatal("the fixture has no 3V3 net")
			}
			if got := check.BoardSource(m, rail); (got != "") != tc.want {
				t.Errorf("BoardSource = %q, want board driven = %v", got, tc.want)
			}
		})
	}
}
