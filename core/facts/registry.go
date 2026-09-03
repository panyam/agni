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
// binary composed in. Nothing reads it directly: DefaultRegistry composes it into a *Registry, and a
// Registry is immutable once built, so every read goes through a value a caller holds. That is the
// shape check.Catalog already has on the rule side (NewCatalog / DefaultCatalog / CatalogWith), and
// the reason it is worth copying is that composing in a KNOWN ORDER removes a class of problem rather
// than guarding against it: collisions are checked once, at the end, so a relation and a predicate
// clash whichever registered first, with no bidirectional check and no init-order reasoning.
//
// The registration half stays global because that is the overlay seam (C18): a private overlay
// blank-imports a package whose init calls RegisterRelation, and the engine is composed BY the
// overlay rather than coupled to one. That is a startup default, never mutated per run, which is
// exactly the carve-out C22 makes for ambient state.

// Projector derives a relation's rows from a Model. It runs once per fact base; an empty result is
// correct when the Model lacks the tier the relation needs (silent-by-construction, never
// fabricated). A projector DERIVES from the Model and never consults a second authoritative store
// (C8), which is what keeps the fact base a view rather than a copy.
type Projector func(check.Model) []Row

// SpecLibProjector derives rows from a whole datasheet spec library with NO design (WS10-010), the
// library-wide analogue of Projector, so a corpus can be searched without loading a board.
type SpecLibProjector func([]*parampb.PartSpec) []Row

// BuiltinFacts is the bulk payload a standard relation catalog installs. Schema is each relation's
// positional field layout; Catalog is the human-facing picker metadata; Model and SpecLib are the
// design-scoped and library-wide projectors; Doc resolves a relation's reference markdown ("" when it
// has none).
//
// Bulk, not per-relation: a single relation is the OVERLAY shape (one at a time, namespaced), while
// the built-ins arrive as one payload because their projector is a single monolithic pass and several
// share it. The rule side made the same call (check.RegisterBuiltins).
type BuiltinFacts struct {
	Schema  map[string][]Field
	Catalog []RelationInfo
	Model   Projector
	SpecLib SpecLibProjector
	Doc     func(name string) string
}

// Relation is one overlay-supplied relation as a value: the name a query writes, its positional
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
	schema   map[string][]Field // every relation, built-in and overlay alike
	overlay  []Relation         // in composition order, so a merge is deterministic
	builtin  BuiltinFacts
	reserved map[string]string // predicate name -> the engine that claimed it
}

// An Option contributes to a Registry under construction. The three kinds mirror the three ways a
// relation vocabulary is assembled: a bulk built-in payload, one overlay relation, and an engine
// claiming names for predicates it computes itself.
type Option func(*builder)

type builder struct {
	builtin    BuiltinFacts
	hasBuiltin bool
	overlay    []Relation
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

// WithRelation adds one overlay-supplied relation.
//
// This is the open-core seam generalized to the fact layer: the public engine ships the built-in
// relations, and a private overlay contributes its OWN — house part attributes, a compliance
// database, an approved-vendor feed — without editing the engine. A registered relation is a
// first-class citizen of every query surface with no evaluator change, because an engine treats every
// relation uniformly (name -> field layout -> rows).
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
			b.overlay = append(b.overlay, Relation{Name: name, Fields: append([]Field(nil), fields...), Project: project})
		}
	}
}

// Reserving claims names for an engine's own computed predicates (core/query's reaches and the string
// filters), so a relation cannot shadow one. who names the claimant, for the error message.
//
// Order does not matter, and that is the point of composing rather than accumulating. Every option is
// applied first and collisions are swept once at the end, so a predicate and a relation clash
// whichever registered first. An engine and a relation catalog are independent imports whose init
// order is not controllable, so a check that ran at registration time had to look in both directions
// to cover the same ground.
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
// than the first: a caller fixing a composition wants the whole list, not one per rebuild.
func NewRegistry(opts ...Option) (*Registry, error) {
	b := &builder{reserved: map[string]string{}}
	for _, o := range opts {
		o(b)
	}
	r := &Registry{schema: map[string][]Field{}, builtin: b.builtin, reserved: b.reserved}
	for name, f := range b.builtin.Schema {
		r.schema[name] = append([]Field(nil), f...)
	}
	for _, rel := range b.overlay {
		if _, dup := r.schema[rel.Name]; dup {
			b.err(fmt.Errorf("relation %q is registered twice", rel.Name))
			continue
		}
		r.schema[rel.Name] = rel.Fields
		r.overlay = append(r.overlay, rel)
	}
	// One collision sweep, after everything is in. Sorted so the error reads the same on every run
	// (map iteration order would otherwise reshuffle it).
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

// SchemaOf resolves a relation's positional layout. An overlay relation resolves exactly like a
// built-in one, which is what lets an engine treat the two the same.
func (r *Registry) SchemaOf(rel string) ([]Field, bool) {
	f, ok := r.schema[rel]
	return f, ok
}

// IsRelation reports whether a name is a fact-base relation, so an engine can refuse to let a derived
// rule redefine one.
func (r *Registry) IsRelation(rel string) bool { _, ok := r.schema[rel]; return ok }

// Schema returns every relation's layout as a copy. It exists for the drift guard that asserts the
// catalog covers the schema: a relation that is queryable but undiscoverable is what that catches,
// and catching it needs the whole set rather than one lookup.
func (r *Registry) Schema() map[string][]Field {
	out := make(map[string][]Field, len(r.schema))
	for name, f := range r.schema {
		out[name] = append([]Field(nil), f...)
	}
	return out
}

// Installed reports whether this registry carries any relation at all.
//
// It is the difference between "this design has no such facts" and "no relations were ever
// installed", which a fact base otherwise flattens into the same empty result — and an engine whose
// relations are all missing answers nothing while looking exactly like one whose query matched
// nothing. That reads as a clean pass on a design nobody checked.
func (r *Registry) Installed() bool { return len(r.schema) > 0 }

// Rows projects the whole fact base for a design: the built-in relations first, then each overlay
// relation in composition order.
//
// An overlay row is stamped with the name it was REGISTERED under, overriding whatever the projector
// put in Row.Relation. The registration name is the one a query writes, so letting a projector name
// its own rows would let a relation answer under a name nothing registered, and silently shadow
// another. The built-in payload is a single pass over many relations, so its rows carry their own
// Relation and are taken as given.
func (r *Registry) Rows(m check.Model) []Row {
	var out []Row
	if r.builtin.Model != nil {
		out = append(out, r.builtin.Model(m)...)
	}
	for _, rel := range r.overlay {
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

// Relations returns the discoverable relation set: the built-ins plus each overlay relation, the
// latter with argument labels synthesized from its field layout. Order is composition order and is
// not sorted here — a caller that also has predicates to show merges both lists and sorts once.
//
// Each entry's Detail is resolved through the doc resolver, so an undocumented relation still lists
// with its Summary.
func (r *Registry) Relations() []RelationInfo {
	out := make([]RelationInfo, 0, len(r.builtin.Catalog)+len(r.overlay))
	out = append(out, r.builtin.Catalog...)
	for _, rel := range r.overlay {
		args := make([]string, len(rel.Fields))
		for i, f := range rel.Fields {
			args[i] = f.Label()
		}
		out = append(out, RelationInfo{Name: rel.Name, Args: args, Summary: "overlay-registered relation", Kind: KindOverlay})
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

// The registration buffer: the only package state. Written at init, read only by RegistryWith.

var (
	regMu      sync.Mutex
	registered []Option
)

// RegisterBuiltinFacts installs a bulk relation catalog into the process default. A relation catalog
// package calls it from its init, so any binary that blank-imports that package has the relations.
func RegisterBuiltinFacts(bf BuiltinFacts) { addOption(WithBuiltins(bf)) }

// RegisterRelation adds an overlay-supplied relation to the process default. Call it once at init.
func RegisterRelation(name string, fields []Field, project Projector) {
	addOption(WithRelation(name, fields, project))
}

// Reserve claims names for an engine's computed predicates in the process default.
func Reserve(who string, names ...string) { addOption(Reserving(who, names...)) }

// addOption validates the option against everything registered so far and panics if the combination
// cannot compose, before appending.
//
// It composes to check rather than inspecting the option, because the buffer is APPEND-ONLY: a bad
// registration cannot be taken back, so admitting one poisons every later DefaultRegistry and the
// failure then surfaces at some unrelated caller's first query. Validating on the way in keeps the
// buffer always-composable and puts the error at the registration that created the conflict.
//
// Order-dependence disappears with it. A relation registered before the catalog that owns its name
// composes cleanly at the time; the conflict is caught when the CATALOG registers, because that is
// the moment both parties are present. Either way the panic names the second one to arrive.
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
// silently at query time — the same contract as check.DefaultCatalog.
func DefaultRegistry() *Registry { return RegistryWith() }

// RegistryWith composes the registered options followed by the caller's extras, under the same
// checks. It is how an embedder adds relations explicitly rather than through the global seam, and
// how a test builds a registry that owes nothing to what the test binary happened to import.
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
// own and handle a composition error rather than take RegistryWith's panic. That is the difference
// between a binary's own vocabulary (a programming error if it does not compose) and one assembled
// from a deck or a test, where a bad combination is data and wants an error.
func Registered() []Option {
	regMu.Lock()
	defer regMu.Unlock()
	return append([]Option(nil), registered...)
}
