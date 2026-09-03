package facts

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

// TestInstalledFalseOnBareRegistry pins the distinction the package exists to keep: a fact base with
// no relations INSTALLED is a different state from one whose relations matched nothing, and only the
// second is a clean result. A host that omits the relation-catalog import still builds and still
// runs, so this flag is the only place the omission is answerable.
func TestInstalledFalseOnBareRegistry(t *testing.T) {
	restore := Snapshot()
	defer restore()
	schema, registry, order, installed = map[string][]Field{}, map[string]relationDef{}, nil, false
	builtinModel, builtinSpecLib, builtinCatalog, builtinDoc = nil, nil, nil, nil
	if Installed() {
		t.Error("Installed() is true with nothing registered")
	}
	if got := Rows(nil); len(got) != 0 {
		t.Errorf("Rows on a bare registry = %d rows, want 0", len(got))
	}
	// An overlay-only host IS installed: relations came from somewhere, just not the built-in catalog.
	RegisterRelation("test.only", []Field{FieldSubject}, func(check.Model) []Row { return nil })
	if !Installed() {
		t.Error("Installed() is false after an overlay relation registered")
	}
}

// TestReserveIsOrderIndependent pins that a relation and an engine's predicate collide whichever
// registers first. The two are independent imports, so assuming an order is how a shadowed relation
// ships.
func TestReserveIsOrderIndependent(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func()
	}{
		{"reserve then register", func() {
			Reserve("test-engine", "test.clash")
			RegisterRelation("test.clash", []Field{FieldSubject}, func(check.Model) []Row { return nil })
		}},
		{"register then reserve", func() {
			RegisterRelation("test.clash", []Field{FieldSubject}, func(check.Model) []Row { return nil })
			Reserve("test-engine", "test.clash")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := Snapshot()
			defer restore()
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("no panic; a predicate and a relation sharing a name must fail at load")
				}
				if !strings.Contains(r.(string), "test.clash") {
					t.Errorf("panic %v does not name the colliding relation", r)
				}
			}()
			tc.run()
		})
	}
}
