package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// powerVerdict returns rule's verdict on net in d, or nil when the rule gives none.
func powerVerdict(t *testing.T, rule *check.Rule, d *ir.Design, net string) *check.Verdict {
	t.Helper()
	classify.Stamp(d)
	classify.StampNetRoles(d)
	for _, v := range rule.Eval(context.Background(), check.NewModel(d)) {
		if v.Subjects[0].Ref == net {
			return &v
		}
	}
	return nil
}

// input-protection follows a connector's power through a load switch to the load behind it (agni
// issue 1941). HackRF One's VBUS reaches its regulators through P-FET Q5, and the walk stopped there.
func TestInputProtectionCrossesALoadSwitch(t *testing.T) {
	en := ir.PinDirection_PIN_DIRECTION_INPUT
	for _, tc := range []struct {
		name  string
		fet   map[string]string // FET pin name -> net
		fuse  bool
		want  check.Outcome
		entry string
	}{
		{"source on the connector, drain to the load", map[string]string{"S": "PWR", "D": "VSYS", "G": "EN"}, false, check.Fail, "U2's VIN on VSYS"},
		{"drain on the connector, source to the load", map[string]string{"D": "PWR", "S": "VSYS", "G": "EN"}, false, check.Fail, "U2's VIN on VSYS"},
		{"a fuse before the switch", map[string]string{"S": "FUSED", "D": "VSYS", "G": "EN"}, true, check.Pass, "U2's VIN on VSYS"},
		{"only the gate on the connector", map[string]string{"G": "PWR", "S": "VSYS", "D": "OUT"}, false, check.NotConsidered, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := map[string][]supplyPin{
				"J1": {{"VIN", "1", pwrIn}},
				"U2": {{"VIN", "1", pwrIn}},
				"Q1": {{"G", "1", en}, {"S", "2", pass}, {"D", "3", pass}},
			}
			nets := map[string][]string{"PWR": {"J1.1"}, "VSYS": {"U2.1"}}
			pinOf := map[string]string{"G": "Q1.1", "S": "Q1.2", "D": "Q1.3"}
			for pin, net := range tc.fet {
				nets[net] = append(nets[net], pinOf[pin])
			}
			if tc.fuse {
				parts["F1"] = []supplyPin{{"1", "1", pass}, {"2", "2", pass}}
				nets["PWR"] = append(nets["PWR"], "F1.1")
				nets["FUSED"] = append(nets["FUSED"], "F1.2")
			}
			got := powerVerdict(t, inputProtection, pinDesign(parts, nets), "PWR")
			if got == nil {
				t.Fatalf("input-protection gave no verdict on PWR, want %s", tc.want)
			}
			if got.Outcome != tc.want {
				t.Fatalf("input-protection on PWR = %s (%s), want %s", got.Outcome, got.Reason, tc.want)
			}
			if tc.entry != "" && witnessTerm(got, "power entry") != tc.entry {
				t.Errorf("power entry = %q, want %q", witnessTerm(got, "power entry"), tc.entry)
			}
		})
	}
}

// reverse-blocking-absent reads its loads through check.LoadPin, as input-protection does, and does
// not judge a USB bus supply, whose plug cannot mate reversed (agni issue 1941).
func TestReverseBlockingReadsTheSameLoads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		jpin   string
		load   supplyPin
		want   check.Outcome
		reason string
	}{
		{"an LDO's VIN typed passive", "VIN", supplyPin{"VIN", "1", pass}, check.Fail, ""},
		{"a PD controller's VBUS1 typed inout", "VIN", supplyPin{"VBUS1", "1", ir.PinDirection_PIN_DIRECTION_INOUT}, check.Fail, ""},
		{"a load typed power_in", "VIN", supplyPin{"VDD", "1", pwrIn}, check.Fail, ""},
		{"a USB receptacle's VBUS", "VBUS", supplyPin{"VIN", "1", pwrIn}, check.NotConsidered, "J1's VBUS is a USB bus supply"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := pinDesign(map[string][]supplyPin{
				"J1": {{tc.jpin, "1", pwrIn}},
				"U5": {tc.load},
			}, map[string][]string{"PWR": {"J1.1", "U5.1"}})
			got := powerVerdict(t, reverseBlockingAbsent, d, "PWR")
			if got == nil {
				t.Fatalf("reverse-blocking-absent gave no verdict on PWR, want %s", tc.want)
			}
			if got.Outcome != tc.want {
				t.Fatalf("reverse-blocking-absent on PWR = %s (%s), want %s", got.Outcome, got.Reason, tc.want)
			}
			if !strings.Contains(got.Reason, tc.reason) {
				t.Errorf("reason = %q, want it to contain %q", got.Reason, tc.reason)
			}
		})
	}
}

// witnessTerm returns the value of v's witness term with the given label, or "".
func witnessTerm(v *check.Verdict, label string) string {
	if v.Witness == nil {
		return ""
	}
	for _, term := range v.Witness.Terms {
		if term.Label == label {
			return term.Value
		}
	}
	return ""
}
