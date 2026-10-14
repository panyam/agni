package facts

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/panyam/agni/core/check"
)

// TestAProjectionRunsOnlyTheProjectorItReads reads one relation and checks no other projector ran,
// then that each projector runs once however many of its relations are read, concurrently included.
func TestAProjectionRunsOnlyTheProjectorItReads(t *testing.T) {
	var ab, c, ext atomic.Int32
	reg, err := NewRegistry(
		WithBuiltins(BuiltinFacts{
			Schema: map[string][]Field{"t.a": {FieldSubject}, "t.b": {FieldSubject}, "t.c": {FieldSubject}},
			Groups: []ProjectorGroup{
				{Relations: []string{"t.a", "t.b"}, Project: func(check.Model) []Row {
					ab.Add(1)
					return []Row{{Relation: "t.a", Subject: "1"}, {Relation: "t.b", Subject: "2"}}
				}},
				{Relations: []string{"t.c"}, Project: func(check.Model) []Row {
					c.Add(1)
					return []Row{{Relation: "t.c", Subject: "3"}}
				}},
			},
		}),
		WithRelation("t.ext", []Field{FieldSubject}, func(check.Model) []Row {
			ext.Add(1)
			return []Row{{Relation: "elsewhere", Subject: "4"}}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	p := reg.Projection(nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Rows("t.a")
		}()
	}
	wg.Wait()
	if got := p.Rows("t.b"); len(got) != 1 || got[0].Subject != "2" {
		t.Fatalf("t.b = %+v, want the row its group emitted", got)
	}
	if ab.Load() != 1 || c.Load() != 0 || ext.Load() != 0 {
		t.Fatalf("after reading t.a and t.b: ran t.a/t.b %d, t.c %d, t.ext %d times; want 1, 0, 0", ab.Load(), c.Load(), ext.Load())
	}
	if got := p.Rows("t.ext"); len(got) != 1 || got[0].Relation != "t.ext" {
		t.Fatalf("t.ext = %+v, want one row stamped with the registered name", got)
	}
	if p.Rows("t.none") != nil {
		t.Fatal("a relation the registry lacks answered rows")
	}
}

// TestAProjectionWithoutGroupsRunsTheWholePayloadOnce covers a built-in catalog that supplies only
// Model, which a Projection runs whole the first time a built-in relation is read.
func TestAProjectionWithoutGroupsRunsTheWholePayloadOnce(t *testing.T) {
	var runs atomic.Int32
	reg, err := NewRegistry(WithBuiltins(BuiltinFacts{
		Schema: map[string][]Field{"t.a": {FieldSubject}, "t.b": {FieldSubject}},
		Model: func(check.Model) []Row {
			runs.Add(1)
			return []Row{{Relation: "t.a", Subject: "1"}, {Relation: "t.b", Subject: "2"}}
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	p := reg.Projection(nil)
	if len(p.Rows("t.a")) != 1 || len(p.Rows("t.b")) != 1 || runs.Load() != 1 {
		t.Fatalf("ran the payload %d times, want once, with a row for each relation", runs.Load())
	}
}
