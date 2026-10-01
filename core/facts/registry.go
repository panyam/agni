package facts

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"

	"github.com/panyam/agni/core/check"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/jaala/ns"
	"github.com/panyam/jaala/stdlib"
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
	vocab     *ns.Vocabulary    // every queryable name at its path: relations, predicates, modules
	docs      map[string]string // reference markdown for names no relation catalog documents (WithDocs)
}

// An Option contributes to a Registry under construction. The three kinds mirror the three ways a
// relation vocabulary is assembled: a bulk built-in payload, one extension relation, and an engine
// claiming names for predicates it computes itself.
type Option func(*builder)

type builder struct {
	builtin    BuiltinFacts
	hasBuiltin bool
	extension  []Relation
	predicates []namedBuiltin
	modules    []ns.Module
	languages  []ns.Language
	docs       map[string]string
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

// namedBuiltin is a predicate waiting to join the vocabulary at its path.
type namedBuiltin struct {
	path string
	b    ns.Builtin
}

// WithPredicate adds a predicate at path: a filter or a generator over the fact base's Source, which
// reaches the design through EnvOf. A generator must emit only values drawn from the design, since
// evaluation terminates only while nothing invents values.
func WithPredicate(path string, b ns.Builtin) Option {
	return func(bd *builder) { bd.predicates = append(bd.predicates, namedBuiltin{path, b}) }
}

// WithModule adds a module of derived relations, text in a language some engine registered with
// WithLanguage. The fact layer stores the text and never parses it; the language reports what the
// module defines.
func WithModule(path, lang, text string) Option {
	return WithModules(ns.Module{Path: path, Language: lang, Text: text})
}

// WithModules adds several modules as one option, for a library whose modules read each other. A
// module's Origin, the file it was read from, is what an error about it names.
// Composition checks what every module reads against the whole vocabulary, so modules that refer
// across one another must arrive together; added one option at a time, the first would be checked
// before the member it reads exists. Each module's Members field is ignored, since the language
// reports them.
func WithModules(ms ...ns.Module) Option {
	return func(bd *builder) {
		for _, m := range ms {
			bd.modules = append(bd.modules, ns.Module{Path: m.Path, Language: m.Language, Text: m.Text, Origin: m.Origin})
		}
	}
}

// WithDocs supplies reference markdown for names the relation catalog does not document, keyed by
// path: a library's derived relations, which Registry.Doc then serves the way it serves a relation's
// doc. Composition refuses a doc for a path nothing defines, and two docs for one path, so a renamed
// member cannot leave its page behind.
func WithDocs(docs map[string]string) Option {
	return func(bd *builder) {
		if bd.docs == nil {
			bd.docs = map[string]string{}
		}
		for path, d := range docs {
			if _, dup := bd.docs[path]; dup {
				bd.err(fmt.Errorf("two docs supplied for %q", path))
				continue
			}
			bd.docs[path] = d
		}
	}
}

// WithLanguage makes a module language available to the vocabulary. An engine registers its own.
func WithLanguage(l ns.Language) Option {
	return func(bd *builder) { bd.languages = append(bd.languages, l) }
}

// NewRegistry composes a Registry from the given options. It reports every problem it found rather
// than only the first, so a caller fixing a composition sees the whole list.
func NewRegistry(opts ...Option) (*Registry, error) {
	b := &builder{}
	for _, o := range opts {
		o(b)
	}
	r := &Registry{schema: map[string][]Field{}, builtin: b.builtin}
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
	v, errs := r.compose(b)
	r.vocab = v
	b.errs = append(b.errs, errs...)
	if len(b.errs) > 0 {
		return nil, fmt.Errorf("facts: %w", errors.Join(b.errs...))
	}
	return r, nil
}

// compose builds the vocabulary: languages first, so modules can be read; then every relation in
// catalog order, typed from its catalog entry; the standard predicates; the registered predicates;
// then the modules. The vocabulary applies the namespace tree's rules as each name arrives (one
// definer per path; a segment is a module or a member), so a clash is reported whichever side
// registered first.
func (r *Registry) compose(b *builder) (*ns.Vocabulary, []error) {
	var errs []error
	v, err := ns.NewVocabulary(noCatalog{})
	if err != nil {
		return nil, []error{err}
	}
	for _, l := range b.languages {
		if err := v.AddLanguage(l); err != nil {
			errs = append(errs, err)
		}
	}
	added := map[string]bool{}
	addRel := func(name string) {
		if added[name] {
			return
		}
		added[name] = true
		s := ns.Schema{Arity: len(r.schema[name])}
		if info, ok := r.InfoOf(name); ok {
			s.Labels, s.Types, s.Doc = info.Args, ArgTypes(info.Args, info.ArgKinds), info.Summary
		}
		if err := v.AddRelation(name, s); err != nil {
			errs = append(errs, err)
		}
	}
	for _, info := range r.builtin.Catalog {
		if r.IsRelation(info.Name) {
			addRel(info.Name)
		}
	}
	rest := make([]string, 0, len(r.schema))
	for name := range r.schema {
		rest = append(rest, name)
	}
	sort.Strings(rest)
	for _, name := range rest {
		addRel(name)
	}
	if err := stdlib.Register(v); err != nil {
		errs = append(errs, err)
	}
	for _, p := range b.predicates {
		if err := v.AddPredicate(p.path, p.b); err != nil {
			errs = append(errs, err)
		}
	}
	for _, m := range b.modules {
		if err := v.AddModule(m.Path, m.Language, m.Text, m.Origin); err != nil {
			errs = append(errs, withOrigin(err))
		}
	}
	if len(b.modules) > 0 && len(errs) == 0 {
		if err := v.Check(); err != nil {
			errs = append(errs, withOrigin(err))
		}
	}
	for _, path := range slices.Sorted(maps.Keys(b.docs)) {
		if !v.Has(path) {
			errs = append(errs, fmt.Errorf("a doc is supplied for %q, which nothing defines", path))
		}
	}
	r.docs = b.docs
	return v, errs
}

// withOrigin names the file a module error is about. The vocabulary reports a failure one module is
// responsible for as an *ns.ModuleError carrying the module's origin, with a message that names only
// the module's path, which several modules can share; the origin is what tells a reader which file
// to fix.
func withOrigin(err error) error {
	var me *ns.ModuleError
	if errors.As(err, &me) && me.Origin != "" {
		return fmt.Errorf("%s: %w", me.Origin, err)
	}
	return err
}

// noCatalog is the Source an empty vocabulary starts from. It serves nothing and exists to say, in an
// unknown-name error, which import installs the relations, because a binary missing that import
// otherwise reads as a query that named something the design lacks (C29).
type noCatalog struct{}

func (noCatalog) Schema(string) (ns.Schema, bool) { return ns.Schema{}, false }
func (noCatalog) Tuples(string) []ns.Tuple        { return nil }
func (noCatalog) Relations() []string             { return nil }
func (noCatalog) NoVocabularyHint() string {
	return "no fact relations are installed (import a relation catalog, e.g. stdlib/relations)"
}

// Vocabulary is every name a query can call, at its path: the relations, the predicates and the
// modules. It is composed once with the registry and only read afterwards, so an engine can check it
// once and share it across every query.
func (r *Registry) Vocabulary() *ns.Vocabulary { return r.vocab }

// Predicates returns a catalog entry per predicate in the vocabulary, the standard ones included,
// sorted by path.
func (r *Registry) Predicates() []RelationInfo { return r.membersOf(ns.EntryPredicate, KindPredicate) }

// Derived returns a catalog entry per derived relation a module defines, with its signature as the
// modules' language worked it out and its definition, sorted by path. Empty when no module is
// registered, or when the modules do not check, which NewRegistry has already reported.
func (r *Registry) Derived() []RelationInfo { return r.membersOf(ns.EntryDerived, KindDerived) }

// membersOf walks the vocabulary's tree and returns a catalog entry for every member of one kind.
func (r *Registry) membersOf(want ns.EntryKind, kind string) []RelationInfo {
	var out []RelationInfo
	var walk func(module string)
	walk = func(module string) {
		entries, err := r.vocab.Members(module)
		if err != nil {
			return
		}
		for _, e := range entries {
			switch e.Kind {
			case ns.EntryModule:
				walk(e.Path)
			case want:
				info := RelationInfo{Name: e.Path, Summary: e.Doc, Kind: kind, Definition: e.Definition}
				for _, a := range e.Args {
					info.Args = append(info.Args, a.Name)
					if k := argKindOf(a.ArgType); k.Entity != "" || k.KindArg != "" || k.OwnerArg != "" || len(k.ValidOptions) > 0 {
						if info.ArgKinds == nil {
							info.ArgKinds = map[string]ArgKind{}
						}
						info.ArgKinds[a.Name] = k
					}
				}
				out = append(out, info)
			}
		}
	}
	if r.vocab != nil {
		walk("")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ArgTypes maps a catalog entry's per-argument declarations onto the vocabulary's signature, position
// by position: an entity kind, a kind taken per row from another argument, an owner argument, and a
// closed vocabulary of values.
func ArgTypes(labels []string, kinds map[string]ArgKind) []ns.ArgType {
	if len(kinds) == 0 {
		return nil
	}
	out := make([]ns.ArgType, len(labels))
	for i, l := range labels {
		k := kinds[l]
		out[i] = ns.ArgType{Kind: k.Entity, KindFrom: k.KindArg, Owner: k.OwnerArg, Domain: k.ValidOptions}
	}
	return out
}

func argKindOf(t ns.ArgType) ArgKind {
	return ArgKind{Entity: t.Kind, KindArg: t.KindFrom, OwnerArg: t.Owner, ValidOptions: t.Domain}
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

// Doc resolves a name's reference markdown, or "" when it has none: the relation catalog's doc for a
// relation, and a doc supplied with WithDocs for anything else, such as a library member.
func (r *Registry) Doc(name string) string {
	if r.builtin.Doc != nil {
		if d := r.builtin.Doc(name); d != "" {
			return d
		}
	}
	return r.docs[name]
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

// RegisterPredicate adds a predicate to the process default at path. Call it once at init.
func RegisterPredicate(path string, b ns.Builtin) { addOption(WithPredicate(path, b)) }

// RegisterDocs adds reference markdown for names the relation catalog does not document to the
// process default (see WithDocs). Call it once at init, after the names it documents register.
func RegisterDocs(docs map[string]string) { addOption(WithDocs(docs)) }

// RegisterModules adds a library of derived-relation modules to the process default, all at once so
// they may read each other in any order (see WithModules). Call it once at init per library.
func RegisterModules(ms ...ns.Module) { addOption(WithModules(ms...)) }

// RegisterLanguage makes a module language available in the process default. An engine calls it once
// at init for its own language.
func RegisterLanguage(l ns.Language) { addOption(WithLanguage(l)) }

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
//
// The composed registry is cached until something new registers. The registration buffer only grows
// and a Registry is never changed once built, so the cache cannot go stale unnoticed, and every caller
// shares one vocabulary, which an engine checks once rather than per query.
func DefaultRegistry() *Registry {
	regMu.Lock()
	if defaultCache.r != nil && defaultCache.n == len(registered) {
		r := defaultCache.r
		regMu.Unlock()
		return r
	}
	n := len(registered)
	regMu.Unlock()
	r := RegistryWith()
	regMu.Lock()
	if n == len(registered) {
		defaultCache.r, defaultCache.n = r, n
	}
	regMu.Unlock()
	return r
}

// defaultCache is DefaultRegistry's last composition and the buffer length it was composed from.
var defaultCache struct {
	n int
	r *Registry
}

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
