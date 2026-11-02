package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// openPinDesign is one part whose pin sits alone on net, flagged no-connect when flagged is set, as
// KiCad marks a pin under a no-connect X. wiredTo adds a second part's pin to the same net.
func openPinDesign(net string, dir ir.PinDirection, flagged bool, wiredTo ...string) *ir.Design {
	parts := map[string][]supplyPin{"U1": {{net, "1", dir}}}
	conns := []string{"U1.1"}
	for _, ref := range wiredTo {
		parts[ref] = []supplyPin{{"1", "1", ir.PinDirection_PIN_DIRECTION_INPUT}}
		conns = append(conns, ref+".1")
	}
	d := pinDesign(parts, map[string][]string{net: conns})
	if flagged {
		d.Nets[0].Attributes = map[string]string{netgraph.AttrNoConnect: "true"}
	}
	return d
}

// A pin the schematic flags no-connect is unused on purpose, so a rule that would demand a pull-up,
// a driver or a capacitor of it has nothing to judge. The MNT Reform's unused USB hub SMBus pins and
// Glasgow's unused FPGA VPP_FAST pin each failed until agni issue 1244.
func TestAPinFlaggedNoConnectIsNotJudged(t *testing.T) {
	for _, tc := range []struct {
		rule *check.Rule
		net  string
		dir  ir.PinDirection
	}{
		{i2cPullUp, "SDA", ir.PinDirection_PIN_DIRECTION_INOUT},
		{powerInputNotDriven, "VPP_FAST", pwrIn},
		{decouplingPresent, "VPP_FAST", pwrIn},
	} {
		if got := verdictOn(t, tc.rule, openPinDesign(tc.net, tc.dir, true), tc.net); got != string(check.NotConsidered) {
			t.Errorf("%s on a pin flagged no-connect = %q, want not-considered", tc.rule.Name, got)
		}
		if got := verdictOn(t, tc.rule, openPinDesign(tc.net, tc.dir, false), tc.net); got != string(check.Fail) {
			t.Errorf("%s on the same pin unflagged = %q, want fail", tc.rule.Name, got)
		}
	}
}

// A flag on a pin that is also wired to another part is a design error rather than a statement that
// the pin is unused, so the net is still judged.
func TestAFlaggedPinWiredToAnotherPartIsStillJudged(t *testing.T) {
	for _, tc := range []struct {
		rule *check.Rule
		net  string
		dir  ir.PinDirection
	}{
		{i2cPullUp, "SDA", ir.PinDirection_PIN_DIRECTION_INOUT},
		{powerInputNotDriven, "VPP_FAST", pwrIn},
		{decouplingPresent, "VPP_FAST", pwrIn},
	} {
		if got := verdictOn(t, tc.rule, openPinDesign(tc.net, tc.dir, true, "U2"), tc.net); got != string(check.Fail) {
			t.Errorf("%s on a flagged pin wired to U2 = %q, want fail", tc.rule.Name, got)
		}
	}
}

// Each rule's declarative twin leaves a flagged pin alone and still fires on an unflagged one.
func TestTheTwinsLeaveAFlaggedPinAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec *check.Spec
		net  string
		dir  ir.PinDirection
	}{
		{"i2c-pull-up", i2cPullUpSpec, "SDA", ir.PinDirection_PIN_DIRECTION_INOUT},
		{"power-input-not-driven", powerInputNotDrivenSpec, "VPP_FAST", pwrIn},
		{"decoupling-present", decouplingPresentSpec, "VPP_FAST", pwrIn},
	} {
		findings := func(flagged bool) int {
			return len(tc.spec.Eval(context.Background(), check.NewModel(openPinDesign(tc.net, tc.dir, flagged))))
		}
		if f, u := findings(true), findings(false); f != 0 || u != 1 {
			t.Errorf("%s twin findings = %d flagged, %d unflagged, want 0 and 1", tc.name, f, u)
		}
	}
}
