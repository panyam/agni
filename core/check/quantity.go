package check

import (
	"math"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// UnitOhm is the canonical ohm symbol a Quantity carries, re-exported so a rule comparing resistances
// does not import the classify package for one string.
const UnitOhm = classify.UnitOhm

// ComponentValue returns a component's value in its unit's SI BASE unit, the unit symbol, and whether a
// number is available (WS3-118).
//
// ok is false when the component has no value attribute, has one the parser could not read, or is not
// a part that has a value at all. A rule treats all three the same way and skips, never guesses; a
// REPORT may want to tell them apart (see ComponentValueText).
//
// THE UNIT IS RETURNED, NOT ASSUMED, and a caller comparing against a threshold must check it. An empty
// unit means the number is known and the unit is not (a bare value on a class the conventions do not
// cover). Treating it as the unit you wanted is the unlike-units coercion
// docsite/content/architecture/datasheet-layer.md#comparison-semantics forbids, so use ComponentValueIn
// for the common case.
func ComponentValue(m Model, refDes string) (value float64, unit string, ok bool) {
	q := componentQuantity(m, refDes)
	if q == nil || q.Value == nil {
		return 0, "", false
	}
	return q.GetValue(), q.GetUnit(), true
}

// ComponentValueIn returns a component's value only when it is expressed in the unit the caller asked
// for, in that unit's SI base. A rule should reach for this one, so a resistance check asking for ohms
// gets nothing from a capacitor instead of a farad count.
//
// An EMPTY stored unit matches nothing, since a bare number is not evidence of the unit you want.
func ComponentValueIn(m Model, refDes, unit string) (float64, bool) {
	v, u, ok := ComponentValue(m, refDes)
	if !ok || u == "" || u != unit {
		return 0, false
	}
	return v, true
}

// ComponentValueText returns the SOURCE TEXT a component's value was read from, and whether it carried
// one at all. A finding should quote this ("10k" rather than 10000) so a reviewer can find it on the
// schematic.
//
// It is present even when the parse FAILED, so a report can tell "states DNP, no number read" apart
// from "states nothing".
func ComponentValueText(m Model, refDes string) (string, bool) {
	q := componentQuantity(m, refDes)
	if q == nil {
		return "", false
	}
	return q.GetInput(), true
}

// componentQuantity resolves a component's stamped Quantity, or nil. It re-stamps nothing, so a design
// built by hand in a test (no ingestion pass) has no values, as with device_classes.
func componentQuantity(m Model, refDes string) *ir.Quantity {
	for _, c := range m.Components() {
		if c.GetRefDes() == refDes {
			return c.GetValue()
		}
	}
	return nil
}

// OhmsLawCurrent returns the current in AMPS that volts across ohms produces, and whether the inputs
// admit an answer. It is a NAMED PHYSICAL OPERATION rather than a Quantity.Div, the first arithmetic
// in the engine that crosses two units (WS3-085).
//
// No general unit algebra, because every other consumer compares within one unit, and a dimension
// system for six units and three operations would have no second caller. The parameter names state
// which unit each side must be in, so passing farads is a visible mistake. Reconsider if a fourth or
// fifth physical relation shows up.
//
// ok is false for a non-positive or non-finite resistance and for a non-finite voltage. Zero ohms
// matters most, since a sense resistor read as 0 (a short, or an unplaced value) would yield +Inf and
// read downstream as an enormous current. A NEGATIVE voltage is allowed (a low-side sense threshold is
// negative) and yields a negative current.
func OhmsLawCurrent(volts, ohms float64) (amps float64, ok bool) {
	if math.IsNaN(volts) || math.IsInf(volts, 0) {
		return 0, false
	}
	if !(ohms > 0) || math.IsInf(ohms, 0) {
		return 0, false
	}
	return volts / ohms, true
}

// ResistivePowerWatts returns the power in WATTS a current of amps dissipates in ohms, and whether the
// inputs admit an answer. Like OhmsLawCurrent, it is a NAMED operation so a rule does not spell its
// own `i*i*r`, where a unit bug could hide.
//
// It sizes a load switch's pass FET at the current the design draws. The figure is REPORTED, never
// judged, because a verdict needs a thermal limit (package resistance, ambient, accepted rise) that no
// datasheet row or declaration states today.
//
// ok is false for a non-finite current or resistance and for a negative resistance. A NEGATIVE current
// yields the same positive power. Zero is allowed on both sides, since a zero-ohm link dissipates
// nothing, unlike OhmsLawCurrent's zero divisor.
func ResistivePowerWatts(amps, ohms float64) (watts float64, ok bool) {
	if math.IsNaN(amps) || math.IsInf(amps, 0) {
		return 0, false
	}
	if math.IsNaN(ohms) || math.IsInf(ohms, 0) || ohms < 0 {
		return 0, false
	}
	return amps * amps * ohms, true
}

// valueEpsilon is the RELATIVE tolerance for comparing a component value against a number of different
// provenance (a datasheet parameter, a declared budget). Values this parser produces compare exactly to
// each other and do not need it.
//
// 1e-9 is far tighter than any real component tolerance (1% is the precision grade) and far looser than
// double rounding.
const valueEpsilon = 1e-9

// QuantityEqual reports whether two quantities in the same unit are the same number within
// valueEpsilon. Use it instead of == whenever either side did not come from ParseQuantity.
//
// Zero compares exactly, since a relative tolerance around zero is meaningless and zero is a legal
// resistance rather than a rounding artifact.
func QuantityEqual(a, b float64) bool {
	if a == b {
		return true
	}
	if a == 0 || b == 0 {
		return false
	}
	return math.Abs(a-b) <= valueEpsilon*math.Max(math.Abs(a), math.Abs(b))
}
