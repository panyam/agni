package classify

import (
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// underspecifiedInputDir reports whether d is a fallback direction. A reader falls back to one when
// it cannot type a pin's electrical role, and StampPowerInPins may promote such a pin to POWER_IN.
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

// formatTypesPowerIn reports whether a source format can type a pin as a power input. In such a
// format an INPUT pin is the author's choice and not a fallback. KiCad, gEDA and Telesis can; EDIF,
// IPC-2581 and xschem cannot, and neither can any format missing from this list. On a KiCad board an
// INA233's Vin+ and Vbus are drawn as inputs because they are sense pins, and promoting them by name
// read each as an undriven rail (agni issue 1241).
func formatTypesPowerIn(sourceFormat string) bool {
	return strings.HasPrefix(sourceFormat, "kicad") || sourceFormat == "geda" || sourceFormat == "telesis"
}

// formatTypesNoPins reports whether a source format stores no electrical type for any pin. A board
// file such as an Allegro .brd names its pins and types none of them. Promoting its loads to
// POWER_IN by name leaves every regulator output untyped, so every rail would read as undriven
// (agni issue 1361). Such a format is left alone until its own power and ground pin flags are read.
func formatTypesNoPins(sourceFormat string) bool {
	return sourceFormat == "allegro"
}

// StampPowerInPins FILLS the POWER_IN electrical type on under-typed supply pins (WS3-072 PR2), once
// at ingestion. KiCad and gEDA already mark a VCC/VIN pin POWER_IN, so it is a no-op there, and an
// INPUT pin in those formats is left INPUT (formatTypesPowerIn). EDIF's port grammar has only
// INPUT/OUTPUT/INOUT, so a VDD pin reads as plain INPUT. This pass promotes such a pin when its
// direction is under-specified AND its name is a supply name. Every power-pin rule then checks
// PinDir == POWER_IN whatever the format.
//
// This is the FILL variant of the C9 DERIVED-NORMALIZATION carve-out. It narrows an EXISTING
// reader-set field only where the reader was under-specified, so a confident OUTPUT/POWER_OUT is never
// overwritten. When the pass did not run, the reader's INPUT stands and the power-pin rules do not
// fire. That outcome meets carve-out condition (c). It mutates the shared part-type pins, so a
// promotion applies to every instance of the part. It is idempotent.
//
// This is the process-level form; a read carrying its own conventions calls
// (*Lexicon).StampPowerInPins instead (WS3-106).
func StampPowerInPins(d *ir.Design) { ActiveLexicon().StampPowerInPins(d) }
