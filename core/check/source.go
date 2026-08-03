package check

// RuleSource is one origin of rules: the built-ins, an embedder's Go suite, a design's own
// rule file, or (later) the Phase-2 DSL compiler's output — all register through this one
// seam, so nothing downstream distinguishes where a rule came from (WS3-006). Name is the
// source's namespace: the empty name is reserved for the built-in source, whose rule names
// pass through bare; every other source's rules are exposed by the Catalog as
// "<name>/<rule>" so an external suite cannot silently shadow a built-in.
type RuleSource interface {
	Name() string
	Rules() []*Rule
}

// Builtins is the built-in rule set as a RuleSource — the default (and only anonymous)
// source. It reads the Rules registry at call time, so rules registered by package init
// are always included.
var Builtins RuleSource = builtins{}

type builtins struct{}

func (builtins) Name() string   { return "" }
func (builtins) Rules() []*Rule { return Rules }

// NewSource wraps a fixed rule slice as a named RuleSource: the one-liner for an embedder's
// suite or a test source. The name becomes the namespace prefix; it must match the Catalog's
// source-name grammar (lowercase [a-z0-9-]+).
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
// set is NOT here — it is Builtins, composed first — so this holds only the extra sources an
// embedder contributes.
var registeredSources []RuleSource

// RegisterSource adds a rule source to the process-global registry, so a suite living in
// another module — house-style or proprietary rules in the open-core overlay — is picked up by
// the engine's own surfaces (the CLI and serve both compose DefaultCatalog / CatalogWith) with
// no re-wiring. This is the rule-side twin of the reader registry's formats.Register (WS12-004):
// an embedder calls it from an init or the composing binary's main and its rules appear in
// ListRules and run in CheckDesign, namespaced "<source>/<rule>" so they can never shadow a
// built-in.
//
// RegisterSource panics on a nil source, an anonymous source (the empty name is reserved for the
// built-ins), a name outside [a-z0-9-]+, or a duplicate source name — all programming errors
// surfaced at process start, matching the standard library's registry convention
// (image.RegisterFormat, sql.Register) and formats.Register. A source is registered once; the
// deeper composition checks (no "/" in a rule name, no duplicate composed names) stay with the
// Catalog and surface when DefaultCatalog / CatalogWith builds.
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
// built-ins are not included). It is the introspection hook a catalog builder or a test uses;
// callers must not mutate the returned slice.
func RegisteredSources() []RuleSource {
	return registeredSources
}
