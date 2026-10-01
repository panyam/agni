package classify

import (
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// underspecifiedInputDir reports whether d is an input-ish direction a reader falls back to when it
// cannot type a pin's electrical role, which makes it one StampPowerInPins may promote to POWER_IN.
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
// (WS3-072 PR2), once at ingestion. KiCad and gEDA already mark a VCC/VIN pin POWER_IN, so it is a
// no-op there. EDIF's port grammar carries only INPUT/OUTPUT/INOUT, so a VDD pin reads as plain INPUT,
// and this promotes it where the direction is under-specified AND the pin name is a supply name. Every
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
