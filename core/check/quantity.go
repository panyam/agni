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
// ok is false in three DIFFERENT situations that a rule must not distinguish but a REPORT should: the
// component carries no value attribute, it carries one the parser could not read, or it is not a part
// that has a value at all. For a rule the answer is the same either way, and it is the params-tier
// posture: skip, never guess. A rule that fired on a value it could not read would report a defect it
// has no evidence for.
//
// THE UNIT IS RETURNED, NOT ASSUMED, and a caller comparing against a threshold must check it. An empty
// unit means the number is known and the unit is not (a bare value on a class the conventions do not
// cover), which is a real state rather than an error. Treating an empty unit as "the one I wanted" is
// exactly the unlike-units coercion docs/20 forbids, so ComponentValueIn exists for the common case.
func ComponentValue(m Model, refDes string) (value float64, unit string, ok bool) {
	q := componentQuantity(m, refDes)
	if q == nil || q.Value == nil {
		return 0, "", false
	}
	return q.GetValue(), q.GetUnit(), true
}

// ComponentValueIn returns a component's value only when it is expressed in the unit the caller asked
// for, in that unit's SI base. It is the form a rule should reach for: a resistance check asks for ohms
// and gets nothing from a capacitor, rather than a farad count it would then compare against an ohm
// threshold.
//
// An EMPTY stored unit does not match anything, deliberately. A bare number whose unit the conventions
// could not supply is not evidence that it is in the unit you happen to want.
func ComponentValueIn(m Model, refDes, unit string) (float64, bool) {
	v, u, ok := ComponentValue(m, refDes)
	if !ok || u == "" || u != unit {
		return 0, false
	}
	return v, true
}

// ComponentValueText returns the SOURCE TEXT a component's value was read from, and whether it carried
// one at all. It is what a finding should quote: reporting "10k" tells a reviewer where to look on the
// schematic, and reporting 10000 makes them convert it back.
//
// It is present even when the parse FAILED, which is the point of keeping it: "this part states DNP and
// we could not read a number from it" is a reportable gap, and "this part states nothing" is not the
// same gap.
func ComponentValueText(m Model, refDes string) (string, bool) {
	q := componentQuantity(m, refDes)
	if q == nil {
		return "", false
	}
	return q.GetInput(), true
}

// componentQuantity resolves a component's stamped Quantity, or nil. It re-stamps nothing: a design
// built by hand in a test (no ingestion pass) simply has no values, the same fallback posture
// device_classes takes.
func componentQuantity(m Model, refDes string) *ir.Quantity {
	for _, c := range m.Components() {
		if c.GetRefDes() == refDes {
			return c.GetValue()
		}
	}
	return nil
}

// OhmsLawCurrent returns the current in AMPS that volts across ohms produces, and whether the inputs
// admit an answer. It is the first arithmetic anywhere in the engine that crosses two units (WS3-085),
// and it is deliberately a NAMED PHYSICAL OPERATION rather than a Quantity.Div.
//
// WHY NOT A GENERAL UNIT ALGEBRA. Every other consumer of a quantity compares within one unit, which
// the accessors already guarantee: OutputVoltageLimits returns rows param.InBaseUnit has reduced to
// volts, ComponentValueIn refuses a mismatched or empty unit. Scaling a prefix off a printed unit is
// not the same problem: it stays within one dimension, which is why one table settles it and this
// operation still cannot be expressed by one. A dimension system that could type-check
// volts/ohms -> amps in general is
// a large amount of machinery for six units and three operations, and it would have no second caller
// today. A named operation carries the same guarantee in its signature: the parameter names state
// which unit each side must already be in, so a caller passing farads is making a visible mistake
// rather than a silently typed one. It also reads to an EE, who knows what Ohm's law is and does not
// know what a dimension vector is. If a fourth or fifth physical relation shows up, that is the point
// to reconsider, not before.
//
// ok is false for a non-positive or non-finite resistance and for a non-finite voltage. Zero ohms is
// the case that matters: a sense resistor read as 0 is either a short or a value the parser could not
// place, and dividing by it yields +Inf, which every comparison downstream would read as "enormous
// current" and report as a defect. Refusing is the only honest answer. A NEGATIVE voltage is allowed
// (a low-side sense threshold is legitimately negative) and simply yields a negative current.
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
// inputs admit an answer. It is OhmsLawCurrent's sibling and exists for the same reason: the second
// physical relation this rule family needs, kept as a NAMED operation in the model layer rather than as
// an `i*i*r` written inside a rule. A rule that spells its own unit arithmetic is a rule where a unit
// bug can hide, and there is no unit in the expression to read it back from.
//
// It is the sizing half of a controller-based load switch. A switch's pass element is the thing that
// heats up, and what it dissipates at the current the design actually draws is the number that decides
// whether the FET was chosen large enough. That figure is REPORTED, never judged: turning it into a
// verdict needs a thermal limit (a package resistance, an ambient, a rise the house accepts) that no
// datasheet row and no declaration states today.
//
// ok is false for a non-finite current or resistance and for a negative resistance. A NEGATIVE current
// is allowed and yields the same positive power a positive one does, since a current dissipates the
// same whichever way it flows. Zero is allowed on both sides: a zero-ohm link dissipates nothing, which
// is a true answer rather than an unanswerable one, and that is where it differs from OhmsLawCurrent
// (which must refuse a zero divisor).
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
// each other, so this is not for them; it is for the boundary where an exactly-parsed 4700 meets a
// double that arrived by another route entirely.
//
// 1e-9 is far tighter than any real component tolerance (1% is the precision grade) and far looser than
// double rounding, so it separates "the same number written twice" from "actually different" without
// ever masking a real difference.
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
