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
