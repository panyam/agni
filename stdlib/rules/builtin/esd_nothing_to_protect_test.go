package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// A connector net that reaches only passives and rails holds nothing a discharge could damage
// (agni issue 1913). SHLD is a USB-C shell tied to GND through C1 and R1, and RS0 is an SFP rate
// select strapped to +3V3 through R2. Both read not-considered. SIG reaches U1 through the series
// resistor R3, MOD passes through to J2, a module socket the design declares internal, and LANE
// reaches J2 through the AC-coupling capacitor C2, so all three are still judged and fail.
func TestESDLeavesANetWithNothingToProtectUnjudged(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"J1": {{"SH", "1", ir.PinDirection_PIN_DIRECTION_PASSIVE}, {"RS0", "2", ir.PinDirection_PIN_DIRECTION_PASSIVE},
			{"SIG", "3", ir.PinDirection_PIN_DIRECTION_PASSIVE}, {"MOD", "4", ir.PinDirection_PIN_DIRECTION_PASSIVE},
			{"LANE", "5", ir.PinDirection_PIN_DIRECTION_PASSIVE}},
		"J2": twoPin(), "C2": twoPin(), "C1": twoPin(), "R1": twoPin(), "R2": twoPin(), "R3": twoPin(),
		"U1": {{"IN", "1", ir.PinDirection_PIN_DIRECTION_INPUT}},
	}, map[string][]string{
		"SHLD":   {"J1.1", "C1.1", "R1.1"},
		"GND":    {"C1.2", "R1.2"},
		"RS0":    {"J1.2", "R2.1"},
		"+3V3":   {"R2.2"},
		"SIG":    {"J1.3", "R3.1"},
		"SIG_IN": {"R3.2", "U1.1"},
		"MOD":    {"J1.4", "J2.1"},
		"LANE":   {"J1.5", "C2.1"},
		"LANE_M": {"C2.2", "J2.2"},
	})
	for _, n := range d.Nets {
		if n.Name == "+3V3" || n.Name == "GND" {
			n.Attributes = map[string]string{netgraph.AttrGlobal: "true"}
		}
	}
	m := check.NewModel(d, check.WithIntent(internalIntent("J2")))
	got := map[string]check.Verdict{}
	for _, v := range esdProtection.Eval(context.Background(), m) {
		got[v.Subjects[0].Ref] = v
	}
	for _, net := range []string{"SHLD", "RS0"} {
		if v := got[net]; v.Outcome != check.NotConsidered || v.Reason != check.NothingToProtectReason {
			t.Errorf("%s = %s, %q, want not-considered with nothing to protect", net, v.Outcome, v.Reason)
		}
	}
	for _, net := range []string{"SIG", "MOD", "LANE"} {
		if v := got[net]; v.Outcome != check.Fail {
			t.Errorf("%s = %s, %q, want fail", net, v.Outcome, v.Reason)
		}
	}
	fired := map[string]bool{}
	for _, f := range esdProtectionSpec.Eval(context.Background(), m) {
		fired[check.EntityRef(f.Subject)] = true
	}
	if len(fired) != 3 || !fired["SIG"] || !fired["MOD"] || !fired["LANE"] {
		t.Errorf("the twin fires on %v, want SIG, MOD and LANE", fired)
	}
}
