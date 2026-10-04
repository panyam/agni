package service

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/param"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// Overlay is one request's composed catalog configuration: the rule sources to splice onto the
// service's catalog, and the naming lexicon its design reads must be stamped with. Every surface
// that accepts overlay config composes it here, so they all compose identically (WS3-102).
//
// A convention carries rules AND a lexicon, and they land in different places. Rules extend the
// catalog, while the lexicon has to reach the READ, because net roles are resolved once at ingestion.
// Doing only the catalog half compiles the naming rules and leaves every OTHER rule blind to the
// project's rail names.
type Overlay struct {
	Sources []check.RuleSource
	Lexicon *classify.Lexicon
	// Specs is the datasheet corpus this run checks part limits against, nil when there is none. It
	// travels with the rule sources so a run cannot compose one team's rules against another's data.
	Specs param.ParamProvider
	// SymbolPaths are the symbol-library directories this run's config named, added to the read.
	SymbolPaths []string
	// Profiles and Intent record whether this overlay's Sources include a project's interface profiles
	// and a design's intent declaration. See ResolvedConfig for why the flags travel rather than being
	// derived from Sources.
	Profiles bool
	Intent   bool
	// DesignIntent is the declared intent itself, nil when none was declared. Its rules are already in
	// Sources, and the value travels too because part of it is read by the MODEL rather than by a rule:
	// which connectors are internal (agni issue 831).
	DesignIntent *configpb.DesignIntent
	// InterfaceProfiles are the profile VALUES behind the profile source in Sources: a project's own
	// profiles/ and any a request's config names. Running a profile needs only its compiled rules, and
	// these are for the surfaces that read a profile itself, the coverage panel and a review's
	// interface-presence gate, so they see the profiles the run's rules came from (agni issue 833).
	InterfaceProfiles []profiles.Profile
	// Library and LibraryDocs are the project's own derived relations and their pages, composed into
	// a query vocabulary by Registry.
	Library     []LibraryModule
	LibraryDocs map[string]string
	// conventionName is the source name of the convention THIS overlay carries, from a project or the
	// deployment default. A request-supplied convention replaces it by name (WS3-124).
	conventionName string
	// id accumulates the inputs this overlay was composed FROM, which is what Identity hashes. See
	// overlayidentity.go for why it is the inputs rather than the composed value.
	id *overlayID
	// baseConvention is the catalog source name of the SERVER's startup convention (`--conventions`),
	// empty when the caller composed no such default. Catalog drops it before splicing this request's
	// own, so a request-supplied convention overrides rather than stacks (WS3-124).
	//
	// Whoever built the base catalog supplies it, since only they know which source came from the
	// startup flag. The zero value replaces nothing, which is right for the CLI, whose catalog has no
	// startup convention.
	baseConvention string
}

// ComposeOverlay resolves an OverlayConfig into engine inputs. A nil or empty config yields a zero
// Overlay, which changes nothing, so a request that names no overlay behaves exactly as before.
//
// baseConvention is the catalog source name of the SERVER's startup convention, which this request's
// own convention replaces (WS3-124). Pass "" when the caller composed no such default, and nothing is
// replaced. It is a required parameter rather than an optional wither so that a call site forgetting
// it fails to compile instead of silently stacking conventions (WS3-102, WS3-109).
//
// It performs no I/O, since the config arrives as a value. An invalid convention (a pattern that will
// not compile, an unknown component class) is an ERROR, never a skip, because an operator who silently
// got the built-ins would read the clean report as a clean design.
func ComposeOverlay(cfg *webapi.OverlayConfig, baseConvention string) (Overlay, error) {
	o := Overlay{baseConvention: baseConvention}
	// Library modules sent as values compose with no I/O, like the convention below (agni issue 788).
	o.Library, o.LibraryDocs = inlineLibrary(cfg.GetConfig())
	conv := cfg.GetConfig().GetConventions()
	if conv == nil {
		return o, nil
	}
	lex, err := naming.BuildLexicon(conv)
	if err != nil {
		return Overlay{}, err
	}
	o.Lexicon = lex
	// Convention RULES are optional. A config carrying only a lexicon teaches the engine a project's
	// rail names without adding any naming rule.
	if len(conv.GetRules()) > 0 {
		src, err := naming.Source(conv)
		if err != nil {
			return Overlay{}, err
		}
		o.Sources = append(o.Sources, src)
	}
	return o, nil
}

// Catalog splices this overlay's rule sources ONTO base, returning base unchanged when the overlay
// carries none.
//
// It extends base rather than rebuilding a catalog (WS3-107). check.CatalogWith keeps the built-ins
// but drops base, and for a review base holds the --profile-path and --intent-path sources, so one
// naming rule in a convention disabled every interface profile and the whole intent tier. Measured on
// one design, 19 items went pass -> needs-design-intent and 16 went pass -> not-automated.
//
// A request's convention REPLACES the server's startup one rather than stacking on it (WS3-124).
// Catalog composition tags every rule with its source name (check.KeySource), so the replacement
// drops the rules carrying baseConvention's name and nothing else. The built-ins and the profile and
// intent sources are not the request's to remove. A caller that names no base convention (the CLI)
// keeps the additive behaviour.
//
// A name collision returns an error rather than panicking, since the sources come from a REQUEST.
func (o Overlay) Catalog(base *check.Catalog) (*check.Catalog, error) {
	if len(o.Sources) == 0 {
		return base, nil
	}
	if o.baseConvention != "" {
		base = base.Without(check.Facets{Tags: map[string][]string{check.KeySource: {o.baseConvention}}})
	}
	out, err := base.With(o.Sources...)
	if err != nil {
		return nil, o.explainCollision(err)
	}
	return out, nil
}

// explainCollision tells the caller that the source a duplicate-source error collided with may have
// come from a server flag. The caller sees only their own convention, so the bare error reads as
// though they sent it twice.
func (o Overlay) explainCollision(err error) error {
	if !strings.Contains(err.Error(), "duplicate rule source") {
		return err
	}
	return fmt.Errorf("%w (a source of that name is already composed into this server's catalog, "+
		"from --conventions or --profile-path; rename the convention, or name it exactly as the "+
		"server's --conventions to replace it)", err)
}

// ProfileIndex is the set of interface profiles this run's rules came from, keyed by name: base (the
// built-ins and a deployment's --profile-path, already composed) with this overlay's own profiles
// REPLACING a base profile of the same name, as their compiled source supersedes its rules. A
// surface reading a profile rather than running it uses this, so it never describes an interface by
// a definition the run did not check (agni issue 833). base is not modified.
func (o Overlay) ProfileIndex(base map[string][]profiles.Profile) map[string][]profiles.Profile {
	out := make(map[string][]profiles.Profile, len(base))
	for name, ps := range base {
		out[name] = ps
	}
	if base == nil {
		for _, p := range profiles.Profiles {
			out[p.Name] = append(out[p.Name], p)
		}
	}
	replaced := map[string]bool{}
	for _, p := range o.InterfaceProfiles {
		if !replaced[p.Name] {
			out[p.Name] = nil
			replaced[p.Name] = true
		}
		out[p.Name] = append(out[p.Name], p)
	}
	return out
}

// ReadOptions is what the overlay contributes to each design READ: the naming lexicon, the symbol
// paths, and the datasheet device-class lookup, each only when present.
func (o Overlay) ReadOptions() []ReadOption {
	var opts []ReadOption
	if o.Lexicon != nil {
		opts = append(opts, WithLexicon(o.Lexicon))
	}
	if len(o.SymbolPaths) > 0 {
		opts = append(opts, WithSymbolPaths(o.SymbolPaths))
	}
	if o.Specs != nil {
		opts = append(opts, WithDeviceClasses(DeviceClassLookup(o.Specs)))
	}
	if o.DesignIntent != nil {
		opts = append(opts, WithDesignIntent(o.DesignIntent))
	}
	return opts
}

// DeviceClassLookup adapts a param provider to the one question the ingestion class pass asks of it:
// the device_class a seeded spec states for an MPN. readers/formats takes this function and never the
// provider, so the datasheet layer stays out of the readers.
//
// A nil provider returns a nil function rather than panicking later. A caller that composed no corpus
// holds a nil interface, which looks the same as an empty corpus at the call site.
func DeviceClassLookup(specs param.ParamProvider) func(string) string {
	if specs == nil {
		return nil
	}
	return func(mpn string) string {
		if mpn == "" {
			return ""
		}
		return specs.Lookup(mpn).GetDeviceClass()
	}
}

// ConfigResolver turns the ref-shaped tiers of an AnalysisConfig into the engine inputs a run needs.
// It is the port that keeps file I/O out of a service (C13), since a config names its profiles and
// parameters as URIs and only an adapter can read them.
//
// It resolves ANY AnalysisConfig, a request's as well as a project's, so the two cannot drift apart.
// Which config is being resolved shows up only as the namespace. A tier the config does not name is a
// ZERO VALUE, never an error, because most configs declare only some tiers.
type ConfigResolver interface {
	// ResolveConfig loads what cfg's URIs point at.
	//
	// namespace is the catalog source name a profile set is registered under. The caller passes it
	// because a project's config is namespaced by its id and a request's has no id, and two projects on
	// one server would otherwise contribute rule sources of the same name.
	ResolveConfig(ctx context.Context, cfg *webapi.AnalysisConfig, namespace string) (ResolvedConfig, error)
}

// ResolvedConfig is what one AnalysisConfig contributes to a run, as engine values.
type ResolvedConfig struct {
	// Sources are the catalog extensions it supplies: interface profiles, a design's intent.
	Sources []check.RuleSource
	// Specs is the seeded datasheet corpus, nil when there is none. A nil provider is legal and means
	// the datasheet-backed rules read needs-data rather than failing.
	Specs param.ParamProvider
	// SymbolPaths are the resolved symbol-library directories, as host paths the reader can search.
	SymbolPaths []string
	// Profiles and Intent record WHICH tiers the Sources came from, since a compiled interface profile
	// and a compiled intent declaration are both just rules in a catalog. A results document states
	// which tiers were attached. See
	// docsite/content/architecture/checks-contract.md#provenance-is-read-off-the-resolved-overlay.
	Profiles bool
	Intent   bool
	// InterfaceProfiles are the profile values compiled into Sources, nil when the config names none.
	// See Overlay.InterfaceProfiles.
	InterfaceProfiles []profiles.Profile
	// Library is the project's own derived relations (agni issue 773), one module per file of a
	// library directory, and LibraryDocs their optional reference pages keyed by member path. A query
	// run under this config reads them through Overlay.Registry.
	Library     []LibraryModule
	LibraryDocs map[string]string
	// Digest identifies the BYTES this resolution read, so an overlay composed from it can say
	// whether two runs saw the same config. Empty means this resolver does not report one, which is
	// legal and costs the overlay its identity (see Overlay.Identity).
	Digest string
}

// configNeedsResolver reports whether cfg names anything only an adapter can read.
//
// A host with no resolver (the engine in WASM, a service built without one) can still honour a config
// carrying only a resolved convention, because that composes with no I/O. A config naming a URI is
// an error there, on the same terms GetNamingConvention refuses a stored convention, because
// dropping the tier would report a clean run against config that never loaded.
func configNeedsResolver(cfg *webapi.AnalysisConfig) bool {
	return len(cfg.GetProfileUris()) > 0 || len(cfg.GetParamUris()) > 0 ||
		len(cfg.GetSymbolPathUris()) > 0 || len(cfg.GetLibraryUris()) > 0
}

// OverlayFor composes the engine inputs for one design: the project's config where the design
// resolves to one, and nothing but the request's where it does not.
//
// A design that resolves to NO project gets no project config, so it cannot be checked against
// another project's rules (#180). The motivating bug is on
// docsite/content/architecture/projects-and-designs.md#three-tiers-of-configuration.
//
// baseConvention is as on ComposeOverlay. A REQUEST's own overlay wins over the project's.
func OverlayFor(ctx context.Context, resolver ConfigResolver, store ProjectStore, p *webapi.Project, d *webapi.Design, req *webapi.OverlayConfig, baseConvention string) (Overlay, error) {
	// Seeded with every input this call can see. What a resolver reads is folded in below, where it is
	// read, because only the resolver knows what it opened.
	id := &overlayID{}
	id.add("base-convention", []byte(baseConvention))
	id.addProto("request", req)
	id.addProto("project", p)
	id.addProto("design", d)
	if p == nil && d == nil {
		return overlayWithRequest(ctx, resolver, req, Overlay{}, baseConvention, id)
	}
	// A design in no project still has config of its own, its symbol library and its intent, which
	// compose here exactly as they do under a project, with nothing inherited (agni issue 887). A
	// design dropped into the browser rarely comes with a project.yaml.
	//
	// The project's config and the design's intent resolve TOGETHER, as one AnalysisConfig, so a run
	// cannot compose one design's intent against another's profiles. What the project inherits is
	// layered in first, so `merged` is its whole config and not only its own descriptor's.
	inherited, err := ResolveExtends(ctx, store, p)
	if err != nil {
		return Overlay{}, err
	}
	merged := mergeConfig(inherited, d.GetConfig())
	// The request and project protos do not carry the INHERITED config, which comes from another
	// project's descriptor through `extends`.
	id.addProto("inherited-config", merged)
	var o Overlay
	if resolver != nil {
		cfg, err := resolver.ResolveConfig(ctx, merged, configNamespace(p, d))
		if err != nil {
			return Overlay{}, err
		}
		o.Sources, o.Specs, o.Profiles, o.Intent = cfg.Sources, cfg.Specs, cfg.Profiles, cfg.Intent
		o.InterfaceProfiles = cfg.InterfaceProfiles
		o.SymbolPaths = cfg.SymbolPaths
		o.Library, o.LibraryDocs = cfg.Library, cfg.LibraryDocs
		id.addDigest("project-config", cfg.Digest)
	} else if configNeedsResolver(merged) {
		return Overlay{}, fmt.Errorf("%w: %s declares config this deployment cannot resolve (no config resolver wired)", ErrInvalidArgument, configNamespace(p, d))
	}
	// The design's intent arrives as a value and compiles here, with no I/O, on any host.
	if src, err := intentSource(merged, d.GetName()); err != nil {
		return Overlay{}, err
	} else if src != nil {
		o.Sources = append(o.Sources, src)
		o.Intent = true
		o.DesignIntent = merged.GetIntent()
	}
	// The project's convention arrives resolved, so its lexicon and rules compose with no I/O.
	if conv := inherited.GetConventions(); conv != nil {
		projectOv, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conv}}, baseConvention)
		if err != nil {
			return Overlay{}, err
		}
		o.Lexicon = projectOv.Lexicon
		o.Sources = append(o.Sources, projectOv.Sources...)
		o.conventionName = conv.GetName()
	}
	o.baseConvention = baseConvention
	return overlayWithRequest(ctx, resolver, req, o, baseConvention, id)
}

// projectNamespace is the catalog source name a project's profiles are registered under.
//
// It is the project's resource name so that two projects on one server do not collide.
func projectNamespace(p *webapi.Project) string { return p.GetName() }

// configNamespace is the namespace a design's composed config resolves under: its project's, or the
// design's own resource name when it is in no project.
func configNamespace(p *webapi.Project, d *webapi.Design) string {
	if p != nil {
		return projectNamespace(p)
	}
	return d.GetName()
}

// requestNamespace is the source name a REQUEST's profiles are registered under.
//
// It must differ from any project's, because a request's profiles layer ON TOP of the project's and
// two sources sharing a name would collide. A reader seeing `request-profiles/…` in a catalog
// snapshot knows the rule came from the call and not from the project.
const requestNamespace = "request"

// mergeConfig layers b over a, field by field.
//
// It layers field by field because a Project sets everything but intent and a Design sets only
// intent and its own symbols, so a whole-message replace would make a design declaring intent drop
// its project's profiles. The convention is not merged here, because it layers by replacement along
// an extends chain (ResolveExtends) and a design never sets one.
func mergeConfig(a, b *webapi.AnalysisConfig) *webapi.AnalysisConfig {
	out := &webapi.AnalysisConfig{
		Conventions:    a.GetConventions(),
		ProfileUris:    append(append([]string{}, a.GetProfileUris()...), b.GetProfileUris()...),
		ParamUris:      append(append([]string{}, a.GetParamUris()...), b.GetParamUris()...),
		SymbolPathUris: append(append([]string{}, a.GetSymbolPathUris()...), b.GetSymbolPathUris()...),
		LibraryUris:    append(append([]string{}, a.GetLibraryUris()...), b.GetLibraryUris()...),
		Checklists:     mergeChecklists(a.GetChecklists(), b.GetChecklists()),
		Intent:         a.GetIntent(),
	}
	if b.GetConventions() != nil {
		out.Conventions = b.GetConventions()
	}
	if b.GetIntent() != nil {
		out.Intent = b.GetIntent()
	}
	return out
}

// mergeChecklists layers b's checklists over a's by name. One of the same name replaces the earlier
// in place, so an inherited default stays first unless the nearer project redeclares it, and a new
// name is appended after the ones inherited.
func mergeChecklists(a, b []*webapi.NamedChecklist) []*webapi.NamedChecklist {
	out := append([]*webapi.NamedChecklist{}, a...)
	for _, c := range b {
		replaced := false
		for i := range out {
			if out[i].GetName() == c.GetName() {
				out[i], replaced = c, true
				break
			}
		}
		if !replaced {
			out = append(out, c)
		}
	}
	return out
}

// intentSource compiles a config's declared intent into its rule source, nil when it declares none.
// name labels the declaration in findings, and is the design's resource name where a project store
// supplied the intent.
//
// An invalid declaration is an ERROR, never a skip, because a design whose intent silently failed to
// compile would leave every intent-bound checklist item reading needs-design-intent, which looks like
// a design that never declared any.
func intentSource(cfg *webapi.AnalysisConfig, name string) (check.RuleSource, error) {
	di := cfg.GetIntent()
	if di == nil {
		return nil, nil
	}
	if name == "" {
		name = "request"
	}
	decl, err := intent.FromProto(path.Base(name), di)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return intent.Source("intent", decl), nil
}

// overlayWithRequest lets a request's own config override whatever it was layered on.
func overlayWithRequest(ctx context.Context, resolver ConfigResolver, req *webapi.OverlayConfig, base Overlay, baseConvention string, id *overlayID) (Overlay, error) {
	reqOv, err := ComposeOverlay(req, baseConvention)
	if err != nil {
		return Overlay{}, err
	}
	// The request's REF-shaped tiers resolve through the same port a project's do and layer ON TOP of
	// what the project contributed. A convention is different and replaces, below, because it answers
	// for the whole naming vocabulary.
	reqCfg := req.GetConfig()
	var reqResolved ResolvedConfig
	if configNeedsResolver(reqCfg) {
		if resolver == nil {
			return Overlay{}, fmt.Errorf("%w: this deployment cannot resolve config refs (profiles, parameters, intent); send a resolved convention or run against a host that can", ErrInvalidArgument)
		}
		reqResolved, err = resolver.ResolveConfig(ctx, reqCfg, requestNamespace)
		if err != nil {
			return Overlay{}, err
		}
		id.addDigest("request-config", reqResolved.Digest)
	}
	reqIntent, err := intentSource(reqCfg, "")
	if err != nil {
		return Overlay{}, err
	}
	if reqIntent != nil {
		reqOv.Sources = append(reqOv.Sources, reqIntent)
	}
	// Every tier a request can contribute must appear in this guard. A tier missing from it is silently
	// dropped for a request carrying ONLY that tier, which is how symbol paths usually arrive.
	if reqOv.Lexicon == nil && len(reqOv.Sources) == 0 && len(reqResolved.Sources) == 0 &&
		reqResolved.Specs == nil && len(reqResolved.SymbolPaths) == 0 && len(reqResolved.Library) == 0 &&
		len(reqOv.Library) == 0 && len(reqOv.LibraryDocs) == 0 {
		base.id = id
		return base, nil
	}
	// A request convention REPLACES whatever this overlay already carried, project or deployment
	// (WS3-124), lexicon and rules alike.
	out := base
	if reqOv.Lexicon != nil {
		out.Lexicon = reqOv.Lexicon
	}
	// Replacement is by source NAME. Keeping both would run two vocabularies at once, and when they
	// are the same file (--conventions naming the one the project already declares) it would be a
	// duplicate-source error.
	kept := make([]check.RuleSource, 0, len(base.Sources))
	for _, src := range base.Sources {
		if base.conventionName != "" && src.Name() == base.conventionName {
			continue
		}
		// A request's intent REPLACES the design's, as a convention does, because a design has one
		// intended architecture. Keeping both would compile two sources named intent, which the
		// catalog refuses.
		if reqIntent != nil && src.Name() == intent.SourceName {
			continue
		}
		kept = append(kept, src)
	}
	if reqIntent != nil {
		out.DesignIntent = reqCfg.GetIntent()
	}
	out.Sources = append(append(kept, reqResolved.Sources...), reqOv.Sources...)
	// A request corpus WINS over the project's rather than merging, so one team's transcribed limits
	// never decide another's pass/fail.
	if reqResolved.Specs != nil {
		out.Specs = reqResolved.Specs
	}
	// Symbol paths ACCUMULATE, since a request naming a library adds somewhere to look.
	out.SymbolPaths = append(append([]string{}, out.SymbolPaths...), reqResolved.SymbolPaths...)
	// A request's library modules ACCUMULATE too, and a path both define is refused when Registry
	// composes them, as any two definers of one path are.
	out.Library = append(append(append([]LibraryModule{}, out.Library...), reqResolved.Library...), reqOv.Library...)
	out.LibraryDocs = mergeDocs(mergeDocs(out.LibraryDocs, reqResolved.LibraryDocs), reqOv.LibraryDocs)
	out.Profiles = out.Profiles || reqResolved.Profiles
	out.InterfaceProfiles = append(append([]profiles.Profile{}, out.InterfaceProfiles...), reqResolved.InterfaceProfiles...)
	out.Intent = out.Intent || reqResolved.Intent || reqIntent != nil
	out.conventionName = req.GetConfig().GetConventions().GetName()
	// Set the base convention's NAME explicitly, since Overlay.Catalog drops the sources tagged with it.
	// Inheriting whatever the base overlay held would leave the server's convention running alongside the
	// request's (WS3-124). Only a request that SENDS a convention replaces the server's. One carrying
	// only intent, profiles or library modules keeps it, or its rules would vanish from a run that
	// never asked to change the naming vocabulary.
	out.baseConvention = ""
	if req.GetConfig().GetConventions() != nil {
		out.baseConvention = baseConvention
	}
	out.id = id
	return out, nil
}
