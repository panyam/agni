package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// Two rail-named nets that are not rails, beside two that are (agni issue 1914). 12V_VDD is U1's
// own bias supply, its V_{DD} pin typed passive with only C1 on the net. VIN_PWR_ON drives Q1's
// gate, an input, through R1. +5V_LOAD has U2's power_in VDD pin and C2, the forgotten wire the
// supply rules catch, and VIN_CTL mixes a gate with U3's power_in pin. test-point-coverage judges
// only the last two.
func TestTestPointCoverageSkipsANetThatIsNotARail(t *testing.T) {
	in := ir.PinDirection_PIN_DIRECTION_INPUT
	d := pinDesign(map[string][]supplyPin{
		"U1": {{"V_{DD}", "1", pass}}, "U2": {{"VDD", "1", pwrIn}}, "U3": {{"VDD", "1", pwrIn}},
		"Q1": {{"G", "1", in}}, "Q2": {{"G", "1", in}},
		"C1": twoPin(), "C2": twoPin(), "R1": twoPin(), "TP1": twoPin(),
	}, map[string][]string{
		"12V_VDD":    {"U1.1", "C1.1"},
		"VIN_PWR_ON": {"Q1.1", "R1.1"},
		"+5V_LOAD":   {"U2.1", "C2.1"},
		"VIN_CTL":    {"Q2.1", "U3.1"},
		"GND":        {"C1.2", "C2.2", "TP1.1"},
	})
	fired := map[string]bool{}
	for _, f := range testPointCoverage.Findings(context.Background(), check.NewModel(d)) {
		fired[check.EntityRef(f.Subject)] = true
	}
	for net, want := range map[string]bool{"12V_VDD": false, "VIN_PWR_ON": false, "+5V_LOAD": true, "VIN_CTL": true} {
		if fired[net] != want {
			t.Errorf("test-point-coverage on %s fired = %v, want %v", net, fired[net], want)
		}
	}
}

// A flagged rail whose only load sits across a connector the design declares internal holds up
// nothing this board builds, so bulk-cap leaves it unjudged and names the connector (agni issue
// 1914). Jetson's backup-battery feed to the module across J1 is the case. The same net to an
// exposed connector, or with an IC's power_in pin on it, is still judged.
func TestBulkCapLeavesAModuleOnlyRailUnjudged(t *testing.T) {
	design := func(extra ...string) *ir.Design {
		d := pinDesign(map[string][]supplyPin{
			"J1": twoPin(), "R1": twoPin(), "D1": twoPin(), "U1": {{"VBAT", "1", pwrIn}},
		}, map[string][]string{"BBATT": append([]string{"J1.1", "R1.1", "D1.1"}, extra...)})
		for _, n := range d.Nets {
			n.Attributes = map[string]string{netgraph.AttrPowerDriven: "true"}
		}
		return d
	}
	eval := func(d *ir.Design, internal bool) check.Verdict {
		var opts []check.ModelOption
		if internal {
			opts = append(opts, check.WithIntent(internalIntent("J1")))
		}
		for _, v := range bulkCap.Eval(context.Background(), check.NewModel(d, opts...)) {
			if v.Subjects[0].Ref == "BBATT" {
				return v
			}
		}
		t.Fatalf("bulk-cap gave BBATT no verdict")
		return check.Verdict{}
	}
	if v := eval(design(), true); v.Outcome != check.NotConsidered || !strings.Contains(v.Reason, "feeds only J1") {
		t.Errorf("bulk-cap on a rail feeding only the internal J1 = %s, %q, want not-considered naming J1", v.Outcome, v.Reason)
	}
	if fs := bulkCapSpec.Eval(context.Background(), check.NewModel(design(), check.WithIntent(internalIntent("J1")))); len(fs) != 0 {
		t.Errorf("the bulk-cap twin fires %d times on a rail feeding only the internal J1, want none", len(fs))
	}
	if v := eval(design(), false); v.Outcome != check.Fail {
		t.Errorf("bulk-cap with J1 exposed = %s, want fail", v.Outcome)
	}
	if v := eval(design("U1.1"), true); v.Outcome != check.Fail {
		t.Errorf("bulk-cap with an IC load beside J1 = %s, want fail", v.Outcome)
	}
}
