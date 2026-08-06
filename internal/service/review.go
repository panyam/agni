package service

import (
	"context"
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
)

// ReviewLoader is the narrow file surface ReviewService needs: the netlist design, an optional
// separate board export (WS3-089), and the checklist manifest — all mount-scoped by the impl. It is a
// subset of the fat service.Loader plus Manifest, defined at the point of use so the service depends
// only on what it reads (the fat Loader stays the design/render services' contract).
type ReviewLoader interface {
	Design(ctx context.Context, mount, path string, opts ...ReadOption) (*ir.Design, error)
	Board(ctx context.Context, mount, path string) (*geom.BoardGeometry, error)
	Manifest(ctx context.Context, mount, path string) (review.Manifest, error)
}

// ReviewService runs a review checklist manifest over one or more designs, the transport-neutral
// analogue of `agni review` (WS9-047) and the check family's aggregation surface. The catalog, the
// profile presence index, and the datasheet provider are design-INDEPENDENT and injected once
// (composed from serve's --profile-path / --intent-path / --params, exactly as CheckService receives
// its catalog and --params specs); only the Model and its presence/scope closures are per design. It
// knows no transport (C13).
type ReviewService struct {
	loader  ReviewLoader
	catalog *check.Catalog
	// byName is every profile (built-in + overlay) keyed by Name, for the interface-absence check that
	// marks a profile item not-applicable when its interface is absent from the design.
	byName map[string][]profiles.Profile
	// specs is the datasheet knowledge base the params join reads (WS10-003), nil when serve ran
	// without --params; NewModelWithParams guards a nil provider (no joined specs, no false pass).
	specs param.ParamProvider
}

// NewReviewService returns a ReviewService over the given loader, composed rule catalog, profile
// presence index, and optional datasheet provider (nil when no corpus is wired). The catalog and
// index are built once by the caller (serve/CLI) because they are design-independent.
func NewReviewService(loader ReviewLoader, catalog *check.Catalog, byName map[string][]profiles.Profile, specs param.ParamProvider) *ReviewService {
	return &ReviewService{loader: loader, catalog: catalog, byName: byName, specs: specs}
}

// RunReview runs the manifest against each requested design and returns a report per design: one
// design yields a per-item report, several yield a project rollup in request order. The run is
// all-or-nothing — a bad manifest, an unreadable design, or a board_path override at a file that
// carries no board geometry is an error — so a partial read never reports items clean without
// checking them (the same posture as a bad --params corpus failing the CLI run).
func (s *ReviewService) RunReview(ctx context.Context, req *webapi.RunReviewRequest) (*webapi.RunReviewResponse, error) {
	if len(req.GetDesignPath()) == 0 {
		return nil, fmt.Errorf("%w: RunReview needs at least one design_path", ErrInvalidArgument)
	}
	man, err := s.loader.Manifest(ctx, req.GetMount(), req.GetManifestPath())
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	// Per-request overlay config (WS3-102), composed BEFORE any design is read: its lexicon half has
	// to reach the read, since net roles are resolved at ingestion. An empty overlay leaves the
	// service's own catalog and the default vocabulary in place.
	ov, err := ComposeOverlay(req.GetOverlay())
	if err != nil {
		return nil, err
	}
	resp := &webapi.RunReviewResponse{Manifest: man.Name}
	for _, design := range req.GetDesignPath() {
		rep, err := s.runOne(ctx, req.GetMount(), design, req.GetBoardPath(), man, req.GetRatifiedFloor(), ov)
		if err != nil {
			return nil, err
		}
		resp.Reports = append(resp.Reports, reviewReportProto(rep))
	}
	return resp, nil
}

// runOne builds one design's Model (netlist + optional separate board tier + the shared params tier)
// and runs the manifest over it. The board tier is read from board_path, not the design, so a netlist
// entry can attach a separate confidential board export (WS3-089); an override that reads no board is
// an error, never a silent nil.
func (s *ReviewService) runOne(ctx context.Context, mount, design, boardPath string, man review.Manifest, floor float64, ov Overlay) (review.Report, error) {
	m, err := BuildModel(ctx, s.loader, mount, design, boardPath, s.specs, ov.ReadOptions()...)
	if err != nil {
		return review.Report{}, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return review.Report{}, err
	}
	present, scope, compScope := reviewClosures(m, s.byName)
	return review.Run(review.RunParams{
		Model: m, Catalog: cat, Manifest: man, Design: design,
		Present: present, Scope: scope, CompScope: compScope, RatifiedFloor: floor,
		// intent.Emits narrows the intent/ prefix to the compiler's actual name space, so a pre-bound
		// not-yet-shipped intent rule reads not-automated instead of a misleading needs-design-intent
		// (WS3-098). Injected to keep `review` decoupled from the `intent` package.
		IntentRuleKnown: intent.Emits,
	}), nil
}

// reviewClosures builds the presence and scope closures a review run needs over a design's Model and
// the profile index: present marks an item bound to a known-but-absent interface not-applicable, and
// scope/compScope filter a scoped binding's findings to the interface's nets/parts (WS3-058/083). The
// service owns this now that the CLI is a thin client of RunReview (WS9-048); it keeps `review`
// decoupled from `profiles`.
func reviewClosures(m check.Model, byName map[string][]profiles.Profile) (review.PresenceFunc, review.ScopeFunc, review.CompScopeFunc) {
	present := func(name string) (review.Presence, bool) {
		ps, ok := byName[name]
		if !ok {
			return review.IfaceAbsent, false // unknown interface: leave the item running
		}
		// The interface genuinely evaluates when a component declares its host, or when its signal
		// convention is in use AND the convention completeness rule can anchor — the same preconditions
		// the profile's rules apply (WS3-090, and its anchor half WS3-099). The host path does not need
		// the anchor: hostIncompleteRule anchors on the declared component instead.
		for _, p := range ps {
			if profiles.HostDeclared(m, p) || (profiles.InUse(m, p) && profiles.Anchored(m, p)) {
				return review.IfacePresent, true
			}
		}
		// In use but unanchored: the interface is visibly named to the convention, yet the completeness
		// rule has nothing to hang on. Neither absent nor checkable under this profile's naming (WS3-099).
		for _, p := range ps {
			if profiles.InUse(m, p) {
				return review.IfaceConventionUnmatched, true
			}
		}
		// Not strictly evaluable. A host-bound interface that IS named on the board (loose evidence) but
		// whose host is annotated nowhere and whose convention is not in use is host-unsatisfied — the
		// intended check is blocked, so not-automated. A profile with no such evidence is simply absent
		// (-> not-applicable); a genuinely-absent host-bound interface must NOT read not-automated.
		for _, p := range ps {
			if p.HasHost() && profiles.Named(m, p) {
				return review.IfaceHostUnsatisfied, true
			}
		}
		return review.IfaceAbsent, true
	}
	scope := func(name string) map[string]bool {
		out := map[string]bool{}
		for _, p := range byName[name] {
			for n := range profiles.Nets(m, p) {
				out[n] = true
			}
		}
		return out
	}
	compScope := func(name string) map[string]bool {
		out := map[string]bool{}
		for _, p := range byName[name] {
			for c := range profiles.Components(m, p) {
				out[c] = true
			}
		}
		return out
	}
	return present, scope, compScope
}

// reviewReportProto maps a review.Report to its wire form. Outcome is the review.Outcome string as-is
// (both the CLI and a panel key on it); the tally is derived by the consumer from the item outcomes,
// the same pure function review.Report.Tally() applies, so it is not carried on the wire. Findings
// reuse the one canonical FindingProto conversion CheckService uses.
func reviewReportProto(r review.Report) *webapi.ReviewReport {
	out := &webapi.ReviewReport{Manifest: r.Manifest, Design: r.Design}
	for _, ar := range r.Areas {
		area := &checkspb.ReviewArea{Name: ar.Area.Name}
		for _, it := range ar.Items {
			area.Items = append(area.Items, &checkspb.ReviewItem{
				Id:       it.Item.ID,
				Title:    it.Item.Title,
				Outcome:  string(it.Outcome),
				Note:     review.JoinNonEmpty(it.Note, it.Item.Note),
				Findings: FindingProtos(it.Findings),
			})
		}
		out.Areas = append(out.Areas, area)
	}
	return out
}
