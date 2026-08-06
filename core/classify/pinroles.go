package classify

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// underspecifiedInputDir reports whether a direction is an input-ish value a reader falls back to when
// it cannot type a pin's electrical role. EDIF's port grammar carries only INPUT/OUTPUT/INOUT, so a
// supply pin reads as plain INPUT; these are the directions StampPowerInPins may promote to POWER_IN.
// OUTPUT/POWER_OUT/POWER_IN/PASSIVE/NO_CONNECT are confident classifications and are never touched.
func underspecifiedInputDir(d ir.PinDirection) bool {
	switch d {
	case ir.PinDirection_PIN_DIRECTION_INPUT,
		ir.PinDirection_PIN_DIRECTION_INOUT,
		ir.PinDirection_PIN_DIRECTION_UNSPECIFIED:
		return true
	}
	return false
}

// StampPowerInPins FILLS the POWER_IN electrical type on supply pins a reader left under-typed
// (WS3-072 PR2), once at ingestion. A source that classifies pin electrical type (KiCad, gEDA) already
// marks a VCC/VIN pin POWER_IN, so this is a no-op there; a source that does NOT (EDIF's port grammar
// carries only INPUT/OUTPUT/INOUT) leaves a VDD pin as plain INPUT, and this promotes it to POWER_IN
// where the direction is under-specified AND the pin name is a supply name. Then every power-pin rule
// works format-neutrally on a plain PinDir == POWER_IN check and the WS3-036 supplyInputPin interim is
// removed.
//
// This is the FILL variant of the C9 DERIVED-NORMALIZATION carve-out: unlike device_classes / net.roles
// (new fields no reader populates), it normalizes an EXISTING reader-set field to a more specific value
// where the reader was under-specified — and only there, so a confident OUTPUT/POWER_OUT is never
// overwritten. It degrades safely when the pass did not run (the reader's INPUT stands; the power-pin
// rules simply do not fire, absent-tolerant), meeting carve-out condition (c). Mutates the shared
// part-type pins (a pin name is a part-type property, so a promotion is consistent across every
// instance of the part). Idempotent.
// It promotes from the PROCESS-level lexicon; a read carrying its own conventions calls
// (*Lexicon).StampPowerInPins instead (WS3-106).
func StampPowerInPins(d *ir.Design) { ActiveLexicon().StampPowerInPins(d) }
