package query

import (
	"testing"

	"github.com/panyam/agni/datasheet/param"
)

// NewSpecLibBase queries the whole seeded corpus with no design: the datasheet relations range over
// every PartSpec, and model-dependent relations/predicates yield nothing rather than panicking on the
// absent model.
func TestNewSpecLibBase(t *testing.T) {
	ldo := regSpec("ACME-LDO", 20)
	ldo.Attributes = map[string]string{param.AudienceKey: "powertrain"}
	set := param.ParamSet{"ACME-LDO": ldo, "ACME-MCU": regSpec("ACME-MCU", 3.6)}
	b := NewSpecLibBase(set)

	// param ranges the whole spec library (both parts), no design join.
	rows, err := (Naive{}).Eval(mustParse(t, `param(?mpn, ?sym, ?max)`), b)
	if err != nil {
		t.Fatalf("spec library param query: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("spec library param rows = %d, want 2 (both corpus parts)", len(rows))
	}

	// part.audience: only the annotated part appears.
	arows, err := (Naive{}).Eval(mustParse(t, `part.audience(?mpn, ?who)`), b)
	if err != nil {
		t.Fatalf("spec library part.audience query: %v", err)
	}
	if len(arows) != 1 {
		t.Fatalf("part.audience rows = %d, want 1 (only ACME-LDO annotated)", len(arows))
	}

	// reaches is model-dependent; a spec library has no model, so it is a clean empty, not a panic.
	rrows, err := (Naive{}).Eval(mustParse(t, `reaches(?a, ?b)`), b)
	if err != nil {
		t.Fatalf("reaches over a spec library should be a clean empty, got error %v", err)
	}
	if len(rrows) != 0 {
		t.Errorf("reaches over a modelless spec library = %d rows, want 0", len(rrows))
	}
}
