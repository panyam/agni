package query

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// TestCatalogMatchesSchema is the drift guard: every EDB relation and predicate has a catalog entry
// with the right arity, and no catalog entry names a construct that does not exist. A relation added
// to the fact schema (or a predicate to builtins) without a catalog row fails here, so a new relation
// cannot ship undiscoverable. The relation half is registered with core/facts by stdlib/relations
// (imported for the test binary via relations_register_test.go) and the predicate half is query's own
// builtinPredicates; this test spans both, since Catalog does.
func TestCatalogMatchesSchema(t *testing.T) {
	byName := map[string]RelationInfo{}
	for _, r := range Catalog() {
		if _, dup := byName[r.Name]; dup {
			t.Fatalf("duplicate catalog entry %q", r.Name)
		}
		byName[r.Name] = r
	}
	// Every EDB relation is catalogued with matching arity.
	for rel, fields := range facts.DefaultRegistry().Schema() {
		info, ok := byName[rel]
		if !ok {
			t.Errorf("edb relation %q has no catalog entry", rel)
			continue
		}
		if len(info.Args) != len(fields) {
			t.Errorf("catalog %q has %d args, the fact schema has %d fields", rel, len(info.Args), len(fields))
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
		isEDB := facts.DefaultRegistry().IsRelation(name)
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
	reg := facts.RegistryWith(facts.WithRelation(name, []facts.Field{facts.FieldSubject, facts.FieldNum},
		func(check.Model) []facts.Row { return nil }))
	found := false
	for _, r := range CatalogFrom(reg) {
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

// entitySuggestingLabels are arg labels that READ like they name something on the canvas. They carry
// no meaning any more (a column's kind comes from ArgKinds), which is exactly why they need a guard:
// a relation added with one of these labels and no declaration is the shape that used to work by
// accident and now silently types as a scalar (agni issue 548).
var entitySuggestingLabels = map[string]bool{
	"ref_des": true, "net": true, "from": true, "pin": true, "name": true, "label": true,
}

// deliberateScalars are the (relation, arg) pairs whose label suggests an entity and which are
// genuinely NOT one. Each needed a hand-written guard in the old inference; here each is a line
// somebody wrote on purpose.
var deliberateScalars = map[string]string{
	// A datasheet pin belongs to a part TYPE, not to a placement, so there is nothing on the canvas
	// to locate. This pair is why the old code needed its pin/ref_des pairing guard.
	"param.pin/pin":       "a pin of a part type, not a placement",
	"param.pin_range/pin": "a pin of a part type, not a placement",
	"param.pin/name":      "the pin name a datasheet prints, not an entity name",
	// A bus label IS in the entity vocabulary and this is the one place the declaration could say
	// more than the old inference did. Left as a scalar so this change alters no answers; declaring
	// it is a behaviour change with its own review.
	"bus/label": "preserves the pre-declaration answer; making it locatable is a separate change",
}

// TestEveryEntitySuggestingLabelIsDecided fails when a relation carries an argument whose label reads
// like an entity and neither declares a kind for it nor records why it is a scalar. Silence is the
// failure mode this replaces, so silence is what it refuses.
func TestEveryEntitySuggestingLabelIsDecided(t *testing.T) {
	for _, ri := range Catalog() {
		for _, label := range ri.Args {
			if !entitySuggestingLabels[label] {
				continue
			}
			if _, declared := ri.ArgKinds[label]; declared {
				continue
			}
			if _, known := deliberateScalars[ri.Name+"/"+label]; known {
				continue
			}
			t.Errorf("%s declares no kind for its %q argument. Add one to ArgKinds, or add "+
				"%q to deliberateScalars with the reason it is not an entity",
				ri.Name, label, ri.Name+"/"+label)
		}
	}
}

// TestDeclaredArgKindsAreCoherent: a declaration must name arguments the relation actually has, and
// must pick exactly one of the three forms. A typo here would type a column as a scalar in silence,
// which is the failure this whole change exists to stop.
func TestDeclaredArgKindsAreCoherent(t *testing.T) {
	for _, ri := range Catalog() {
		args := map[string]bool{}
		for _, a := range ri.Args {
			args[a] = true
		}
		for label, k := range ri.ArgKinds {
			if !args[label] {
				t.Errorf("%s declares a kind for %q, which is not one of its arguments %v", ri.Name, label, ri.Args)
			}
			switch {
			case k.KindArg != "" && k.Entity != "":
				t.Errorf("%s/%s declares both a fixed Entity and a KindArg; a column takes its kind from one or the other", ri.Name, label)
			case k.KindArg != "" && !args[k.KindArg]:
				t.Errorf("%s/%s takes its kind from %q, which is not one of its arguments %v", ri.Name, label, k.KindArg, ri.Args)
			case k.OwnerArg != "" && !args[k.OwnerArg]:
				t.Errorf("%s/%s names owner %q, which is not one of its arguments %v", ri.Name, label, k.OwnerArg, ri.Args)
			case k.KindArg == "" && k.Entity == "":
				t.Errorf("%s/%s declares neither an Entity nor a KindArg, so it says nothing; omit it instead", ri.Name, label)
			}
		}
	}
}
