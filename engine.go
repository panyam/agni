// Package agni composes the engine for a program that embeds Agni as a library rather than running
// the `agni` binary. One call produces the composed rule catalog, the relation registry, and the
// services that run over them.
//
// There are FOUR global registration hooks and three fail quietly when a binary misses one. Without
// stdlib/rules/builtin the catalog is empty, without stdlib/relations the fact base is, and either
// reports every design clean, so New refuses both at startup. See
// docsite/content/build/extending.md#compose-in-main.
//
// Every option takes a VALUE (a loaded profile set, a parsed declaration, an fs.FS), never a path.
// Reading files is the caller's job (C22, C13).
package agni

import (
	"errors"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/service"
	"github.com/panyam/agni/stdlib/profiles"
)

// Engine holds one rule catalog, one relation registry, and the project-resolution ports the
// services run against. New builds it once and nothing mutates it, so two surfaces built from one
// Engine agree about which rules are in effect.
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
// registration hook is unpopulated, returning MissingBuiltinsError or MissingRelationsError, each
// naming the import that fixes it.
//
// It returns an error where check.CatalogWith and facts.NewRegistry panic. Those run from an init or
// a composing main, where a startup panic is the convention; New runs inside an embedder's program,
// which has to decide for itself what to do about a bad composition.
func New(opts ...Option) (*Engine, error) {
	b := &builder{}
	for _, o := range opts {
		o(b)
	}
	registry := facts.RegistryWith(b.factOptions...)
	catalog, byName, err := composeRules(b.profiles, b.sources...)
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
// The check asks check.BuiltinRules rather than measuring the composed catalog. stdlib/profiles
// registers its own source from an init and this package imports it, so a program missing the
// built-ins still composes a NON-EMPTY catalog holding the interface-profile rules alone.
var MissingBuiltinsError = errors.New(
	`agni: the built-in rule catalog is not installed, so none of the shipped EE rules will run and a ` +
		`design is checked only against whatever this program composed itself. ` +
		`Add: import _ "github.com/panyam/agni/stdlib/rules/builtin"`)

// MissingRelationsError reports that no relation catalog was installed, so the fact base is empty
// and every datalog-authored rule matches nothing. Without this error the mistake neither fails to
// build nor errors at runtime.
var MissingRelationsError = errors.New(
	`agni: no fact relations are installed, so every datalog rule matches nothing and reports clean. ` +
		`Add: import _ "github.com/panyam/agni/stdlib/relations"`)

// checkSeams refuses the two compositions that would run clean rather than fail, and records the two
// that are legitimate choices as warnings.
//
// An empty catalog or an empty fact base makes every design look clean and no caller wants either.
// Shipping without the datalog rule suite or without an inline-query compiler is a real choice for an
// embedder, so those only warn. WithoutDatalogRules drops the first warning.
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
// import that would change it. An embedder may want the engine without one of these pieces, and
// nothing else reports their absence; an empty catalog or fact base gets an error from New instead.
func (e *Engine) Warnings() []string { return e.warnings }

// Catalog returns the composed rule catalog, holding the built-ins, every RegisterSource'd suite,
// and the profile, intent and ad-hoc sources the options supplied. Callers must not mutate the rules
// it holds.
func (e *Engine) Catalog() *check.Catalog { return e.catalog }

// Registry returns the composed relation registry, for a caller running its own queries through
// core/query's *From entry points rather than through a service.
func (e *Engine) Registry() *facts.Registry { return e.registry }

// ProfileIndex returns the by-name interface-profile index the review's absence gate reads. An
// interface counts as evaluating when any profile under its name is in the run, and the item scoped
// to it unions their nets.
//
// A caller running a review itself needs it, and it MUST come from the same call that built the
// catalog. A separately built index can silently disagree, clearing the gate on an interface whose
// rules the catalog dropped, so the item scores a clean pass on an interface nothing checked.
func (e *Engine) ProfileIndex() map[string][]profiles.Profile { return e.byName }

// ProjectResolver returns the resolver the rule-running services use to find a design's project and
// compose that project's config into a run. It is nil when no project store or config resolver was
// supplied, and the services accept nil by running on the engine's composed defaults.
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

// RuleLoader is the union of the loader interfaces CheckService and ReviewService take, so one call
// can build both without widening either service's own contract.
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

// RuleServices builds the two services that RUN rules from this Engine's one catalog, the
// CheckService behind a check panel and ListRules, and the ReviewService behind the review
// resources.
//
// They come back TOGETHER and the catalog is not a parameter, so a caller cannot hand the two
// surfaces different catalogs (WS3-048). A rule missing from the check panel's catalog looks exactly
// like a rule that ran and found nothing.
func (e *Engine) RuleServices(d RuleServiceDeps) (*service.CheckService, *service.ReviewService) {
	resolver := e.ProjectResolver()
	return service.NewCheckService(d.Loader, e.catalog, d.Specs, d.BaseConvention, d.Loader, resolver).WithProfileIndex(e.byName),
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
