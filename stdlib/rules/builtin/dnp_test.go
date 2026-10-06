package builtin

import (
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
