package param

import (
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// TestEveryEnumMemberHasItsOwnToken walks the proto enum value maps rather than a hand-written list,
// so a member added to the contract without a case here fails instead of falling silently to
// "unspecified". Silent is the dangerous direction: a query filtering on the token would match
// nothing and read as "no such rows" rather than as an unrendered enum.
//
// Every relation test exercises only the members its fixtures happen to use, which is why this walks
// the map.
func TestEveryEnumMemberHasItsOwnToken(t *testing.T) {
	t.Run("LimitKind", func(t *testing.T) {
		seen := map[string]bool{}
		for name, n := range parampb.LimitKind_value {
			got := LimitKindToken(parampb.LimitKind(n))
			if n == 0 {
				if got != "unspecified" {
					t.Errorf("%s renders %q, want unspecified", name, got)
				}
				continue
			}
			if got == "unspecified" {
				t.Errorf("%s falls through to unspecified; add its case to LimitKindToken", name)
			}
			if seen[got] {
				t.Errorf("%s renders %q, already used by another member", name, got)
			}
			seen[got] = true
		}
	})
	t.Run("PinFunction", func(t *testing.T) {
		seen := map[string]bool{}
		for name, n := range parampb.PinFunction_value {
			got := PinFunctionToken(parampb.PinFunction(n))
			if n == 0 {
				if got != "unspecified" {
					t.Errorf("%s renders %q, want unspecified", name, got)
				}
				continue
			}
			if got == "unspecified" {
				t.Errorf("%s falls through to unspecified; add its case to PinFunctionToken", name)
			}
			if seen[got] {
				t.Errorf("%s renders %q, already used by another member", name, got)
			}
			seen[got] = true
		}
	})
	t.Run("Modality", func(t *testing.T) {
		seen := map[string]bool{}
		for name, n := range parampb.Modality_value {
			got := ModalityToken(parampb.Modality(n))
			if n == 0 {
				if got != "unspecified" {
					t.Errorf("%s renders %q, want unspecified", name, got)
				}
				continue
			}
			if got == "unspecified" {
				t.Errorf("%s falls through to unspecified; add its case to ModalityToken", name)
			}
			if seen[got] {
				t.Errorf("%s renders %q, already used by another member", name, got)
			}
			seen[got] = true
		}
	})
}

// TestTokensAreTheContractStrings pins the literals a saved datalog query matches on
// (`param.range(?m, ?s, "absolute_max", ?min, ?max)`). Renaming one silently stops matching rows
// that still exist, which reads as a clean result.
func TestTokensAreTheContractStrings(t *testing.T) {
	if got := LimitKindToken(parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX); got != "absolute_max" {
		t.Errorf("absolute max token = %q", got)
	}
	if got := LimitKindToken(parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING); got != "recommended_operating" {
		t.Errorf("recommended operating token = %q", got)
	}
	if got := PinFunctionToken(parampb.PinFunction_PIN_FUNCTION_NO_CONNECT); got != "no_connect" {
		t.Errorf("no connect token = %q", got)
	}
	if got := ModalityToken(parampb.Modality_MODALITY_REQUIRED); got != "required" {
		t.Errorf("required token = %q", got)
	}
}
