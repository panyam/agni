package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestTestPointCoverageSkipsRegulatorInternals: the rule asks which RAILS cannot be probed, and a
// regulator's own plumbing is not a rail. The feedback half has been excluded since WS3-067; the
// switch node was not, which on one real board produced 20 findings telling a factory test to probe
// the highest dV/dt node in the design (agni 680).
func TestTestPointCoverageSkipsRegulatorInternals(t *testing.T) {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "REG", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_OUT}}},
			{Name: "MCU", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
		}}},
		Components: []*ir.Component{
			{RefDes: "REG1", Sections: []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U1", Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			// TP1 is what puts the design in scope at all: has_test_points is design-wide, so
			// without one the rule declines to run and every assertion below passes vacuously.
			{RefDes: "TP1", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			tnet("12V_OUT", "REG1.1", "U1.1"),     // a real rail with no test point -> fires
			tnet("12V_PROBED", "REG1.1", "TP1.1"), // a real rail carrying one -> quiet
			tnet("12V_FB", "REG1.1", "U1.1"),      // feedback tap, must not be probed -> quiet
			tnet("12V_SW", "REG1.1", "U1.1"),      // switch node, must not be probed -> quiet
			tnet("12V_BOOT", "REG1.1", "U1.1"),    // bootstrap node -> quiet
			tnet("12V_PHASE", "REG1.1", "U1.1"),   // phase node -> quiet
			// Not must-not-probe nets. Quiet because they are not rails, so the rule's message
			// would name the wrong subject (agni 680).
			tnet("12V_MODE1", "REG1.1", "U1.1"), // mode strap -> quiet
			tnet("20V_EN", "REG1.1", "U1.1"),    // enable input -> quiet
			tnet("12V_VDRV", "REG1.1", "U1.1"),  // gate-drive supply -> quiet
		},
	}

	fired := map[string]bool{}
	for _, f := range check.RunDesign(d) {
		if f.Rule == "test-point-coverage" {
			fired[check.EntityRef(f.Subject)] = true
		}
	}

	// The positive control. The rule must still be running and still finding the thing it is for,
	// or the exclusions below prove nothing.
	if !fired["12V_OUT"] {
		t.Error("12V_OUT (a rail with no test point) should be flagged; the rule is not running")
	}
	for _, quiet := range []string{"12V_PROBED", "12V_FB", "12V_SW", "12V_BOOT", "12V_PHASE", "12V_MODE1", "20V_EN", "12V_VDRV"} {
		if fired[quiet] {
			t.Errorf("%s must not be flagged: a regulator internal is not a rail to probe", quiet)
		}
	}
}
