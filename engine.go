// Package agni composes the engine. It is the entry point for a program that embeds Agni as a
// library rather than running the `agni` binary: one call produces the composed rule catalog, the
// composed relation registry, and the services that run over them.
//
// It exists because composing correctly means getting FOUR independent global registration seams
// right, and three of them fail quietly when a binary misses one. A program that forgets
// stdlib/rules/builtin has an empty catalog, one that forgets stdlib/relations has an empty fact
// base, and either reports every design clean. New refuses both rather than running, so the
// composition mistake surfaces at startup instead of as a green report on a design nobody checked.
//
// Files are NOT this package's business. Every option takes a VALUE (a loaded profile set, a parsed
// declaration, an fs.FS), never a path, because configuration travels as a value and reading it is
// the caller's world (C22, C13). The CLI reads its flags and hands the results here; an embedder
// reads its own config however it likes and does the same.
package agni

import (
	"errors"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/profiles"
)

// Engine is a composed engine: one rule catalog, one relation registry, and the project-resolution
// ports the services run against. It is built once by New and never mutated, so two surfaces built
// from one Engine cannot disagree about which rules are in effect.
type Engine struct {
	catalog  *check.Catalog
	registry *facts.Registry
	byName   map[string][]profiles.Profile
	store    service.ProjectStore
	config   service.ConfigResolver
	resolver *service.ProjectResolver
	env      service.ReviewEnv
	warnings []string
}

// New composes an Engine from the registered built-ins plus the options given. It fails when a
// composition seam is unpopulated, because every one of those failures is otherwise a clean report
// on an unchecked design; see MissingBuiltinsError and MissingRelationsError for the two it can
// return and what import fixes each.
//
// It returns an error where check.CatalogWith and facts.NewRegistry panic. The divergence is
// deliberate: those two are called from an init or from a composing main, where a panic at process
// start is the standard-library convention and the caller is the programmer who made the mistake.
// New is called by an embedder whose own program has to decide what to do about a bad composition,
// and a library that panics inside a host's startup path takes that decision away.
func New(opts ...Option) (*Engine, error) {
	b := &builder{}
	for _, o := range opts {
		o(b)
	}
	registry := facts.RegistryWith(b.factOptions...)
	catalog, byName, err := composeRules(b.profiles, b.intent, b.sources...)
	if err != nil {
		return nil, err
	}
	e := &Engine{
		catalog:  catalog,
		registry: registry,
		byName:   byName,
		store:    b.store,
		config:   b.config,
		resolver: b.resolver,
		env: service.ReviewEnv{
			ProducerVersion: b.version,
			Profiles:        len(b.profiles) > 0,
			Intent:          b.intent != nil,
		},
	}
	if err := e.checkSeams(b); err != nil {
		return nil, err
	}
	return e, nil
}

// MissingBuiltinsError reports that the built-in rule source was never installed, so the shipped EE
// rule catalog is absent and a run reports only whatever the caller composed itself.
//
// It asks check.BuiltinRules rather than measuring the composed catalog, which would not catch this:
// stdlib/profiles registers its own source from an init and this package imports it, so a program
// missing the built-ins still composes a NON-EMPTY catalog holding the interface-profile rules
// alone. A size check would pass while every rule the engine is known for was missing.
var MissingBuiltinsError = errors.New(
	`agni: the built-in rule catalog is not installed, so none of the shipped EE rules will run and a ` +
		`design is checked only against whatever this program composed itself. ` +
		`Add: import _ "github.com/panyam/agni/stdlib/rules/builtin"`)

// MissingRelationsError reports that no relation catalog was installed, so the fact base is empty
// and every datalog-authored rule matches nothing. This is the failure examples/extension carried a
// hand-written warning comment about, since it neither fails to build nor errors at runtime.
var MissingRelationsError = errors.New(
	`agni: no fact relations are installed, so every datalog rule matches nothing and reports clean. ` +
		`Add: import _ "github.com/panyam/agni/stdlib/relations"`)

// checkSeams refuses the two compositions that would run clean rather than fail, and records the two
// that are legitimate choices as warnings.
//
// The split is not symmetric because the seams are not. An empty catalog and an empty fact base have
// no legitimate reading: nothing a caller could want produces either, and both make every design
// look clean. Shipping without the datalog rule suite, or without an inline-query compiler, is a
// real choice an embedder may make, so those are reported and not refused. WithoutDatalogRules is
// how a caller says the first one is deliberate and drops the warning.
func (e *Engine) checkSeams(b *builder) error {
	if !e.registry.Installed() {
		return MissingRelationsError
	}
	if len(check.BuiltinRules()) == 0 {
		return MissingBuiltinsError
	}
	if !b.noDatalog && !hasSource(e.catalog, datalogSourceName) {
		e.warnings = append(e.warnings, `no datalog-authored rule suite is installed, so the rules `+
			`authored in datalog rather than Go will not run. Add: import _ "github.com/panyam/agni/stdlib/rules/datalog", `+
			`or pass agni.WithoutDatalogRules() to say the absence is deliberate.`)
	}
	if !reviewQueryCompilerInstalled() {
		e.warnings = append(e.warnings, `no inline-query compiler is installed, so a review manifest `+
			`binding an item to an inline query will fail to load. Add: import _ "github.com/panyam/agni/stdlib/reviewquery"`)
	}
	return nil
}

// Warnings reports compositions that are legitimate but worth saying out loud, each naming the
// import that would change it. They are warnings rather than errors because an embedder may
// genuinely want the engine without one of these pieces; an empty catalog or an empty fact base
// gets an error from New instead. A caller that ignores the slice gets the behaviour it asked for,
// silently, which is the whole reason the slice exists.
func (e *Engine) Warnings() []string { return e.warnings }

// Catalog returns the composed rule catalog: the built-ins, every RegisterSource'd suite, and the
// profile, intent and ad-hoc sources the options supplied. Callers must not mutate the rules it
// holds.
func (e *Engine) Catalog() *check.Catalog { return e.catalog }

// Registry returns the composed relation registry, for a caller running its own queries through
// core/query's *From entry points rather than through a service.
func (e *Engine) Registry() *facts.Registry { return e.registry }

// ProfileIndex returns the by-name interface-profile index the review's absence gate reads: an
// interface counts as evaluating when any profile under its name is in the run, and the item scoped
// to it unions their nets.
//
// It is exposed because a caller running a review itself needs it, and it MUST come from the same
// call that built the catalog. An index built separately can disagree with the catalog about which
// profiles are in effect, and the disagreement is silent: the gate clears on an interface whose
// rules the catalog dropped, so an item scoped by it scores a clean pass on an interface nothing
// checked.
func (e *Engine) ProfileIndex() map[string][]profiles.Profile { return e.byName }

// ProjectResolver returns the resolver the rule-running services use to find a design's project and
// compose that project's config into a run. It is nil when no project store was supplied, which the
// services accept: a design that resolves to no project runs on the engine's composed defaults.
func (e *Engine) ProjectResolver() *service.ProjectResolver {
	if e.resolver != nil {
		return e.resolver
	}
	if e.store == nil && e.config == nil {
		return nil
	}
	return &service.ProjectResolver{Store: e.store, Config: e.config}
}

// ProjectService returns the project/design listing service, or nil when no store was supplied.
func (e *Engine) ProjectService() *service.ProjectService {
	if e.store == nil {
		return nil
	}
	return service.NewProjectService(e.store)
}

// RuleLoader is what the two rule-running services need between them. CheckService and
// ReviewService take different loader interfaces, so naming the intersection lets one call build
// both without widening either service's own contract.
type RuleLoader interface {
	service.Loader
	service.ReviewLoader
	service.ConventionLoader
}

// RuleServiceDeps is the per-deployment I/O a rule-running service needs and the Engine does not
// hold: where designs are read from, where stored runs live, the seeded datasheet corpus, and the
// name of the deployment's base naming convention.
type RuleServiceDeps struct {
	Loader         RuleLoader
	ReviewStore    service.ReviewStore
	Specs          param.ParamProvider
	BaseConvention string
}

// RuleServices builds the two services that RUN rules, from this Engine's one catalog: the
// CheckService behind a check panel and ListRules, and the ReviewService behind the review
// resources.
//
// They are returned TOGETHER, and the catalog is not a parameter, so a caller cannot hand one
// surface the composed catalog and the other something else. That drift is what this shape exists
// to prevent and it is not hypothetical: --profile-path reached both surfaces only after WS3-048,
// while --intent-path and a naming config's rules reached reviews alone. A rule missing from the
// check panel's catalog is indistinguishable there from a rule that ran and found nothing, so the
// disagreement is invisible from the outside.
func (e *Engine) RuleServices(d RuleServiceDeps) (*service.CheckService, *service.ReviewService) {
	resolver := e.ProjectResolver()
	return service.NewCheckService(d.Loader, e.catalog, d.Specs, d.BaseConvention, d.Loader, resolver),
		service.NewReviewService(d.Loader, d.ReviewStore, e.catalog, e.byName, d.Specs, e.env, d.BaseConvention, resolver)
}

// hasSource reports whether any rule in the catalog came from the named source, by the composed
// "<source>/<rule>" name the Catalog exposes.
func hasSource(c *check.Catalog, name string) bool {
	prefix := name + "/"
	for _, r := range c.Rules() {
		if len(r.Name) > len(prefix) && r.Name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
