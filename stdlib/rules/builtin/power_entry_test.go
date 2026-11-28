package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// What input-protection counts as a power entry on a connector net (agni issue 1939). The connector's
// own pin and a clamp's supply pin are never the load. A connector pin typed as power with no load
// behind it is not-considered with a reason, and an untyped one is a signal with no verdict. A supply pin the symbol typed inout or passive
// still is, read from its name. A sense pin the symbol typed input is not, as on an INA233 (agni
// issue 1241).
func TestInputProtectionCountsTheRealLoad(t *testing.T) {
	inout, input := ir.PinDirection_PIN_DIRECTION_INOUT, ir.PinDirection_PIN_DIRECTION_INPUT
	for _, tc := range []struct {
		name  string
		jdir  ir.PinDirection
		part  []supplyPin
		mpn   string
		want  check.Outcome
		entry string
	}{
		{"only the connector's own power_in pin", pwrIn, nil, "", check.NotConsidered, ""},
		{"a TVS array's supply pin on a connector power pin", pwrIn, []supplyPin{{"VCC", "1", pwrIn}}, "TPD4E05U06", check.NotConsidered, ""},
		{"a TVS array's supply pin on a passive connector pin", pass, []supplyPin{{"VCC", "1", pwrIn}}, "TPD4E05U06", "", ""},
		{"a PD controller's VBUS1 typed inout", pass, []supplyPin{{"VBUS1", "1", inout}}, "", check.Fail, "U5's VBUS1 on PWR"},
		{"an LDO's VIN typed passive", pass, []supplyPin{{"VIN", "1", pass}}, "", check.Fail, "U5's VIN on PWR"},
		{"a load typed power_in beside the connector's own", pwrIn, []supplyPin{{"VDD", "1", pwrIn}}, "", check.Fail, "U5's VDD on PWR"},
		{"a current monitor's Vbus sense pin typed input", pass, []supplyPin{{"Vbus", "1", input}}, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parts := map[string][]supplyPin{"J1": {{"VBUS", "1", tc.jdir}, {"GND", "2", pass}}}
			nets := map[string][]string{"PWR": {"J1.1"}, "GND": {"J1.2"}}
			if tc.part != nil {
				parts["U5"] = tc.part
				nets["PWR"] = append(nets["PWR"], "U5.1")
			}
			d := pinDesign(parts, nets)
			for _, c := range d.Components {
				if c.RefDes == "U5" && tc.mpn != "" {
					c.Attributes = map[string]string{"MPN": tc.mpn}
				}
			}
			classify.Stamp(d)
			classify.StampNetRoles(d)
			m := check.NewModel(d)
			if tc.mpn != "" && !m.HasClass("U5", check.ClassTVS) {
				t.Fatalf("U5 with MPN %s does not read as a TVS", tc.mpn)
			}
			var got *check.Verdict
			for _, v := range inputProtection.Eval(context.Background(), m) {
				if v.Subjects[0].Ref == "PWR" {
					got = &v
				}
			}
			if tc.want == "" {
				if got != nil {
					t.Fatalf("input-protection judged PWR as %s, want no verdict: %+v", got.Outcome, got.Witness)
				}
				return
			}
			if got == nil || got.Outcome != tc.want {
				t.Fatalf("input-protection on PWR = %v, want %s", got, tc.want)
			}
			if tc.want == check.NotConsidered {
				if !strings.Contains(got.Reason, "reaches no load") {
					t.Errorf("reason = %q, want it to say the power pin reaches no load", got.Reason)
				}
				return
			}
			entry := ""
			for _, term := range got.Witness.Terms {
				if term.Label == "power entry" {
					entry = term.Value
				}
			}
			if entry != tc.entry {
				t.Errorf("witness power entry = %q, want %q", entry, tc.entry)
			}
		})
	}
}
