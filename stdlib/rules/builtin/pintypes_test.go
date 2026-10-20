package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

const (
	triState      = ir.PinDirection_PIN_DIRECTION_TRISTATE
	openCollector = ir.PinDirection_PIN_DIRECTION_OPEN_COLLECTOR
	output        = ir.PinDirection_PIN_DIRECTION_OUTPUT
	untyped       = ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
)

// A pin its symbol declares tri-state is typed, so it is no unspecified pin beside the driver it
// shares a net with (agni issue 1082). Jetson's TXU0304 OE on +1V8 was the case.
func TestATriStatePinIsNotUntyped(t *testing.T) {
	design := func(dir ir.PinDirection) *ir.Design {
		return pinDesign(map[string][]supplyPin{
			"U1": {{"OUT", "1", output}}, "U2": {{"OE", "8", dir}},
		}, map[string][]string{"EN": {"U1.1", "U2.8"}})
	}
	if got := verdictOn(t, unspecifiedPinWithDriver, design(triState), "EN"); got == string(check.Fail) {
		t.Errorf("unspecified-pin-with-driver beside a tri-state pin = %q, want no failure", got)
	}
	if got := verdictOn(t, unspecifiedPinWithDriver, design(untyped), "EN"); got != string(check.Fail) {
		t.Errorf("unspecified-pin-with-driver beside an untyped pin = %q, want fail", got)
	}
}

// Several tri-state or open-collector outputs on one net are a bus or a wire-OR, not two drivers
// fighting, so output-output-conflict passes them where it fails two push-pull outputs.
func TestSharedTriStateAndOpenCollectorOutputsDoNotConflict(t *testing.T) {
	design := func(a, b ir.PinDirection) *ir.Design {
		return pinDesign(map[string][]supplyPin{
			"U1": {{"Q", "1", a}}, "U2": {{"Q", "1", b}},
		}, map[string][]string{"BUS": {"U1.1", "U2.1"}})
	}
	for name, d := range map[string]*ir.Design{"tri-state": design(triState, triState), "open-collector": design(openCollector, openCollector)} {
		if got := verdictOn(t, outputOutputConflict, d, "BUS"); got == string(check.Fail) {
			t.Errorf("output-output-conflict on two %s outputs = %q, want no failure", name, got)
		}
	}
	if got := verdictOn(t, outputOutputConflict, design(output, output), "BUS"); got != string(check.Fail) {
		t.Errorf("output-output-conflict on two push-pull outputs = %q, want fail", got)
	}
}
