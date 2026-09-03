package facts

import (
	"fmt"
	"sync"

	"github.com/panyam/agni/core/check"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Projector derives a relation's rows from a Model. It runs once per fact base; an empty result is
// correct when the Model lacks the tier the relation needs (silent-by-construction, never
// fabricated). A projector DERIVES from the Model and never consults a second authoritative store
// (C8), which is what keeps the fact base a view rather than a copy.
type Projector func(check.Model) []Row

// SpecLibProjector derives rows from a whole datasheet spec library with NO design (WS10-010), the
// library-wide analogue of Projector, so a corpus can be searched without loading a board. A nil
// projector (nothing registered) yields an empty library base.
type SpecLibProjector func([]*parampb.PartSpec) []Row

// BuiltinFacts is the bulk payload the standard relation catalog installs. Schema is each relation's
// positional field layout; Catalog is the human-facing picker metadata; Model and SpecLib are the
// design-scoped and library-wide projectors; Doc resolves a relation's reference markdown ("" when it
// has none).
//
// Bulk, not per-relation: RegisterRelation stays the per-relation OVERLAY seam (one relation at a
// time, namespaced), while the built-ins arrive as one payload because their projector is a single
// monolithic pass and several share it. The rule side made the same call (check.RegisterBuiltins).
type BuiltinFacts struct {
	Schema  map[string][]Field
	Catalog []RelationInfo
	Model   Projector
	SpecLib SpecLibProjector
	Doc     func(name string) string
}

type relationDef struct {
	fields  []Field
	project Projector
}

var (
	mu       sync.RWMutex
	schema   = map[string][]Field{} // built-in relation -> positional layout
	registry = map[string]relationDef{}
	order    []string // overlay registration order, so a fact base merges deterministically
	// reserved holds names an ENGINE has claimed for a computed predicate of its own (core/query's
	// reaches and string filters). It exists so the collision check survives in either init order:
	// this package cannot know what a predicate is, and an engine's init may run before or after a
	// relation catalog's, so both directions are checked rather than one.
	reserved = map[string]string{} // name -> the engine that claimed it

	builtinModel   Projector
	builtinSpecLib SpecLibProjector
	builtinCatalog []RelationInfo
	builtinDoc     func(name string) string
	installed      bool // RegisterBuiltinFacts has run; see Installed
)

// RegisterBuiltinFacts installs the standard relations. The catalog package calls it from its init,
// so any binary that blank-imports that package has the relations available before it builds a fact
// base. It panics on a name that collides with an already-registered relation or an engine-reserved
// predicate, because a duplicate registration is a programming error that must fail loudly at load
// rather than silently at query time.
func RegisterBuiltinFacts(bf BuiltinFacts) {
	mu.Lock()
	defer mu.Unlock()
	for name, fields := range bf.Schema {
		if _, ok := schema[name]; ok {
			panic(fmt.Sprintf("facts: RegisterBuiltinFacts(%q) collides with a registered relation", name))
		}
		if who, ok := reserved[name]; ok {
			panic(fmt.Sprintf("facts: RegisterBuiltinFacts(%q) collides with a predicate reserved by %s", name, who))
		}
		schema[name] = append([]Field(nil), fields...)
	}
	builtinModel = bf.Model
	builtinSpecLib = bf.SpecLib
	builtinCatalog = bf.Catalog
	builtinDoc = bf.Doc
	installed = true
}

// RegisterRelation adds an overlay-supplied relation. name is the relation as queries write it (e.g.
// "house.approved"), fields is its positional layout over Row, and project derives its rows from a
// Model. Call it once at init time.
//
// This is the overlay seam (the open-core story generalized to the fact layer): the public engine
// ships the built-in relations, and a private overlay contributes its OWN — house part attributes, a
// compliance database, an approved-vendor feed — without editing the engine. A registered relation is
// a first-class citizen of every query surface, with no evaluator change, because an engine treats
// every relation uniformly (name -> field layout -> rows).
func RegisterRelation(name string, fields []Field, project Projector) {
	mu.Lock()
	defer mu.Unlock()
	if name == "" {
		panic("facts: RegisterRelation with empty name")
	}
	if len(fields) == 0 {
		panic(fmt.Sprintf("facts: RegisterRelation(%q) with no fields", name))
	}
	if project == nil {
		panic(fmt.Sprintf("facts: RegisterRelation(%q) with nil projector", name))
	}
	if _, ok := schema[name]; ok {
		panic(fmt.Sprintf("facts: RegisterRelation(%q) collides with a built-in relation", name))
	}
	if who, ok := reserved[name]; ok {
		panic(fmt.Sprintf("facts: RegisterRelation(%q) collides with a predicate reserved by %s", name, who))
	}
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("facts: RegisterRelation(%q) already registered", name))
	}
	registry[name] = relationDef{fields: append([]Field(nil), fields...), project: project}
	order = append(order, name)
}

// Reserve claims a name for an engine's own computed predicate, so a relation cannot shadow it. who
// names the claimant for the panic message. It is idempotent for the same claimant, so an engine may
// reserve its vocabulary from an init that runs more than once in a test binary.
//
// It panics when the name is already a relation, which is the other half of the checks above: the two
// registrations may run in either order (a relation catalog and an engine are independent imports),
// so each one checks against the other rather than assuming it went second.
func Reserve(who string, names ...string) {
	mu.Lock()
	defer mu.Unlock()
	for _, name := range names {
		if prev, ok := reserved[name]; ok && prev != who {
			panic(fmt.Sprintf("facts: Reserve(%q) by %s already reserved by %s", name, who, prev))
		}
		if _, ok := schema[name]; ok {
			panic(fmt.Sprintf("facts: Reserve(%q) by %s collides with a built-in relation", name, who))
		}
		if _, ok := registry[name]; ok {
			panic(fmt.Sprintf("facts: Reserve(%q) by %s collides with a registered relation", name, who))
		}
		reserved[name] = who
	}
}

// SchemaOf resolves a relation's positional layout, built-in first and then the overlay registry, so
// an engine treats a registered relation exactly like a built-in one.
func SchemaOf(rel string) ([]Field, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if f, ok := schema[rel]; ok {
		return f, true
	}
	if d, ok := registry[rel]; ok {
		return d.fields, true
	}
	return nil, false
}

// IsRelation reports whether a name is a fact-base relation (built-in or overlay-registered), so an
// engine can refuse to let a derived rule redefine one.
func IsRelation(rel string) bool {
	_, ok := SchemaOf(rel)
	return ok
}

// Rows projects the whole fact base for a design: the built-in relations first, then each
// overlay-registered relation in registration order, so a merge is deterministic.
//
// An overlay row is stamped with the name it was REGISTERED under, overriding whatever the projector
// put in Row.Relation. The registration name is the one a query writes, so letting a projector name
// its own rows would let a relation answer under a name nothing registered — and silently shadow
// another. The built-in payload is a single pass over many relations, so its rows carry their own
// Relation and are taken as given.
func Rows(m check.Model) []Row {
	mu.RLock()
	model, names := builtinModel, append([]string(nil), order...)
	defs := make([]relationDef, len(names))
	for i, n := range names {
		defs[i] = registry[n]
	}
	mu.RUnlock()

	var out []Row
	if model != nil {
		out = append(out, model(m)...)
	}
	for i, d := range defs {
		for _, r := range d.project(m) {
			r.Relation = names[i]
			out = append(out, r)
		}
	}
	return out
}

// SpecLibRows projects the datasheet spec library with no design attached.
func SpecLibRows(specs []*parampb.PartSpec) []Row {
	mu.RLock()
	p := builtinSpecLib
	mu.RUnlock()
	if p == nil {
		return nil
	}
	return p(specs)
}

// Installed reports whether a relation catalog has been registered at all. It is the difference
// between "this design has no such facts" and "no relations were ever installed", which a fact base
// otherwise flattens into the same empty result — and an engine whose relations are all missing
// answers nothing while looking exactly like an engine whose query matched nothing. That reads as a
// clean pass on a design nobody checked, so a caller composing an engine gates on this rather than
// discovering it from an empty answer.
func Installed() bool {
	mu.RLock()
	defer mu.RUnlock()
	return installed || len(registry) > 0
}

// Schema returns every registered relation's positional layout, built-in and overlay alike, as a
// copy. It exists for the drift guard that asserts the catalog covers the schema
// (TestCatalogMatchesSchema): a relation that is queryable but undiscoverable is the failure it
// catches, and catching it needs the whole set rather than one lookup.
func Schema() map[string][]Field {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string][]Field, len(schema)+len(registry))
	for name, f := range schema {
		out[name] = append([]Field(nil), f...)
	}
	for name, d := range registry {
		out[name] = append([]Field(nil), d.fields...)
	}
	return out
}

// Snapshot captures the whole registry and returns a function that restores it. It exists for tests
// that register a throwaway relation (registration is process-global and panics on a duplicate, so a
// leaked one breaks the next test), and for a host that composes more than one engine in one process.
//
// The returned function is not safe to call concurrently with a registration, which is the same
// constraint the Register* functions already carry: registration is an init-time activity.
func Snapshot() (restore func()) {
	mu.Lock()
	defer mu.Unlock()
	oSchema := make(map[string][]Field, len(schema))
	for k, v := range schema {
		oSchema[k] = v
	}
	oReg := make(map[string]relationDef, len(registry))
	for k, v := range registry {
		oReg[k] = v
	}
	oReserved := make(map[string]string, len(reserved))
	for k, v := range reserved {
		oReserved[k] = v
	}
	oOrder := append([]string(nil), order...)
	oModel, oSpec, oCat, oDoc, oInst := builtinModel, builtinSpecLib, builtinCatalog, builtinDoc, installed
	return func() {
		mu.Lock()
		defer mu.Unlock()
		schema, registry, reserved, order = oSchema, oReg, oReserved, oOrder
		builtinModel, builtinSpecLib, builtinCatalog, builtinDoc, installed = oModel, oSpec, oCat, oDoc, oInst
	}
}

// Reset empties the registry. It is the companion to Snapshot — take a snapshot, Reset, exercise a
// bare registry, restore — and exists so the uninstalled-catalog state is reachable in a test.
// That state is otherwise unreachable from any binary that imports a catalog, and it is the one
// state where every relation is unknown, so leaving it untestable is how the silent-empty fact base
// survived (see Installed).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	schema = map[string][]Field{}
	registry = map[string]relationDef{}
	reserved = map[string]string{}
	order = nil
	builtinModel, builtinSpecLib, builtinCatalog, builtinDoc = nil, nil, nil, nil
	installed = false
}
