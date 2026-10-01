package facts

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/panyam/agni/core/check"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Globals for REGISTRATION, values for USE.
//
// The only package state is a buffer of registration options, written at init by whatever packages a
// binary composed in. Nothing reads it directly. DefaultRegistry composes it into a *Registry, which
// is immutable once built, so every read goes through a value a caller holds. check.Catalog has the
// same shape (NewCatalog / DefaultCatalog / CatalogWith). Collisions are checked once, after
// composing, so a relation and a predicate clash whichever registered first and init order never
// matters.
//
// Registration stays global because it is the extension hook (C18). A private extension blank-imports
// a package whose init calls RegisterRelation, so the engine is composed BY the extension rather than
// coupled to one. It is a startup default never mutated per run, the carve-out C22 makes for ambient
// state.

// Projector derives a relation's rows from a Model. It runs once per fact base, and an empty result is
// correct when the Model lacks the tier the relation needs (never fabricated). A projector DERIVES
// from the Model and never consults a second authoritative store (C8), so the fact base stays a view
// rather than a copy.
type Projector func(check.Model) []Row

// SpecLibProjector derives rows from a whole datasheet spec library with NO design (WS10-010), the
// library-wide analogue of Projector, so a corpus can be searched without loading a board.
type SpecLibProjector func([]*parampb.PartSpec) []Row

// BuiltinFacts is the bulk payload a standard relation catalog installs. Schema is each relation's
// positional field layout; Catalog is the human-facing picker metadata; Model and SpecLib are the
// design-scoped and library-wide projectors; Doc resolves a relation's reference markdown ("" when it
// has none).
//
// The built-ins arrive as one payload because their projector is a single monolithic pass that
// several relations share. One relation at a time is the EXTENSION shape (WithRelation). The rule side
// does the same (check.RegisterBuiltins).
type BuiltinFacts struct {
	Schema  map[string][]Field
	Catalog []RelationInfo
	Model   Projector
	SpecLib SpecLibProjector
	Doc     func(name string) string
}

// Relation is one extension-supplied relation as a value: the name a query writes, its positional
// layout over Row, and the projector that derives its rows.
type Relation struct {
	Name    string
	Fields  []Field
	Project Projector
}

// Registry is a composed relation catalog: what relations exist, how each projects, and what to call
// them. It is built once and never mutated, so reads need no lock and a caller holding one cannot
// have it change underneath them because something else registered later.
type Registry struct {
	schema    map[string][]Field // every relation, built-in and extension alike
	extension []Relation         // in composition order, so a merge is deterministic
	builtin   BuiltinFacts
	reserved  map[string]string // predicate name -> the engine that claimed it
}

// An Option contributes to a Registry under construction. The three kinds mirror the three ways a
// relation vocabulary is assembled: a bulk built-in payload, one extension relation, and an engine
// claiming names for predicates it computes itself.
type Option func(*builder)

type builder struct {
	builtin    BuiltinFacts
	hasBuiltin bool
	extension  []Relation
	reserved   map[string]string
	errs       []error
}

func (b *builder) err(e error) { b.errs = append(b.errs, e) }

// WithBuiltins installs a bulk relation catalog. Only one may be supplied.
func WithBuiltins(bf BuiltinFacts) Option {
	return func(b *builder) {
		if b.hasBuiltin {
			b.err(errors.New("two built-in relation catalogs supplied; one catalog owns the built-in vocabulary"))
			return
		}
		b.builtin, b.hasBuiltin = bf, true
	}
}

// WithRelation adds one extension-supplied relation, such as a private extension's house part
// attributes or approved-vendor feed, without editing the engine. It reaches every query surface with
// no evaluator change, because an engine treats every relation uniformly (name -> field layout ->
// rows).
func WithRelation(name string, fields []Field, project Projector) Option {
	return func(b *builder) {
		switch {
		case name == "":
			b.err(errors.New("relation with empty name"))
		case len(fields) == 0:
			b.err(fmt.Errorf("relation %q has no fields", name))
		case project == nil:
			b.err(fmt.Errorf("relation %q has a nil projector", name))
		default:
			b.extension = append(b.extension, Relation{Name: name, Fields: append([]Field(nil), fields...), Project: project})
		}
	}
}

// Reserving claims names for an engine's own computed predicates (core/query's reaches and the string
// filters), so a relation cannot shadow one. who names the claimant, for the error message.
//
// Order does not matter. Collisions are swept once after every option applies, so a predicate and a
// relation clash whichever registered first, which matters because an engine and a relation catalog
// are independent imports with no controllable init order.
func Reserving(who string, names ...string) Option {
	return func(b *builder) {
		for _, n := range names {
			if prev, ok := b.reserved[n]; ok && prev != who {
				b.err(fmt.Errorf("predicate %q claimed by both %s and %s", n, prev, who))
				continue
			}
			b.reserved[n] = who
		}
	}
}

// NewRegistry composes a Registry from the given options. It reports every problem it found rather
// than only the first, so a caller fixing a composition sees the whole list.
func NewRegistry(opts ...Option) (*Registry, error) {
	b := &builder{reserved: map[string]string{}}
	for _, o := range opts {
		o(b)
	}
	r := &Registry{schema: map[string][]Field{}, builtin: b.builtin, reserved: b.reserved}
	for name, f := range b.builtin.Schema {
		r.schema[name] = append([]Field(nil), f...)
	}
	for _, rel := range b.extension {
		if _, dup := r.schema[rel.Name]; dup {
			b.err(fmt.Errorf("relation %q is registered twice", rel.Name))
			continue
		}
		r.schema[rel.Name] = rel.Fields
		r.extension = append(r.extension, rel)
	}
	// One collision sweep after everything is in, sorted so the error reads the same on every run.
	var clashes []string
	for name := range r.schema {
		if who, ok := r.reserved[name]; ok {
			clashes = append(clashes, fmt.Sprintf("relation %q collides with a predicate reserved by %s", name, who))
		}
	}
	sort.Strings(clashes)
	for _, c := range clashes {
		b.err(errors.New(c))
	}
	if len(b.errs) > 0 {
		return nil, fmt.Errorf("facts: %w", errors.Join(b.errs...))
	}
	return r, nil
}

// SchemaOf resolves a relation's positional layout. An extension relation resolves exactly like a
// built-in one.
func (r *Registry) SchemaOf(rel string) ([]Field, bool) {
	f, ok := r.schema[rel]
	return f, ok
}

// IsRelation reports whether a name is a fact-base relation, so an engine can refuse to let a derived
// rule redefine one.
func (r *Registry) IsRelation(rel string) bool { _, ok := r.schema[rel]; return ok }

// InfoOf resolves one relation's catalog entry, for a caller that needs its per-argument metadata
// rather than its row layout. It searches the built-in catalog only, so an extension relation reports
// false. It is a linear scan because it runs once per query at validation time over about a hundred
// entries.
func (r *Registry) InfoOf(rel string) (RelationInfo, bool) {
	for _, info := range r.builtin.Catalog {
		if info.Name == rel {
			return info, true
		}
	}
	return RelationInfo{}, false
}

// Schema returns every relation's layout as a copy. It exists for the drift guard asserting the
// catalog covers the schema, which needs the whole set to catch a relation that is queryable but
// undiscoverable.
func (r *Registry) Schema() map[string][]Field {
	out := make(map[string][]Field, len(r.schema))
	for name, f := range r.schema {
		out[name] = append([]Field(nil), f...)
	}
	return out
}

// Installed reports whether this registry carries any relation at all. It separates "this design has
// no such facts" from "no relations were ever installed", which an empty fact base flattens into the
// same result, and the second reads as a clean pass on a design nobody checked.
func (r *Registry) Installed() bool { return len(r.schema) > 0 }

// Rows projects the whole fact base for a design, the built-in relations first and then each
// extension relation in composition order.
//
// An extension row is stamped with the name it was REGISTERED under, overriding whatever the projector
// put in Row.Relation, so a projector cannot answer under a name nothing registered and shadow
// another. The built-in payload is one pass over many relations, so its rows keep their own Relation.
func (r *Registry) Rows(m check.Model) []Row {
	var out []Row
	if r.builtin.Model != nil {
		out = append(out, r.builtin.Model(m)...)
	}
	for _, rel := range r.extension {
		for _, row := range rel.Project(m) {
			row.Relation = rel.Name
			out = append(out, row)
		}
	}
	return out
}

// SpecLibRows projects the datasheet spec library with no design attached.
func (r *Registry) SpecLibRows(specs []*parampb.PartSpec) []Row {
	if r.builtin.SpecLib == nil {
		return nil
	}
	return r.builtin.SpecLib(specs)
}

// Relations returns the discoverable relation set, the built-ins plus each extension relation, the
// latter with argument labels synthesized from its field layout. Order is composition order, unsorted,
// so a caller that also has predicates to show merges both lists and sorts once.
//
// Each entry's Detail is resolved through the doc resolver, so an undocumented relation still lists
// with its Summary.
func (r *Registry) Relations() []RelationInfo {
	out := make([]RelationInfo, 0, len(r.builtin.Catalog)+len(r.extension))
	out = append(out, r.builtin.Catalog...)
	for _, rel := range r.extension {
		args := make([]string, len(rel.Fields))
		for i, f := range rel.Fields {
			args[i] = f.Label()
		}
		out = append(out, RelationInfo{Name: rel.Name, Args: args, Summary: "extension-registered relation", Kind: KindExtension})
	}
	for i := range out {
		out[i].Detail = r.Doc(out[i].Name)
	}
	return out
}

// Doc resolves a relation's reference markdown, or "" when this registry has no resolver or the
// relation has no doc.
func (r *Registry) Doc(name string) string {
	if r.builtin.Doc == nil {
		return ""
	}
	return r.builtin.Doc(name)
}

// The registration buffer is the only package state. It is written at init and read by RegistryWith,
// Registered and addOption.

var (
	regMu      sync.Mutex
	registered []Option
)

// RegisterBuiltinFacts installs a bulk relation catalog into the process default. A relation catalog
// package calls it from its init, so any binary that blank-imports that package has the relations.
func RegisterBuiltinFacts(bf BuiltinFacts) { addOption(WithBuiltins(bf)) }

// RegisterRelation adds an extension-supplied relation to the process default. Call it once at init.
func RegisterRelation(name string, fields []Field, project Projector) {
	addOption(WithRelation(name, fields, project))
}

// Reserve claims names for an engine's computed predicates in the process default.
func Reserve(who string, names ...string) { addOption(Reserving(who, names...)) }

// addOption composes the option with everything registered so far and panics if that fails, before
// appending. The buffer is APPEND-ONLY, so an admitted bad registration would poison every later
// DefaultRegistry and surface at some unrelated caller's first query. Validating here puts the panic
// at the registration that created the conflict, naming whichever party arrived second.
//
// Cost is quadratic in the number of registrations, which is init-time and in the low tens.
func addOption(o Option) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, err := NewRegistry(append(append([]Option(nil), registered...), o)...); err != nil {
		panic("facts: " + err.Error())
	}
	registered = append(registered, o)
}

// DefaultRegistry composes everything registered at init. It panics on a composition error, because a
// duplicate or shadowing relation is a programming error that must fail loudly at load rather than
// silently at query time. check.DefaultCatalog has the same contract.
func DefaultRegistry() *Registry { return RegistryWith() }

// RegistryWith composes the registered options followed by the caller's extras, under the same
// checks. An embedder uses it to add relations explicitly rather than through global registration.
// A test uses it to build a registry that owes nothing to what the test binary happened to import.
func RegistryWith(extra ...Option) *Registry {
	regMu.Lock()
	opts := append(append([]Option(nil), registered...), extra...)
	regMu.Unlock()
	r, err := NewRegistry(opts...)
	if err != nil {
		panic("facts: registry failed composition: " + err.Error())
	}
	return r
}

// Registered returns a copy of the options registered at init, so a caller can compose them with its
// own and handle a composition error rather than take RegistryWith's panic. Use it where a bad
// combination is data rather than a programming error, as in a vocabulary assembled from a deck or a
// test.
func Registered() []Option {
	regMu.Lock()
	defer regMu.Unlock()
	return append([]Option(nil), registered...)
}
