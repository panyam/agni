package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// railFromConnector is a connector J1 whose pin sits on 3V3 beside U1's supply pin, plus whatever
// else the case wires onto the board. U2 is the converter, with a switch pin, a feedback pin and a
// power-output pin each landing wherever the case's nets put them.
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
