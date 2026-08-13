package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/rules/intent"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ErrReviewStoreNotConfigured is returned by every review resource method when the server was started
// without a review store. It is its own sentinel, like ErrNativeNotEnabled and ErrExtractNotEnabled,
// so the transport can map it to a failed-precondition rather than a generic invalid argument: the
// request was fine, the deployment is not configured to answer it.
var ErrReviewStoreNotConfigured = errors.New("no review store configured")

// ReviewLoader is the narrow file surface ReviewService needs: the netlist design, an optional
// separate board export (WS3-089), and a stored checklist manifest — all mount-scoped by the impl. It
// is a subset of the fat service.Loader plus Manifest, defined at the point of use so the service
// depends only on what it reads (the fat Loader stays the design/render services' contract).
//
// Manifest is deliberately NOT on the run path (WS9-050). CreateReview takes the checklist as a
// value, so it needs no loader at all to score a design; this method backs GetReviewManifest alone,
// which exists so a client that only holds a ref can obtain that value. The refs are keys the impl
// resolves inside a mount, never host paths the service may interpret (C22).
type ReviewLoader interface {
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
	Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error)
	Manifest(ctx context.Context, uri artifact.URI) (review.Manifest, error)
	// DesignHash returns "sha256:<hex>" over the design's bytes, the revision identity a stored run
	// records so a document is never silently re-read against a design that has since changed. It is on
	// the loader because hashing means reading bytes, and this package does no I/O (C13/C22).
	//
	// An unreadable design is NOT an error here: the run itself already succeeded, so failing a whole
	// create over a provenance field would be worse than a document that honestly records no hash.
	// Return ("", nil) in that case, which DesignRef.content_hash explicitly allows.
	DesignHash(ctx context.Context, uri artifact.URI) (string, error)
}

// ReviewEnv is the deployment-level provenance a stored run records: which overlay tiers were
// composed into the catalog this service holds, and what build produced the document.
//
// It is injected rather than derived because the service genuinely cannot see it. By the time a
// catalog reaches here, an overlay profile is just another rule in it, so "were profiles attached"
// is unanswerable from the catalog alone. RunConfig exists precisely so a reader can tell a design
// with no datasheet violations from a run that had no datasheet corpus attached, and a service that
// guessed those flags would produce documents that lie about what was evaluable.
type ReviewEnv struct {
	// ProducerVersion is the engine build identity (version.Version() at the entrypoint).
	ProducerVersion string
	// Profiles reports that --profile-path overlay profiles were composed into the catalog.
	Profiles bool
	// Intent reports that a --intent-path design-intent declaration was composed into the catalog.
	Intent bool
}

// ReviewService runs a review checklist manifest over one or more designs, the transport-neutral
// analogue of `agni review` (WS9-047) and the check family's aggregation surface. The catalog, the
// profile presence index, and the datasheet provider are design-INDEPENDENT and injected once
// (composed from serve's --profile-path / --intent-path / --params, exactly as CheckService receives
// its catalog and --params specs); only the Model and its presence/scope closures are per design. It
// knows no transport (C13).
type ReviewService struct {
	// projects resolves a design to its project and loads that project's config; nil when this
	// deployment declares none. fallback is the deployment default used for a design with no project.
	projects *ProjectResolver
	fallback Overlay
	loader   ReviewLoader
	catalog  *check.Catalog
	// byName is every profile (built-in + overlay) keyed by Name, for the interface-absence check that
	// marks a profile item not-applicable when its interface is absent from the design.
	byName map[string][]profiles.Profile
	// specs is the datasheet knowledge base the params join reads (WS10-003), nil when serve ran
	// without --params; NewModelWithParams guards a nil provider (no joined specs, no false pass).
	specs param.ParamProvider
	// store persists runs (WS9-053). Nil means no store was configured, and the four resource methods
	// then report that rather than half-working: a create that ran the checks and dropped the result
	// would be the worst of both, since it costs the full sweep and leaves nothing behind.
	store ReviewStore
	env   ReviewEnv
	// baseConvention is the catalog source name of the deployment's --conventions default, "" when
	// there is none. Same role it plays on CheckService: a request's own convention replaces it.
	baseConvention string
}

// NewReviewService returns a ReviewService over the given loader, review store, composed rule
// catalog, profile presence index, and optional datasheet provider (nil when no corpus is wired). The
// catalog and index are built once by the caller (serve/CLI) because they are design-independent.
//
// store may be nil, which disables the review resource methods; pass a MemReviewStore for a caller
// that wants runs to work without persisting them, which is what `agni review` does. baseConvention
// names the startup convention a request-supplied one replaces; "" when the catalog carries none.
func NewReviewService(loader ReviewLoader, store ReviewStore, catalog *check.Catalog, byName map[string][]profiles.Profile, specs param.ParamProvider, env ReviewEnv, baseConvention string, projects *ProjectResolver) *ReviewService {
	return &ReviewService{loader: loader, store: store, catalog: catalog, byName: byName, specs: specs, env: env, baseConvention: baseConvention, projects: projects}
}

// reviewStore returns the configured store or an error naming the flag that configures it. Every
// resource method goes through it, so the "not configured" message is written once and a deployment
// that forgot the volume gets told which flag it forgot rather than a nil dereference.
func (s *ReviewService) reviewStore() (ReviewStore, error) {
	if s.store == nil {
		return nil, ErrReviewStoreNotConfigured
	}
	return s.store, nil
}

// CreateReview runs the checklist against the design and persists the result (WS9-053). The run is
// all-or-nothing — an invalid manifest, an unreadable design, or a board_ref at a file that carries
// no board geometry is an error — so a partial read never reports items clean without checking them
// (the same posture as a bad --params corpus failing the CLI run). Nothing is stored when the run
// fails, so the store never accumulates half-answers.
//
// The checklist arrives as a VALUE (WS9-050), so the RUN itself needs no filesystem: every input is
// either in the request or was injected at construction. That is also why the manifest is validated
// here rather than trusted. A manifest that never passed through review.Load has had no parser
// enforce its rules, and an item carrying two mutually-exclusive bindings would otherwise score a
// design against a check its author did not ask for.
//
// The document it stores is the same self-contained CheckResults `agni review --results-out` writes,
// carrying the checklist SNAPSHOT rather than its name, so re-rendering the run later reproduces what
// was actually asked instead of whatever the checklist file says by then.
func (s *ReviewService) CreateReview(ctx context.Context, req *webapi.CreateReviewRequest) (*webapi.Review, error) {
	parent, err := reviewParent(req.GetParent())
	if err != nil {
		return nil, err
	}
	designURI, err := artifactURI(req.GetDesignUri())
	if err != nil {
		return nil, err
	}
	boardURI, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	store, err := s.reviewStore()
	if err != nil {
		return nil, err
	}
	if req.GetDesignUri() == "" {
		return nil, fmt.Errorf("%w: CreateReview needs a design_ref", ErrInvalidArgument)
	}
	if req.GetManifest() == nil {
		return nil, fmt.Errorf("%w: CreateReview needs a manifest (resolve a stored one with GetReviewManifest)", ErrInvalidArgument)
	}
	man := ManifestFromProto(req.GetManifest())
	if err := review.Validate(man); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	// Per-request overlay config (WS3-102), composed BEFORE the design is read: its lexicon half has
	// to reach the read, since net roles are resolved at ingestion. An empty overlay leaves the
	// service's own catalog and the default vocabulary in place.
	ov, err := s.projects.Overlay(ctx, designURI, req.GetOverlay(), s.fallback, s.baseConvention)
	if err != nil {
		return nil, err
	}
	rep, cat, err := s.runOne(ctx, designURI, boardURI, man, req.GetRatifiedFloor(), ov)
	if err != nil {
		return nil, err
	}
	// The hash is provenance, not a precondition: a design that ran but cannot be re-read still
	// produced real outcomes, so an unreadable source records no hash rather than failing the create.
	hash, err := s.loader.DesignHash(ctx, designURI)
	if err != nil {
		hash = ""
	}
	doc := &checkspb.CheckResults{
		Meta: &checkspb.ResultsMeta{
			Schema:          results.Schema,
			Producer:        results.Producer,
			ProducerVersion: s.env.ProducerVersion,
			// A native run records what it could NOT check as well as what it found: a rule whose fact
			// tier is absent reads not-applicable, and a review item that did not evaluate never reads
			// pass. That is the axis an imported vendor report does not have.
			CoverageAxis: true,
		},
		Design: &checkspb.DesignRef{Source: designURI.String(), ContentHash: hash},
		// Provenance comes off the RESOLVED overlay, not off this service's startup config. The run used
		// ov.SpecsOr(s.specs) and ov.Catalog(s.catalog), so reading the flags back from s.specs/s.env
		// described the deployment rather than the run: a design in a project that declares params/,
		// profiles/ and conventions.yaml scored against all three and recorded `run: {}`, while the same
		// document's own catalog snapshot listed the project's rules. A reader comparing two runs would
		// have concluded the corpus was never attached.
		Run: RunConfigProto(ov.Provenance(RunProvenance{
			Params:      s.specs != nil,
			Profiles:    s.env.Profiles,
			Intent:      s.env.Intent,
			Conventions: req.GetOverlay().GetConfig().GetConventions().GetName(),
		}), req.GetRatifiedFloor()),
		// The catalog snapshot is the one composed for THIS run, overlay included, not the service's
		// base: a reader has to see the rules that actually ran, or a per-request convention's rules
		// would be missing from the record of a run they shaped.
		Catalog:          results.RuleRecords(cat.Rules()),
		Manifest:         man.Name,
		ManifestSnapshot: req.GetManifest(),
		Areas:            reviewAreaProtos(rep),
	}
	name, createdAt, err := store.Create(ctx, parent, doc)
	if err != nil {
		return nil, err
	}
	doc.Meta.CreatedAt = createdAt
	return &webapi.Review{Name: name, Results: doc}, nil
}

// GetReview returns a stored run. It reads only the store: the document is self-contained, so neither
// the design nor the checklist it was about needs to still exist.
func (s *ReviewService) GetReview(ctx context.Context, req *webapi.GetReviewRequest) (*webapi.Review, error) {
	store, err := s.reviewStore()
	if err != nil {
		return nil, err
	}
	doc, err := store.Get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return &webapi.Review{Name: req.GetName(), Results: doc}, nil
}

// ListReviews returns stored runs newest first, paginated, optionally narrowed to one project and to
// one design.
//
// An empty parent lists EVERY run, parented or not. That is what keeps the two name shapes from
// costing a client anything: a viewer asking "what runs exist" asks once, and only narrows when it
// has a project in mind.
func (s *ReviewService) ListReviews(ctx context.Context, req *webapi.ListReviewsRequest) (*webapi.ListReviewsResponse, error) {
	store, err := s.reviewStore()
	if err != nil {
		return nil, err
	}
	parent, err := reviewParent(req.GetParent())
	if err != nil {
		return nil, err
	}
	design, err := parseReviewFilter(req.GetFilter())
	if err != nil {
		return nil, err
	}
	docs, names, next, err := store.List(ctx, parent, int(req.GetPageSize()), req.GetPageToken(), design)
	if err != nil {
		return nil, err
	}
	resp := &webapi.ListReviewsResponse{NextPageToken: next}
	for i, doc := range docs {
		resp.Reviews = append(resp.Reviews, &webapi.Review{Name: names[i], Results: doc})
	}
	return resp, nil
}

// DeleteReview removes a stored run. Deleting an absent run is ErrNotFound rather than a silent
// success, so a client acting on a stale listing is told rather than left believing it cleaned up.
func (s *ReviewService) DeleteReview(ctx context.Context, req *webapi.DeleteReviewRequest) (*emptypb.Empty, error) {
	store, err := s.reviewStore()
	if err != nil {
		return nil, err
	}
	if err := store.Delete(ctx, req.GetName()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// parseReviewFilter reads the one supported AIP-160 filter, `design="..."`, returning the design or
// "" for an empty filter.
//
// An unsupported filter is an ERROR rather than an ignored argument, and that is the whole reason
// this is a function instead of a string compare. A client that believed it had narrowed to its own
// board, and silently got every board's runs, would read another team's failures as its own. Refusing
// what we do not implement keeps a wrong answer from looking like a right one.
func parseReviewFilter(filter string) (string, error) {
	f := strings.TrimSpace(filter)
	if f == "" {
		return "", nil
	}
	value, ok := strings.CutPrefix(f, "design=")
	if !ok {
		return "", fmt.Errorf("%w: filter %q is not supported; the only supported filter is design=\"<ref>\"", ErrInvalidArgument, filter)
	}
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		value = unquoted
	}
	if value == "" {
		return "", fmt.Errorf("%w: filter %q names an empty design", ErrInvalidArgument, filter)
	}
	return value, nil
}

// GetReviewManifest resolves a stored checklist into the value CreateReview takes. It is the one place
// in this service that reads a file, and it is a separate RPC precisely so that read is visible in
// the contract rather than hidden inside a run: a caller that already holds a manifest never triggers
// it, and a host with no filesystem simply does not serve it.
//
// It validates before returning, so a malformed checklist is reported once, here, with the item that
// is wrong — rather than on every subsequent run, or worse, silently at scoring time.
func (s *ReviewService) GetReviewManifest(ctx context.Context, req *webapi.GetReviewManifestRequest) (*webapi.GetReviewManifestResponse, error) {
	if req.GetUri() == "" {
		return nil, fmt.Errorf("%w: GetReviewManifest needs a uri", ErrInvalidArgument)
	}
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	man, err := s.loader.Manifest(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	if err := review.Validate(man); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return &webapi.GetReviewManifestResponse{Manifest: ManifestProto(man)}, nil
}

// runOne builds one design's Model (netlist + optional separate board tier + the shared params tier)
// and runs the manifest over it. The board tier is read from board_ref, not the design, so a netlist
// entry can attach a separate confidential board export (WS3-089); an override that reads no board is
// an error, never a silent nil.
// It returns the composed catalog alongside the report because the stored document has to record the
// rules that ACTUALLY ran. That is the per-request catalog, overlay spliced on, not the service's
// base one: a run shaped by a request's own naming convention would otherwise archive a rule list its
// findings could not have come from.
func (s *ReviewService) runOne(ctx context.Context, designURI, boardURI artifact.URI, man review.Manifest, floor float64, ov Overlay) (review.Report, *check.Catalog, error) {
	m, err := BuildModel(ctx, s.loader, designURI, boardURI, ov.SpecsOr(s.specs), ov.ReadOptions()...)
	if err != nil {
		return review.Report{}, nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return review.Report{}, nil, err
	}
	present, scope, compScope := reviewClosures(m, s.byName)
	return review.Run(review.RunParams{
		Model: m, Catalog: cat, Manifest: man, Design: designURI.String(),
		Present: present, Scope: scope, CompScope: compScope, RatifiedFloor: floor,
		// intent.Emits narrows the intent/ prefix to the compiler's actual name space, so a pre-bound
		// not-yet-shipped intent rule reads not-automated instead of a misleading needs-design-intent
		// (WS3-098). Injected to keep `review` decoupled from the `intent` package.
		IntentRuleKnown: intent.Emits,
	}), cat, nil
}

// reviewClosures builds the presence and scope closures a review run needs over a designURI's Model and
// the profile index: present marks an item bound to a known-but-absent interface not-applicable, and
// scope/compScope filter a scoped binding's findings to the interface's nets/parts (WS3-058/083). The
// service owns this now that the CLI is a thin client of the review service (WS9-048); it keeps `review`
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
func reviewAreaProtos(r review.Report) []*checkspb.ReviewArea {
	var out []*checkspb.ReviewArea
	for _, ar := range r.Areas {
		area := &checkspb.ReviewArea{Name: ar.Area.Name}
		for _, it := range ar.Items {
			area.Items = append(area.Items, &checkspb.ReviewItem{
				Id:       it.Item.ID,
				Title:    it.Item.Title,
				Outcome:  string(it.Outcome),
				Note:     review.JoinNonEmpty(it.Note, it.Item.Note),
				Findings: FindingProtos(it.Findings),
				Unmet:    unmetProtos(it.Unmet),
			})
		}
		out = append(out, area)
	}
	return out
}

// reviewParent validates an optional parent project name. Empty is legal and means "no project",
// which is the ordinary state of a design on a mounted folder rather than a missing argument.
//
// A malformed parent is an ERROR rather than a silent fallback to the unparented collection. A client
// that believed it had scoped to its project and quietly got everything would read another team's
// verdicts as its own, which is the same failure parseReviewFilter refuses a bad filter for.
func reviewParent(parent string) (string, error) {
	if parent == "" {
		return "", nil
	}
	if _, ok := ProjectID(parent); !ok {
		return "", fmt.Errorf("%w: parent %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, parent)
	}
	return parent, nil
}

// unmetProtos carries a needs-data item's unmet dependencies onto the wire. Order is preserved
// because UnseededSymbols already sorted them, and the results document's byte-for-byte re-render
// guarantee depends on it.
func unmetProtos(deps []check.UnmetDependency) []*checkspb.UnmetDependency {
	if len(deps) == 0 {
		return nil
	}
	out := make([]*checkspb.UnmetDependency, 0, len(deps))
	for _, d := range deps {
		out = append(out, &checkspb.UnmetDependency{
			Mpn: d.MPN, Manufacturer: d.Manufacturer, Symbol: d.Symbol, SpecAbsent: d.SpecAbsent,
		})
	}
	return out
}
