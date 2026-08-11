package service

import (
	"context"

	"github.com/panyam/agni/datasheet/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/artifact"
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
	Config ProjectConfigLoader
}

// Overlay composes the config for one design: its project's where it has one, the fallback where it
// does not, and the request's own on top of either.
//
// The resolution failing is NOT an error. A malformed descriptor somewhere on a mount should not
// make an unrelated design unreadable, and the design still has the fallback to run against; the
// error surfaces where it is actionable, in the project surfaces themselves. What matters here is
// that a design which resolves to nothing gets nothing, so one project's rules cannot reach it.
func (r *ProjectResolver) Overlay(ctx context.Context, uri artifact.URI, req *webapi.OverlayConfig, fallback Overlay, baseConvention string) (Overlay, error) {
	var p *webapi.Project
	var d *webapi.Design
	// A caller asking for the built-in catalog is asking to be treated as though this design belonged
	// to no project, so the resolution simply does not happen. Filtering the config out afterwards
	// would be a second implementation of "no project" that could drift from the real one.
	if req.GetIgnoreProject() {
		return OverlayFor(ctx, nil, nil, nil, req, fallback, baseConvention)
	}
	if r != nil && r.Store != nil {
		if design, project, err := r.Store.ResolveDesign(ctx, uri); err == nil {
			p, d = project, design
		}
	}
	var loader ProjectConfigLoader
	if r != nil {
		loader = r.Config
	}
	return OverlayFor(ctx, loader, p, d, req, fallback, baseConvention)
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
