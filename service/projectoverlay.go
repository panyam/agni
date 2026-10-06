package service

import (
	"context"
	"errors"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/timing"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// ProjectResolver is the two ports a rule-running surface needs to answer "whose config applies to
// this design", the store that maps an artifact to its project and the loader that turns that
// project's config into engine inputs.
//
// Every surface that runs rules needs both or neither, so they are one constructor parameter. Wired
// per surface, --conventions once reached `agni check` and not `agni review` (WS3-102, WS3-109).
//
// A nil resolver means this deployment resolves no projects, and every design runs as one in no
// project, as on a server started with no descriptors.
type ProjectResolver struct {
	Store  ProjectStore
	Config ConfigResolver
}

// Overlay composes the config for one design, its project's where it has one, and the request's own
// on top. A design in no project gets the request's config alone. The serve flags reach it another
// way, through the catalog and the datasheet provider the services hold and the process lexicon, so
// an overlay never carries them (agni issue 755).
//
// Finding NO descriptor is not an error, since a loose file belongs to no project, and neither is an
// unknown mount. A descriptor that EXISTS and does not parse is
// returned as an error, as ResolveDesign reports it. Swallowing it ran the design against the
// built-in vocabulary, which on one folder gave 40 findings the project's lexicon would not have
// raised and missed 95 it would have (#307). A malformed descriptor for an UNRELATED design still
// does not surface here. See docsite/content/architecture/projects-and-designs.md#resolution-is-an-interface-not-a-path-convention.
func (r *ProjectResolver) Overlay(ctx context.Context, uri artifact.URI, req *webapi.OverlayConfig, baseConvention string) (Overlay, error) {
	defer timing.Begin(ctx, "resolve.overlay")()
	var p *webapi.Project
	var d *webapi.Design
	// A caller asking for the built-in catalog is treated as though the design belonged to no
	// project, so resolution is skipped rather than its result filtered out afterwards.
	if req.GetIgnoreProject() {
		return OverlayFor(ctx, nil, nil, nil, nil, req, baseConvention)
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
	return OverlayFor(ctx, resolver, store, p, d, req, baseConvention)
}

// Checklists returns the checklists a design's project declares, inherited ones included in the order
// ResolveExtends composes them, and the project's name. A design in no project, or a deployment that
// resolves none, returns (nil, "", nil), and a project declaring none returns (nil, name, nil), so a
// caller can tell an operator which fix applies. A descriptor that exists and does not parse is an
// error, as for Overlay.
func (r *ProjectResolver) Checklists(ctx context.Context, uri artifact.URI) ([]*webapi.NamedChecklist, string, error) {
	if r == nil || r.Store == nil {
		return nil, "", nil
	}
	_, p, err := r.Store.ResolveDesign(ctx, uri)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, "", nil
	case err != nil:
		return nil, "", err
	case p == nil:
		return nil, "", nil
	}
	cfg, err := ResolveExtends(ctx, r.Store, p)
	if err != nil {
		return nil, "", err
	}
	return cfg.GetChecklists(), p.GetName(), nil
}

// PickChecklist returns the checklist called name, or the first, which is the project's default, when
// name is empty. It returns nil when there is none.
func PickChecklist(lists []*webapi.NamedChecklist, name string) *webapi.NamedChecklist {
	for _, c := range lists {
		if name == "" || c.GetName() == name {
			return c
		}
	}
	return nil
}

// DeclaredChecklists names what a project does declare, for a message saying it lacks the one asked
// for.
func DeclaredChecklists(lists []*webapi.NamedChecklist) string {
	if len(lists) == 0 {
		return "which declares none"
	}
	names := make([]string, 0, len(lists))
	for _, c := range lists {
		names = append(names, c.GetName())
	}
	return "which declares " + strings.Join(names, ", ")
}

// RunProvenance is which config tiers a run actually had attached, the value a results document's
// RunConfig records.
//
// The answer has two sources. A deployment composes its startup flags into the service's own catalog
// and specs, and a project composes its own onto the request. A document built from only the first
// reports `params: false` for a run scored against a project's seeded corpus.
type RunProvenance struct {
	Params      bool
	Profiles    bool
	Intent      bool
	Conventions string
}

// Provenance reports what a run under this overlay actually had attached, meaning this overlay's own
// tiers unioned with the deployment defaults the caller composed into its catalog and specs.
//
// Union rather than override, because the two stack for every tier except the convention. A server
// started with --profile-path serving a project that also declares profiles runs BOTH. The convention
// is already resolved here, since a request-supplied one replaces whatever was in place (WS3-124), so
// conventionName wins and the deployment's name is only the fallback.
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

// SpecsOver returns the datasheet corpus this run should use: the project's own layered over the
// shared one (--params or --params-url) per MPN, so the project decides every part it seeds and the
// shared corpus answers for the rest (agni issue 749). Each citation names the corpus it came from,
// because a limit transcribed outside the project now decides some of its verdicts, and a reader has
// to be able to see which. DECISIONS.md records the reversal from the project replacing the shared
// corpus outright. nil when neither exists.
func (o Overlay) SpecsOver(shared param.ParamProvider) param.ParamProvider {
	if o.Specs == nil && shared == nil {
		return nil
	}
	return param.Layered{{Name: param.CorpusProject, Provider: o.Specs}, {Name: param.CorpusShared, Provider: shared}}
}

// Sources resolves an artifact ref to the artifact each tier should read, applying the enclosing
// design's declaration when there is one.
//
// This is the served counterpart of what `cmd/agni` does around `ResolveSources`. Without it a
// netlist entry drew an auto-layout on the server while the CLI drew the declared companion's real
// sheets (agni issue 656, C32).
//
// A nil resolver, a resolver with no store, and a ref belonging to no declared design all yield the
// ref in every tier. That is the ordinary case for a mounted folder, not an error.
//
// isDir is derived rather than stat'ed (C13), because a design's URI IS its directory (`fsstore`
// sets it from the descriptor's folder), so only a ref equal to it names the design.
//
// asNamed comes off the request, because the CLI is a client of these services and its --as-named
// asks for the file it named.
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
// It is the one call a service makes to learn which artifact each tier should open. Resolving here
// rather than in the loader keeps the loader a reader of what it is handed, so a caller can read a
// companion as a netlist by naming it.
//
// boardOverride is the request's own board_uri and it WINS over the design's declaration, matching
// `--board-path` on the CLI. A zero override, which is nearly every call, leaves the declared board.
func (r *ProjectResolver) TierURIs(ctx context.Context, u artifact.URI, boardOverride artifact.URI, asNamed bool) (netlist, board, geometry artifact.URI, err error) {
	defer timing.Begin(ctx, "resolve.tiers")()
	src, err := r.Sources(ctx, u, asNamed)
	if err != nil {
		return u, boardOverride, u, err
	}
	if netlist, err = ParseArtifactURI(src.NetlistURI); err != nil {
		return u, boardOverride, u, err
	}
	if geometry, err = ParseArtifactURI(src.GeometryURI); err != nil {
		return u, boardOverride, u, err
	}
	board = boardOverride
	if board.IsZero() && src.BoardURI != src.NetlistURI {
		// Only when the design declared a SEPARATE board. Otherwise stay zero so BuildModel reads
		// the netlist artifact for copper when it carries any and errors on a non-board override.
		if board, err = ParseArtifactURI(src.BoardURI); err != nil {
			return u, boardOverride, u, err
		}
	}
	return netlist, board, geometry, nil
}
