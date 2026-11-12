package geda

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// gEDA's tri, oc and oe pintypes keep their own types (agni issue 1082).
func TestGedaOutputKindsKeepTheirTypes(t *testing.T) {
	for raw, want := range map[string]ir.PinDirection{
		"tri": ir.PinDirection_PIN_DIRECTION_TRISTATE,
		"oc":  ir.PinDirection_PIN_DIRECTION_OPEN_COLLECTOR,
		"oe":  ir.PinDirection_PIN_DIRECTION_OPEN_EMITTER,
		"clk": ir.PinDirection_PIN_DIRECTION_UNSPECIFIED,
	} {
		if got := pinDir(raw); got != want {
			t.Errorf("pinDir(%q) = %v, want %v", raw, got, want)
		}
	}
}
