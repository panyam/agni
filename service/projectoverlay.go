package service

import (
	"context"
	"errors"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ProjectResolver is the two ports a rule-running surface needs to answer "whose config applies to
// this design": the store that maps an artifact to its project, and the loader that turns that
// project's config into engine inputs.
//
// They travel together because neither is useful alone, and because every surface that runs rules
// needs both or none. Bundling them makes that a single constructor parameter rather than two, which
// matters more than it looks: --conventions once reached `agni check` and not `agni review` because
// each knob was wired per surface, and a surface that forgets one of a pair is exactly that bug
// again (WS3-102, WS3-109).
//
// A nil resolver means this deployment resolves no projects. Every design then falls back to the
// caller's default, which is how a server started with no descriptors behaves — and how the CLI
// behaved before projects existed at all.
type ProjectResolver struct {
	Store  ProjectStore
	Config ConfigResolver
}

// Overlay composes the config for one design: its project's where it has one, the fallback where it
// does not, and the request's own on top of either.
//
// Finding NO descriptor is not an error: a loose file genuinely belongs to no project, and it runs
// against the fallback. A descriptor that EXISTS and does not parse is a different thing, and it is
// returned.
//
// The distinction is the whole point, and it used to be flattened. "A malformed descriptor somewhere
// on a mount should not make an unrelated design unreadable" is sound, but it was implemented by
// discarding every resolution error, which also swallowed the descriptor governing THIS design. A
// run then composed against the built-in vocabulary and reported findings that looked authoritative:
// on one folder that was 40 findings the project's own lexicon would not have raised and 95 it would
// have. ResolveDesign already tells the two apart — absent is (nil, nil, nil), malformed is an error
// — so honoring that costs nothing and keeps the unrelated-neighbour case tolerant.
//
// This matches how the rest of the config tiers already fail: a malformed overlay profile or
// conventions file fails the run with a teaching error rather than being silently skipped.
func (r *ProjectResolver) Overlay(ctx context.Context, uri artifact.URI, req *webapi.OverlayConfig, fallback Overlay, baseConvention string) (Overlay, error) {
	var p *webapi.Project
	var d *webapi.Design
	// A caller asking for the built-in catalog is asking to be treated as though this design belonged
	// to no project, so the resolution simply does not happen. Filtering the config out afterwards
	// would be a second implementation of "no project" that could drift from the real one.
	if req.GetIgnoreProject() {
		return OverlayFor(ctx, nil, nil, nil, nil, req, fallback, baseConvention)
	}
	if r != nil && r.Store != nil {
		design, project, err := r.Store.ResolveDesign(ctx, uri)
		switch {
		case err == nil:
			p, d = project, design
		case errors.Is(err, ErrNotFound):
			// Nothing to resolve against (an unknown mount). Same as having no descriptor.
		default:
			return Overlay{}, err
		}
	}
	var resolver ConfigResolver
	if r != nil {
		resolver = r.Config
	}
	var store ProjectStore
	if r != nil {
		store = r.Store
	}
	return OverlayFor(ctx, resolver, store, p, d, req, fallback, baseConvention)
}

// RunProvenance is which config tiers a run actually had attached, the value a results document's
// RunConfig records.
//
// It exists because that question has two sources and either alone gives a wrong answer. A deployment
// composes its startup flags into the service's own catalog and specs; a project composes its own onto
// the request. A document built from only the first reports `params: false` for a run scored against a
// project's seeded corpus, which is the reassuring direction to be wrong in and exactly the failure the
// field was added to prevent.
type RunProvenance struct {
	Params      bool
	Profiles    bool
	Intent      bool
	Conventions string
}

// Provenance reports what a run under this overlay actually had attached: this overlay's own tiers
// unioned with the deployment defaults the caller composed into its catalog and specs.
//
// Union rather than override, because the two genuinely stack for every tier except the convention. A
// server started with --profile-path serving a project that also declares profiles ran BOTH, and a
// document claiming only one of them would misdescribe the catalog its own snapshot records. The
// convention is the exception and is already resolved by the time it gets here: a request-supplied one
// replaces whatever was in place (WS3-124), so conventionName is the single answer and the deployment's
// name is only the fallback when nothing replaced it.
func (o Overlay) Provenance(deployment RunProvenance) RunProvenance {
	p := RunProvenance{
		Params:      o.Specs != nil || deployment.Params,
		Profiles:    o.Profiles || deployment.Profiles,
		Intent:      o.Intent || deployment.Intent,
		Conventions: o.conventionName,
	}
	if p.Conventions == "" {
		p.Conventions = deployment.Conventions
	}
	return p
}

// RunConfigProto is the one place a RunProvenance becomes a results document's RunConfig, so the CLI's
// check path and the service's review path cannot describe the same tiers differently.
func RunConfigProto(p RunProvenance, ratifiedFloor float64) *checkspb.RunConfig {
	return &checkspb.RunConfig{
		Params:        p.Params,
		Profiles:      p.Profiles,
		Intent:        p.Intent,
		Conventions:   p.Conventions,
		RatifiedFloor: ratifiedFloor,
	}
}

// SpecsOr returns the datasheet corpus this run should use: the project's when it supplied one, the
// deployment's otherwise.
//
// The project WINS rather than merging, and that is the same rule the rest of this config follows. A
// merged corpus would let one team's transcribed limits decide another team's pass/fail, which is
// the class of cross-design leak this whole change exists to close — and a silent one, because a
// parameter that came from the wrong seed still produces a confident number.
func (o Overlay) SpecsOr(fallback param.ParamProvider) param.ParamProvider {
	if o.Specs != nil {
		return o.Specs
	}
	return fallback
}
