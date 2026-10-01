package check

// RuleSource is one origin of rules, such as the built-ins, an embedder's Go suite or a design's
// own rule file. Nothing downstream distinguishes where a rule came from (WS3-006). Name is the
// source's namespace. The empty name is reserved for the built-ins, whose rule names pass through
// bare, and every other source's rules are exposed as "<name>/<rule>" so an external suite cannot
// silently shadow a built-in.
type RuleSource interface {
	Name() string
	Rules() []*Rule
}

// SupersedingSource is an OPTIONAL capability a RuleSource may also implement, for a source that
// REPLACES rules another source contributed instead of running alongside them. Composition
// type-asserts for it, because widening RuleSource would break every out-of-module suite.
//
// Each returned Facets selects superseded rules with Filter's vocabulary. Names supersedes
// individual rules (a house rule replacing one built-in) and Tags a whole family (every rule of one
// interface profile, via the "profile" tag).
//
// A source's declaration never applies to its own rules. See applySupersessions for why.
type SupersedingSource interface {
	RuleSource
	Supersedes() []Facets
}

// Supersession records that one source's rules were dropped from a composed catalog because another
// source superseded them. The Catalog keeps these so a surface can SAY what it suppressed, since a
// silently dropped rule turns a false failure into an invisible gap.
type Supersession struct {
	By    string   // Name() of the superseding source
	Rules []string // composed names of the rules that were dropped, in catalog order
}

// NewSupersedingSource wraps a fixed rule slice as a named source that supersedes every rule matching
// any of the given Facets, for when the superseded set is known at construction.
func NewSupersedingSource(name string, rules []*Rule, supersedes ...Facets) SupersedingSource {
	return supersedingSource{fixedSource{name: name, rules: rules}, supersedes}
}

type supersedingSource struct {
	fixedSource
	supersedes []Facets
}

func (s supersedingSource) Supersedes() []Facets { return s.supersedes }

// Builtins is the built-in rule set as a RuleSource, and the only anonymous source. It reads the
// set RegisterBuiltins installed at call time. Without stdlib/rules/builtin imported the set is
// empty and the engine runs only the sources that ARE registered.
var Builtins RuleSource = builtins{}

type builtins struct{}

func (builtins) Name() string   { return "" }
func (builtins) Rules() []*Rule { return builtinRules }

// builtinRules and builtinSpecs are the standard rule catalog and its declarative-twin map,
// installed by stdlib/rules/builtin at init through RegisterBuiltins. They are empty in a
// program that does not import that package.
var (
	builtinRules []*Rule
	builtinSpecs map[string]*Spec
)

// RegisterBuiltins installs the standard EE rule catalog as the anonymous built-in source, whose
// rules pass through the Catalog un-namespaced. stdlib/rules/builtin calls it from its init. rules
// is exposed through Builtins and BuiltinRules. specs holds the Go-eval'd rules' declarative twins
// (BuiltinSpecs), which the parity tests hold to their Go Eval. A second call replaces the set
// rather than appending, so the last import wins.
func RegisterBuiltins(rules []*Rule, specs map[string]*Spec) {
	builtinRules, builtinSpecs = rules, specs
}

// BuiltinRules returns the installed built-in rule set (empty when stdlib/rules/builtin is not
// imported). Callers must not mutate the returned slice.
func BuiltinRules() []*Rule { return builtinRules }

// BuiltinSpecs returns the built-in rules' declarative-twin map (empty when stdlib/rules/builtin
// is not imported). Callers must not mutate the returned map.
func BuiltinSpecs() map[string]*Spec { return builtinSpecs }

// NewSource wraps a fixed rule slice as a named RuleSource, for an embedder's
// suite or a test source. The name becomes the namespace prefix and must match the Catalog's
// source-name grammar, lowercase [a-z0-9-]+.
func NewSource(name string, rules []*Rule) RuleSource {
	return fixedSource{name: name, rules: rules}
}

type fixedSource struct {
	name  string
	rules []*Rule
}

func (s fixedSource) Name() string   { return s.name }
func (s fixedSource) Rules() []*Rule { return s.rules }

// registeredSources are the out-of-module rule suites added via RegisterSource. The built-in
// set is NOT here, since Builtins is composed first on its own.
var registeredSources []RuleSource

// RegisterSource adds a rule source to the process-global registry, so a suite living in another
// module is picked up by the CLI and serve (both compose DefaultCatalog / CatalogWith) with no
// re-wiring. It is the rule-side twin of formats.Register (WS12-004). Call it from an init or the
// composing binary's main, and the rules appear in ListRules and CheckDesign as "<source>/<rule>".
//
// RegisterSource panics on a nil source, an anonymous source, a name outside [a-z0-9-]+, or a
// duplicate source name, as image.RegisterFormat and sql.Register do. The deeper checks (no "/" in a
// rule name, no duplicate composed names) run when DefaultCatalog / CatalogWith builds.
func RegisterSource(s RuleSource) {
	if s == nil {
		panic("check: RegisterSource(nil)")
	}
	name := s.Name()
	switch {
	case name == "":
		panic("check: RegisterSource: an external source must be named (the empty name is reserved for the built-ins)")
	case !sourceNameRe.MatchString(name):
		panic("check: RegisterSource: source name " + name + " must match [a-z0-9-]+")
	}
	for _, existing := range registeredSources {
		if existing.Name() == name {
			panic("check: RegisterSource: duplicate source name " + name)
		}
	}
	registeredSources = append(registeredSources, s)
}

// RegisteredSources returns the sources added via RegisterSource, in registration order (the
// built-ins are not included). Callers must not mutate the returned slice.
func RegisteredSources() []RuleSource {
	return registeredSources
}
