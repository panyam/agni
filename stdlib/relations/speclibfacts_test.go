package relations

import (
	"testing"

	"github.com/panyam/agni/core/facts"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func TestSpecLibFacts(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	specs := []*parampb.PartSpec{
		{
			Mpn:        "ACME-LDO",
			Attributes: map[string]string{"audience": "powertrain, chassis"},
			Docs:       []*parampb.SourceDoc{{Id: "d", Title: "ACME Rev A"}},
			Parameters: []*parampb.Parameter{{
				Symbol: "VIN", LimitKind: parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
				Value: &parampb.RangeValue{Max: f(20)}, Unit: "V",
				Prov: &parampb.ParamProvenance{DocRef: "d", Page: 1, Confidence: 1},
			}},
		},
		{
			Mpn:  "ACME-MCU",
			Docs: []*parampb.SourceDoc{{Id: "d", Title: "MCU Rev A"}},
			Parameters: []*parampb.Parameter{{
				Symbol: "VDD", Value: &parampb.RangeValue{Max: f(3.6)}, Unit: "V",
				Prov: &parampb.ParamProvenance{DocRef: "d", Page: 1, Confidence: 1},
			}},
		},
		{Mpn: ""}, // no MPN: cannot join, skipped entirely
	}

	rows := SpecLibFacts(specs)
	byRel := map[string][]facts.Row{}
	for _, r := range rows {
		byRel[r.Relation] = append(byRel[r.Relation], r)
	}

	if got := len(byRel[RelParam]); got != 2 {
		t.Errorf("param rows = %d, want 2 (VIN + VDD; the no-MPN spec is skipped)", got)
	}
	aud := byRel[RelPartAudience]
	if len(aud) != 2 {
		t.Fatalf("part.audience rows = %d, want 2 (ACME-LDO x2; ACME-MCU has no audience)", len(aud))
	}
	for _, r := range aud {
		if r.Subject != "ACME-LDO" {
			t.Errorf("audience subject = %q, want ACME-LDO", r.Subject)
		}
	}
	// Sorted by relation, so param ("param") precedes part.audience — a spec library query prints stably.
	if len(rows) > 0 && rows[0].Relation != RelParam {
		t.Errorf("first row relation = %q, want %q (rows must be sorted)", rows[0].Relation, RelParam)
	}
	// A param row keeps its datasheet Citation through the spec library projection.
	if len(byRel[RelParam][0].Cites) == 0 {
		t.Error("spec library param row lost its Citation")
	}
}
