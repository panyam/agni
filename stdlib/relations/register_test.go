package relations_test

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/relations" // registers the built-in relations via init
)

// TestBuiltinRelationsRegisteredWithQuery is the seam guard (issue 10): blank-importing
// stdlib/relations must install the built-in EDB relations into the query engine through
// query.RegisterBuiltinFacts, so query.Catalog lists them and query.NewBase projects them. Before
// this package existed the query engine held these relations directly; if the init or the
// RegisterBuiltinFacts payload ever regressed (a dropped projector, an empty schema), this fails
// where the byte-identical CLI checks might not pinpoint the wiring.
func TestBuiltinRelationsRegisteredWithQuery(t *testing.T) {
	// The catalog lists the built-in relations, with their kinds, once relations is imported.
	want := map[string]string{
		"component.mpn":     query.KindNetlist,
		"rail":              query.KindNetlist,
		"board.track_width": query.KindBoard,
		"param":             query.KindDatasheet,
	}
	got := map[string]string{}
	for _, r := range query.Catalog() {
		got[r.Name] = r.Kind
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("query.Catalog() missing built-in relation %q with kind %q (got kind %q)", name, kind, got[name])
		}
	}

	// NewBase projects the built-in relations from a Model: a component on a net yields a
	// component-on-net fact, which is empty when the built-ins are not registered.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{{Name: "N1", Prov: &ir.Provenance{SourceFile: "t"},
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}},
	}
	rows, err := (query.Naive{}).Eval(query.MustParse(`component-on-net(?r,?n) => ?r, ?n`), query.NewBase(check.NewModel(d)))
	if err != nil {
		t.Fatalf("query over a built-in relation errored: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("component-on-net over U1/N1 = %d rows, want 1 (built-in relations not projected?)", len(rows))
	}
}

// TestBuiltinRelationCollisionDetected proves the schema half of the registration wired: an overlay
// RegisterRelation on a built-in name must panic, which only works if RegisterBuiltinFacts installed
// that name into the engine schema.
func TestBuiltinRelationCollisionDetected(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("RegisterRelation on the built-in name component.mpn did not panic; the built-in schema was not registered")
		}
	}()
	query.RegisterRelation("component.mpn", []query.Field{query.FieldSubject}, func(check.Model) []query.FactRow { return nil })
}
