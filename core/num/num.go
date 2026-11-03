// Package num narrows integers for the wire, saturating rather than wrapping.
//
// agni mostly narrows counts and coordinates read off a design file. A design file can come from
// anyone, including a file dropped into the browser demo. A plain int32(n) of a value past the
// range wraps to a number with the wrong sign, and that number reads as a real answer. Saturating
// holds the value at the end of the range it overflowed, so it is still the largest or smallest
// value the field can hold.
package num

import "math"

// Int32 narrows v to an int32, holding it at math.MaxInt32 or math.MinInt32 when it is out of range.
func Int32[T ~int | ~int64](v T) int32 {
	switch {
	case int64(v) > math.MaxInt32:
		return math.MaxInt32
	case int64(v) < math.MinInt32:
		return math.MinInt32
	}
	return int32(v)
}

// Int64 converts an unsigned count such as a runtime.MemStats field to an int64, holding it at
// math.MaxInt64 when it is out of range.
func Int64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

// Uint32 converts a count such as a slice length to a uint32, holding it at 0 or math.MaxUint32 when
// it is out of range.
func Uint32[T ~int | ~int64](v T) uint32 {
	switch {
	case int64(v) < 0:
		return 0
	case int64(v) > math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(v)
}
