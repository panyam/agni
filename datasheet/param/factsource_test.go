package param

import (
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// ParamSet is the directory-backed FactSource; the bank-wide datalog surface depends on this.
var _ FactSource = ParamSet{}

func TestParamSetAllSpecs(t *testing.T) {
	s := ParamSet{
		"ACME-B": {Mpn: "ACME-B"},
		"ACME-A": {Mpn: "ACME-A"},
		"ACME-C": {Mpn: "ACME-C"},
	}
	got := s.AllSpecs()
	if len(got) != 3 {
		t.Fatalf("AllSpecs len = %d, want 3", len(got))
	}
	// Ordered by key so a bank-wide query prints deterministically.
	for i, want := range []string{"ACME-A", "ACME-B", "ACME-C"} {
		if got[i].GetMpn() != want {
			t.Errorf("AllSpecs[%d] = %q, want %q (must be sorted by key)", i, got[i].GetMpn(), want)
		}
	}
	if n := len(ParamSet(nil).AllSpecs()); n != 0 {
		t.Errorf("nil set AllSpecs = %d rows, want 0", n)
	}
}

// ensure the proto import is used even if the struct literals above ever drop the field.
var _ = &parampb.PartSpec{}
