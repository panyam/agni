package classify

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// underspecifiedInputDir reports whether d is a direction a reader falls back to when it cannot type
// a pin's electrical role, which makes it one StampPowerInPins may promote to POWER_IN.
// OUTPUT/POWER_OUT/POWER_IN/PASSIVE/NO_CONNECT are confident classifications and are never touched.
// INPUT and INOUT count only where the format has no power-input type to choose instead (see
// formatTypesPowerIn).
func underspecifiedInputDir(d ir.PinDirection, typesPowerIn bool) bool {
	switch d {
	case ir.PinDirection_PIN_DIRECTION_UNSPECIFIED:
		return true
	case ir.PinDirection_PIN_DIRECTION_INPUT, ir.PinDirection_PIN_DIRECTION_INOUT:
		return !typesPowerIn
	}
	return false
}

// formatTypesPowerIn reports whether a source format can type a pin as a power input, so that a pin
// it typed INPUT is the author's choice and not a fallback. KiCad, gEDA and Telesis can; EDIF,
// IPC-2581 and xschem cannot, and neither can a format this list does not know. On a KiCad board an
// INA233's Vin+ and Vbus are drawn as inputs because they are sense pins, and promoting them by name
// read each as an undriven rail (agni issue 1241).
func formatTypesPowerIn(sourceFormat string) bool {
	return strings.HasPrefix(sourceFormat, "kicad") || sourceFormat == "geda" || sourceFormat == "telesis"
}

// StampPowerInPins FILLS the POWER_IN electrical type on supply pins a reader left under-typed
// (WS3-072 PR2), once at ingestion. KiCad and gEDA already mark a VCC/VIN pin POWER_IN, so it is a
// no-op there, and a pin those formats typed INPUT stays INPUT (formatTypesPowerIn). EDIF's port
// grammar carries only INPUT/OUTPUT/INOUT, so a VDD pin reads as plain INPUT, and this promotes it
// where the direction is under-specified AND the pin name is a supply name. Every
// power-pin rule then checks PinDir == POWER_IN whatever the format.
//
// This is the FILL variant of the C9 DERIVED-NORMALIZATION carve-out. It narrows an EXISTING
// reader-set field only where the reader was under-specified, so a confident OUTPUT/POWER_OUT is never
// overwritten. When the pass did not run, the reader's INPUT stands and the power-pin rules do not
// fire, which meets carve-out condition (c). It mutates the shared part-type pins, so a promotion holds
// for every instance of the part. Idempotent.
//
// This is the process-level form; a read carrying its own conventions calls
// (*Lexicon).StampPowerInPins instead (WS3-106).
func StampPowerInPins(d *ir.Design) { ActiveLexicon().StampPowerInPins(d) }
