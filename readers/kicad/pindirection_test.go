package kicad

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// A tri-state, open-collector or open-emitter pin keeps its symbol's declared type and does not read
// as untyped (agni issue 1082). A pin KiCad itself calls unspecified still reads as unspecified.
func TestKiCadOutputKindsKeepTheirTypes(t *testing.T) {
	for raw, want := range map[string]ir.PinDirection{
		"tri_state":      ir.PinDirection_PIN_DIRECTION_TRISTATE,
		"open_collector": ir.PinDirection_PIN_DIRECTION_OPEN_COLLECTOR,
		"open_emitter":   ir.PinDirection_PIN_DIRECTION_OPEN_EMITTER,
		"unspecified":    ir.PinDirection_PIN_DIRECTION_UNSPECIFIED,
		"output":         ir.PinDirection_PIN_DIRECTION_OUTPUT,
	} {
		if got := mapSchDirection(raw); got != want {
			t.Errorf("mapSchDirection(%q) = %v, want %v", raw, got, want)
		}
	}
}
