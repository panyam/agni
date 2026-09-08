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

// Sources resolves an artifact ref to the artifact each tier should read, applying the enclosing
// design's declaration when there is one.
//
// This is the served counterpart of what `cmd/agni` does around `ResolveSources`, and it exists
// because nothing on this side called it: every geometry decision keyed off the URI as handed in, so
// a netlist entry yielded no faithful geometry and the design fell back to an auto-layout while the
// CLI drew the real sheets from the declared companion (agni issue 656, constraint C32).
//
// A nil resolver, a resolver with no store, and a ref belonging to no declared design all yield the
// ref in every tier. That is the ordinary case for a mounted folder, not an error, which is why the
// store's own miss is (nil, nil, nil).
//
// isDir is derived rather than stat'ed, because a design's URI IS its directory (`fsstore` sets it
// from the descriptor's folder), so a ref equal to it names the design and nothing else can. The
// service reads no filesystem of its own (C13), and this is what lets it decide without one.
//
// asNamed comes off the request. It has to be expressible, because the CLI is itself a client of
// these services and carries the same flag: resolving here unconditionally would silently override
// a caller that asked for the file it named.
func (r *ProjectResolver) Sources(ctx context.Context, uri artifact.URI, asNamed bool) (Resolution, error) {
	ref := uri.String()
	plain := Resolution{DesignSources: DesignSources{NetlistURI: ref, BoardURI: ref, GeometryURI: ref}}
	if r == nil || r.Store == nil {
		return plain, nil
	}
	d, _, err := r.Store.ResolveDesign(ctx, uri)
	if err != nil {
		return plain, err
	}
	return ResolveSources(d, ref, d != nil && ref == d.GetUri(), asNamed), nil
}

// TierURIs is Sources with the refs parsed back into artifact URIs, which is what every read takes.
//
// It is the one call a service makes to learn which artifact each of its tiers should open. Doing it
// per service rather than inside the loader keeps the loader a reader of what it is handed, which is
// what lets a caller deliberately read a companion as a netlist by naming it.
//
// boardOverride is the request's own board_uri, and it WINS over the design's declaration, matching
// `--board-path` on the CLI: a caller who named a board is answering the question the descriptor
// would otherwise answer. A zero override leaves the declared board in place, which is the case that
// was broken, since the request field is empty on nearly every call.
func (r *ProjectResolver) TierURIs(ctx context.Context, u artifact.URI, boardOverride artifact.URI, asNamed bool) (netlist, board, geometry artifact.URI, err error) {
	src, err := r.Sources(ctx, u, asNamed)
	if err != nil {
		return u, boardOverride, u, err
	}
	if netlist, err = artifactURI(src.NetlistURI); err != nil {
		return u, boardOverride, u, err
	}
	if geometry, err = artifactURI(src.GeometryURI); err != nil {
		return u, boardOverride, u, err
	}
	board = boardOverride
	if board.IsZero() && src.BoardURI != src.NetlistURI {
		// Only when the design declared a SEPARATE board. Leaving it zero otherwise preserves
		// BuildModel's own rule, which reads the netlist artifact for copper when it carries any and
		// treats a non-board override as a loud error.
		if board, err = artifactURI(src.BoardURI); err != nil {
			return u, boardOverride, u, err
		}
	}
	return netlist, board, geometry, nil
}
