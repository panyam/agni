package facts

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/jaala/ns"
)

func rel(name string) Option {
	return WithRelation(name, []Field{FieldSubject}, func(check.Model) []Row { return nil })
}

// TestInstalledSeparatesEmptyFromAbsent pins the distinction the package exists to keep. A registry
// with no relations is a different state from one whose relations matched nothing, and only the
// second is a clean result. A host that omits the relation-catalog import still builds and still
// runs, so this is the only place the omission is answerable.
func TestInstalledSeparatesEmptyFromAbsent(t *testing.T) {
	bare, err := NewRegistry()
	if err != nil {
		t.Fatalf("empty registry: %v", err)
	}
	if bare.Installed() {
		t.Error("Installed() is true with nothing registered")
	}
	if got := bare.Rows(nil); len(got) != 0 {
		t.Errorf("Rows on a bare registry = %d rows, want 0", len(got))
	}
	// An overlay-only registry IS installed, since relations came from somewhere, just not a
	// built-in catalog.
	one, err := NewRegistry(rel("test.only"))
	if err != nil {
		t.Fatalf("overlay-only registry: %v", err)
	}
	if !one.Installed() {
		t.Error("Installed() is false with an overlay relation registered")
	}
}

// TestCompositionIsOrderIndependent is why composing beats accumulating. A predicate and a relation
// sharing a name must clash whichever was supplied first; an engine and a relation catalog are
// independent imports whose init order nothing controls, so a check that ran at registration time had
// to look in both directions to cover the same ground.
func TestCompositionIsOrderIndependent(t *testing.T) {
	for name, opts := range map[string][]Option{
		"predicate then relation": {WithPredicate("test.clash", always), rel("test.clash")},
		"relation then predicate": {rel("test.clash"), WithPredicate("test.clash", always)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewRegistry(opts...)
			if err == nil {
				t.Fatal("no error; a predicate and a relation sharing a name must fail composition")
			}
			if !strings.Contains(err.Error(), "test.clash") {
				t.Errorf("error %v does not name the colliding relation", err)
			}
		})
	}
}

// always is a filter predicate that keeps every binding.
var always = ns.Filter(1, func([]ns.Value) (bool, error) { return true, nil })

// TestTreeRulesHoldAcrossEveryKind pins that the vocabulary's two rules (one definer per path; a
// segment is a module or a member) reach agni's composition for every kind of name it registers,
// including the string tests the engine contract ships, so a relation cannot take str.contains.
func TestTreeRulesHoldAcrossEveryKind(t *testing.T) {
	cases := map[string][]Option{
		"relation and predicate":       {rel("test.f"), WithPredicate("test.f", always)},
		"two predicates":               {WithPredicate("test.f", always), WithPredicate("test.f", always)},
		"relation and standard filter": {rel("str.contains")},
		"member and module prefix":     {rel("test"), rel("test.child")},
		"module in no known language":  {WithModule("lib", "no-such-language", "a(?x) :- b(?x);")},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRegistry(opts...); err == nil {
				t.Error("composed; want the vocabulary to refuse it")
			}
		})
	}
	if _, err := NewRegistry(rel("test.a"), rel("test.b"), WithPredicate("test.f", always)); err != nil {
		t.Errorf("a well-formed tree was refused: %v", err)
	}
}

// TestPredicatesListsEveryPredicateOnce pins the catalog's view of the vocabulary: the standard string
// tests and a registered predicate are both listed, each as a predicate, and a relation is not.
func TestPredicatesListsEveryPredicateOnce(t *testing.T) {
	reg, err := NewRegistry(rel("test.r"), WithPredicate("test.f", always))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, p := range reg.Predicates() {
		seen[p.Name]++
		if p.Kind != KindPredicate {
			t.Errorf("%s listed as %q, want %q", p.Name, p.Kind, KindPredicate)
		}
	}
	for _, want := range []string{"test.f", "str.contains", "str.glob", "absent"} {
		if seen[want] != 1 {
			t.Errorf("%s listed %d times, want once", want, seen[want])
		}
	}
	if seen["test.r"] != 0 {
		t.Error("a relation is listed as a predicate")
	}
}

// TestNewRegistryReportsEveryProblem pins that composition accumulates rather than short-circuits. A
// caller fixing a composition wants the whole list, not one problem per rebuild.
func TestNewRegistryReportsEveryProblem(t *testing.T) {
	_, err := NewRegistry(
		WithRelation("", []Field{FieldSubject}, func(check.Model) []Row { return nil }),
		WithRelation("test.nofields", nil, func(check.Model) []Row { return nil }),
		WithRelation("test.nilproj", []Field{FieldSubject}, nil),
	)
	if err == nil {
		t.Fatal("three malformed relations composed without error")
	}
	for _, want := range []string{"empty name", "test.nofields", "test.nilproj"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %v omits %q", err, want)
		}
	}
}

// TestDuplicateRelationFailsComposition pins that a name cannot be claimed twice, which is what keeps
// a query's meaning from depending on which registration ran last.
func TestDuplicateRelationFailsComposition(t *testing.T) {
	if _, err := NewRegistry(rel("test.dup"), rel("test.dup")); err == nil {
		t.Fatal("a relation registered twice composed without error")
	}
}

// TestRegistryIsASnapshot pins the property that makes a Registry a value rather than a view. A
// registration after composition does not change what an existing registry answers. A caller holding
// one cannot have it change underneath them.
func TestRegistryIsASnapshot(t *testing.T) {
	before, err := NewRegistry(rel("test.before"))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	after, err := NewRegistry(rel("test.before"), rel("test.after"))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if before.IsRelation("test.after") {
		t.Error("a registry composed earlier sees a relation added later")
	}
	if !after.IsRelation("test.after") {
		t.Error("a registry composed later does not see its own relation")
	}
}

// TestOverlayRowsAreStampedWithTheRegisteredName pins that a projector cannot name its own rows.
// The registration name is the one a query writes, so a projector naming them could answer under a
// name nothing registered, and silently shadow another relation.
func TestOverlayRowsAreStampedWithTheRegisteredName(t *testing.T) {
	reg, err := NewRegistry(WithRelation("test.stamped", []Field{FieldSubject}, func(check.Model) []Row {
		return []Row{{Relation: "something.else", Subject: "U1"}}
	}))
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	rows := reg.Rows(nil)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].Relation != "test.stamped" {
		t.Errorf("row relation = %q, want the registered name", rows[0].Relation)
	}
}
