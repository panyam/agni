package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pullUpDesign is an I2C clock line with a pull-up to +1V8 through R1 and one to +3V3 through R2,
// with the named resistors marked do-not-populate.
func pullUpDesign(dnp ...string) *ir.Design {
	d := pinDesign(map[string][]supplyPin{
		"R1": twoPin(), "R2": twoPin(),
		"U1": {{"SCL", "1", ir.PinDirection_PIN_DIRECTION_INOUT}},
		"U2": {{"OUT", "1", pwrOut}}, "U3": {{"OUT", "1", pwrOut}},
	}, map[string][]string{
		"I2C_SCL": {"U1.1", "R1.1", "R2.1"},
		"+1V8":    {"R1.2", "U2.1"},
		"+3V3":    {"R2.2", "U3.1"},
	})
	for _, c := range d.Components {
		for _, ref := range dnp {
			if c.RefDes == ref {
				c.Attributes = map[string]string{"dnp": "yes"}
			}
		}
	}
	return d
}

// A do-not-populate pull-up is an assembly option, not a second pull-up (agni issue 938). Jetson's
// camera lines carry a fitted pull-up to 1.8 V beside a DNP one to 3.3 V.
func TestADoNotPopulatePullUpPullsNothing(t *testing.T) {
	if got := verdictOn(t, i2cPullUpSplitRail, pullUpDesign("R2"), "I2C_SCL"); got == string(check.Fail) {
		t.Errorf("split-rail with the 3.3 V pull-up unfitted = %q, want it not to fail", got)
	}
	if got := verdictOn(t, i2cPullUpSplitRail, pullUpDesign(), "I2C_SCL"); got != string(check.Fail) {
		t.Errorf("split-rail with both pull-ups fitted = %q, want fail", got)
	}
	// With every pull-up unfitted the line has none, which is the finding i2c-pull-up exists for.
	if got := verdictOn(t, i2cPullUp, pullUpDesign("R1", "R2"), "I2C_SCL"); got != string(check.Fail) {
		t.Errorf("i2c-pull-up with every pull-up unfitted = %q, want fail", got)
	}
	if got := verdictOn(t, i2cPullUp, pullUpDesign("R2"), "I2C_SCL"); got != string(check.Pass) {
		t.Errorf("i2c-pull-up with one pull-up fitted = %q, want pass", got)
	}
}

// A capacitor marked do-not-populate decouples nothing, so a rail whose only capacitor is unfitted
// has none (agni issue 961). The spec twin is asked the same question.
func TestADoNotPopulateCapacitorDecouplesNothing(t *testing.T) {
	design := func(dnp bool) *ir.Design {
		d := pinDesign(map[string][]supplyPin{
			"U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}}, "C1": twoPin(),
		}, map[string][]string{"VDD_CORE": {"U1.1", "C1.1"}, "GND": {"U1.2", "C1.2"}})
		if dnp {
			for _, c := range d.Components {
				if c.RefDes == "C1" {
					c.Attributes = map[string]string{"dnp": "yes"}
				}
			}
		}
		return d
	}
	if got := verdictOn(t, decouplingPresent, design(true), "VDD_CORE"); got != string(check.Fail) {
		t.Errorf("decoupling-present with its only capacitor unfitted = %q, want fail", got)
	}
	if got := verdictOn(t, decouplingPresent, design(false), "VDD_CORE"); got != string(check.Pass) {
		t.Errorf("decoupling-present with the capacitor fitted = %q, want pass", got)
	}
	twin := func(d *ir.Design) int {
		return len(decouplingPresentSpec.Eval(context.Background(), check.NewModel(d)))
	}
	if twin(design(true)) != 1 || twin(design(false)) != 0 {
		t.Errorf("decoupling-present twin findings = %d unfitted, %d fitted, want 1 and 0", twin(design(true)), twin(design(false)))
	}
}

// A test point marked do-not-populate still covers its rail, because the probe lands on the pad,
// which is copper on the bare board. The RoyalBlue54L Feather marks every one of its pad test points
// DNP for that reason (agni issue 961).
func TestADoNotPopulateTestPointStillCovers(t *testing.T) {
	comp := func(ref string, dnp bool) *ir.Component {
		c := &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
		if dnp {
			c.Attributes = map[string]string{"dnp": "yes"}
		}
		return c
	}
	fired := func(d *ir.Design) []string {
		var out []string
		for _, f := range testPointCoverage.Findings(context.Background(), check.NewModel(d)) {
			out = append(out, check.EntityRef(f.Subject))
		}
		return out
	}
	covered := &ir.Design{
		Components: []*ir.Component{comp("TP1", true), comp("TP2", true), comp("U1", false)},
		Nets:       []*ir.Net{tnet("GND", "U1.1", "TP1.1"), tnet("VCC", "U1.2", "TP2.1")},
	}
	if got := fired(covered); len(got) != 0 {
		t.Errorf("rails covered by unfitted pad test points fired on %v, want none", got)
	}
	uncovered := &ir.Design{
		Components: []*ir.Component{comp("TP1", true), comp("U1", false)},
		Nets:       []*ir.Net{tnet("GND", "U1.1", "TP1.1"), tnet("VCC", "U1.2")},
	}
	if got := fired(uncovered); len(got) != 1 || got[0] != "VCC" {
		t.Errorf("a board whose only test point is unfitted still declares the convention; fired on %v, want exactly VCC", got)
	}
}

// markDNP marks the named parts of d do-not-populate.
func markDNP(d *ir.Design, refs ...string) *ir.Design {
	for _, c := range d.Components {
		for _, ref := range refs {
			if c.RefDes == ref {
				if c.Attributes == nil {
					c.Attributes = map[string]string{}
				}
				c.Attributes["dnp"] = "yes"
			}
		}
	}
	return d
}

// A circuit left unfitted as a whole is never built, so a rule that no longer credits its unfitted
// capacitor or clamp must not judge it either (agni issue 961). Jetson's Tag-Connect JTAG translator
// (J20, U59, R341, R342, C237) is one, and crediting alone failed its rail and its reset line.
func TestAnUnfittedCircuitIsNotJudged(t *testing.T) {
	rail := func() *ir.Design {
		d := pinDesign(map[string][]supplyPin{
			"U1": {{"VCC", "1", pwrIn}, {"GND", "2", pwrIn}}, "C1": twoPin(),
		}, map[string][]string{"TC_VCC": {"U1.1", "C1.1"}, "GND": {"U1.2", "C1.2"}})
		for _, n := range d.Nets {
			if n.Name == "TC_VCC" {
				n.Attributes = map[string]string{"power_driven": "true"}
			}
		}
		return d
	}
	if got := verdictOn(t, decouplingPresent, markDNP(rail(), "U1", "C1"), "TC_VCC"); got != "" {
		t.Errorf("decoupling-present on a rail whose only load is unfitted = %q, want no verdict", got)
	}
	if got := verdictOn(t, decouplingPresent, markDNP(rail(), "C1"), "TC_VCC"); got != string(check.Fail) {
		t.Errorf("decoupling-present on a fitted load with its capacitor unfitted = %q, want fail", got)
	}
	if got := verdictOn(t, bulkCap, markDNP(rail(), "U1", "C1"), "TC_VCC"); got != string(check.NotConsidered) {
		t.Errorf("bulk-cap on a rail whose every part is unfitted = %q, want not-considered", got)
	}
	if got := verdictOn(t, bulkCap, markDNP(rail(), "C1"), "TC_VCC"); got != string(check.Fail) {
		t.Errorf("bulk-cap on a fitted rail with its capacitor unfitted = %q, want fail", got)
	}

	line := func() *ir.Design {
		return pinDesign(map[string][]supplyPin{
			"J1": twoPin(), "R1": twoPin(), "U1": {{"TRST", "1", ir.PinDirection_PIN_DIRECTION_INPUT}},
		}, map[string][]string{"TC_TRST": {"J1.1", "R1.1"}, "TRST": {"R1.2", "U1.1"}})
	}
	if got := verdictOn(t, esdProtection, markDNP(line(), "J1", "R1"), "TC_TRST"); got != string(check.NotConsidered) {
		t.Errorf("esd-protection on a connector net whose every part is unfitted = %q, want not-considered", got)
	}
	if got := verdictOn(t, esdProtection, line(), "TC_TRST"); got != string(check.Fail) {
		t.Errorf("esd-protection on the fitted line = %q, want fail", got)
	}
}
