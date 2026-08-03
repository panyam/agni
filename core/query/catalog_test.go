package query

import (
	"testing"

	"github.com/panyam/agni/core/check"
)

// TestCatalogMatchesSchema is the drift guard: every built-in EDB relation and predicate has a
// catalog entry with the right arity, and no catalog entry names a construct that does not exist.
// A relation added to edbSchema (or a predicate to builtins) without a catalog row fails here, so a
// new relation cannot ship undiscoverable.
func TestCatalogMatchesSchema(t *testing.T) {
	byName := map[string]RelationInfo{}
	for _, r := range builtinCatalog {
		if _, dup := byName[r.Name]; dup {
			t.Fatalf("duplicate catalog entry %q", r.Name)
		}
		byName[r.Name] = r
	}
	// Every EDB relation is catalogued with matching arity.
	for rel, fields := range edbSchema {
		info, ok := byName[rel]
		if !ok {
			t.Errorf("edb relation %q has no catalog entry", rel)
			continue
		}
		if len(info.Args) != len(fields) {
			t.Errorf("catalog %q has %d args, edbSchema has %d fields", rel, len(info.Args), len(fields))
		}
	}
	// Every built-in predicate is catalogued.
	for name := range builtins {
		if _, ok := byName[name]; !ok {
			t.Errorf("built-in predicate %q has no catalog entry", name)
		}
	}
	// No catalog row names a nonexistent built-in construct.
	for name, info := range byName {
		_, isEDB := edbSchema[name]
		_, isPred := builtins[name]
		if !isEDB && !isPred {
			t.Errorf("catalog entry %q (kind %s) is neither an EDB relation nor a predicate", name, info.Kind)
		}
	}
}

func TestCatalogSortedByKindThenName(t *testing.T) {
	cat := Catalog()
	rank := map[string]int{}
	for i, k := range KindOrder {
		rank[k] = i
	}
	for i := 1; i < len(cat); i++ {
		prev, cur := cat[i-1], cat[i]
		if rank[prev.Kind] > rank[cur.Kind] {
			t.Errorf("kind order violated at %d: %s before %s", i, prev.Kind, cur.Kind)
		}
		if prev.Kind == cur.Kind && prev.Name > cur.Name {
			t.Errorf("name order violated within %s: %q before %q", cur.Kind, prev.Name, cur.Name)
		}
	}
}

func TestCatalogIncludesOverlayRelation(t *testing.T) {
	// Register a throwaway overlay relation and confirm it surfaces with synthesized arg labels.
	const name = "test.catalog_overlay"
	if _, seen := registry[name]; !seen {
		RegisterRelation(name, []Field{FieldSubject, FieldNum}, func(check.Model) []check.FactRow { return nil })
	}
	found := false
	for _, r := range Catalog() {
		if r.Name == name {
			found = true
			if r.Kind != KindOverlay {
				t.Errorf("overlay relation kind = %s, want %s", r.Kind, KindOverlay)
			}
			if len(r.Args) != 2 || r.Args[0] != "subject" || r.Args[1] != "n" {
				t.Errorf("overlay args = %v, want [subject n]", r.Args)
			}
		}
	}
	if !found {
		t.Errorf("overlay relation %q not in catalog", name)
	}
}
