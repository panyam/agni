package num

import (
	"math"
	"testing"
)

func TestNarrowingSaturatesAtTheEndOfTheRange(t *testing.T) {
	if got := Int32(int64(math.MaxInt32) + 1); got != math.MaxInt32 {
		t.Errorf("Int32(MaxInt32+1) = %d, want MaxInt32", got)
	}
	if got := Int32(int64(math.MinInt32) - 1); got != math.MinInt32 {
		t.Errorf("Int32(MinInt32-1) = %d, want MinInt32", got)
	}
	if got := Int32(-7); got != -7 {
		t.Errorf("Int32(-7) = %d, want -7", got)
	}
	if got := Int64(math.MaxUint64); got != math.MaxInt64 {
		t.Errorf("Int64(MaxUint64) = %d, want MaxInt64", got)
	}
	if got := Int64(42); got != 42 {
		t.Errorf("Int64(42) = %d, want 42", got)
	}
	if got := Uint32(-1); got != 0 {
		t.Errorf("Uint32(-1) = %d, want 0", got)
	}
	if got := Uint32(int64(math.MaxUint32) + 1); got != math.MaxUint32 {
		t.Errorf("Uint32(MaxUint32+1) = %d, want MaxUint32", got)
	}
}
