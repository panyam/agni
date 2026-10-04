package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/artifact"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/review"
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
// so the transport maps it to failed-precondition rather than invalid-argument, since the request was
// fine and the deployment is what is missing.
var ErrReviewStoreNotConfigured = errors.New("no review store configured")

// ReviewLoader is the narrow file surface ReviewService needs, defined at the point of use as a
// subset of service.Loader plus Manifest. It reads the netlist design, an optional separate board
// export (WS3-089) and a stored checklist manifest, each resolved inside a mount by the impl and never
// as a host path (C22).
//
// Manifest is NOT on the run path (WS9-050). CreateReview takes the checklist as a value, so only
// GetReviewManifest calls it.
type ReviewLoader interface {
	Design(ctx context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error)
	Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error)
	Manifest(ctx context.Context, uri artifact.URI) (review.Manifest, error)
	// DesignHash returns "sha256:<hex>" over the design's bytes, the revision identity a stored run
	// records so a document is never re-read against a design that has since changed. It is on the
	// loader because hashing reads bytes and this package does no I/O (C13/C22).
	//
	// An unreadable design is NOT an error, since the run already succeeded. Return ("", nil), which
	// DesignRef.content_hash allows.
	DesignHash(ctx context.Context, uri artifact.URI) (string, error)
}

// ReviewEnv is the deployment-level provenance a stored run records, naming which overlay tiers were
// composed into the catalog this service holds and what build produced the document.
//
// It is injected rather than derived because a composed catalog cannot answer "were profiles
// attached", since an overlay profile is just another rule in it. See
// docsite/content/architecture/checks-contract.md#provenance-is-read-off-the-resolved-overlay.
type ReviewEnv struct {
	// ProducerVersion is the engine build identity (version.Version() at the entrypoint).
	ProducerVersion string
	// Profiles reports that --profile-path overlay profiles were composed into the catalog.
	Profiles bool
	// Intent reports that a --intent-path design-intent declaration was composed into the catalog.
	Intent bool
}

// ReviewService runs a review checklist manifest over one or more designs, the transport-neutral
// analogue of `agni review` (WS9-047). The catalog, the profile presence index and the datasheet
// provider are design-INDEPENDENT and injected once, as CheckService receives its catalog and specs.
// Only the Model and its presence/scope closures are per design. It knows no transport (C13).
type ReviewService struct {
	// projects plays the same role as on CheckService; see ProjectResolver.
	projects *ProjectResolver
	loader   ReviewLoader
	catalog  *check.Catalog
	// byName is every profile (built-in + overlay) keyed by Name, for the interface-absence check that
	// marks a profile item not-applicable when its interface is absent from the design.
	byName map[string][]profiles.Profile
	// specs is the datasheet knowledge base the params join reads (WS10-003), nil without --params.
	specs param.ParamProvider
	// store persists runs (WS9-053). Nil makes the four resource methods refuse up front, rather than
	// a create running the full sweep and dropping the result.
	store ReviewStore
	env   ReviewEnv
	// baseConvention is the catalog source name of the deployment's --conventions default, "" when
	// there is none. A request's own convention replaces it, as on CheckService.
	baseConvention string
}

// NewReviewService returns a ReviewService over the given loader, review store, composed rule
// catalog, profile presence index, and optional datasheet provider (nil when no corpus is wired).
//
// store may be nil, which disables the review resource methods; pass a MemReviewStore for a caller
// that wants runs without persisting them, as `agni review` does.
func NewReviewService(loader ReviewLoader, store ReviewStore, catalog *check.Catalog, byName map[string][]profiles.Profile, specs param.ParamProvider, env ReviewEnv, baseConvention string, projects *ProjectResolver) *ReviewService {
	return &ReviewService{loader: loader, store: store, catalog: catalog, byName: byName, specs: specs, env: env, baseConvention: baseConvention, projects: projects}
}

// reviewStore returns the configured store or ErrReviewStoreNotConfigured. Every resource method
// goes through it.
func (s *ReviewService) reviewStore() (ReviewStore, error) {
	if s.store == nil {
		return nil, ErrReviewStoreNotConfigured
	}
	return s.store, nil
}

// CreateReview runs the checklist against the design and persists the result (WS9-053). The run is
// all-or-nothing, so an invalid manifest, an unreadable design, or a board_ref at a file with no board
// geometry is an error and nothing is stored.
//
// The checklist arrives as a VALUE (WS9-050) and is validated here rather than trusted. A manifest
// that never passed through review.Load has had no parser enforce its rules, and an item carrying two
// mutually-exclusive bindings would otherwise score a check its author did not ask for.
//
// The stored document is the same self-contained CheckResults `agni review --results-out` writes,
// carrying the checklist SNAPSHOT rather than its name (docsite/content/architecture/web-services.md).
func (s *ReviewService) CreateReview(ctx context.Context, req *webapi.CreateReviewRequest) (*webapi.Review, error) {
	parent, err := reviewParent(req.GetParent())
	if err != nil {
		return nil, err
	}
	designURI, err := ParseArtifactURI(req.GetDesignUri())
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
	if err := review.ValidateStructure(man); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	// Per-request overlay config (WS3-102), composed BEFORE the design is read, because net roles
	// are resolved at ingestion and its lexicon half has to reach the read. An empty overlay leaves the
	// service's own catalog and the default vocabulary in place.
	ov, err := s.projects.Overlay(ctx, designURI, req.GetOverlay(), s.baseConvention)
	if err != nil {
		return nil, err
	}
	// The manifest's inline queries compile against the vocabulary the review runs with, the design's
	// project library and any library sent with the request included (agni issues 779, 788), so a query
	// naming a house member validates here exactly as it will run.
	vocab, err := ov.Registry()
	if err != nil {
		return nil, err
	}
	if err := review.Validate(man, review.WithVocabulary(vocab)); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	// Tiers from the design's declaration, the same call CheckDesign makes, so a review in the browser
	// scores against the board the design declares, as the CLI's does (agni issues 646, 656; C32).
	netlistURI, boardURI, _, err := s.projects.TierURIs(ctx, designURI, boardURI, req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	// The budget reaches every checklist query through ctx (agni issue 792); an item past it reads
	// inconclusive, naming the budget, and the others still answer.
	rep, cat, err := s.runOne(query.NarrowBudget(ctx, req.GetWorkBudget()), netlistURI, boardURI, man, req.GetRatifiedFloor(), ov)
	if err != nil {
		return nil, err
	}
	// The hash is provenance, not a precondition, so an unreadable source records no hash. It is taken
	// from the NETLIST tier the run scored, not the request's URI, because a design folder does not hash.
	hash, err := s.loader.DesignHash(ctx, netlistURI)
	if err != nil {
		hash = ""
	}
	doc := &checkspb.CheckResults{
		Meta: &checkspb.ResultsMeta{
			Schema:          results.Schema,
			Producer:        results.Producer,
			ProducerVersion: s.env.ProducerVersion,
			// A native run records what it could NOT check as well as what it found. See
			// docsite/content/architecture/checks-contract.md#the-outcome-vocabulary-names-every-way-a-question-went-unanswered.
			CoverageAxis: true,
		},
		Design: &checkspb.DesignRef{Source: designURI.String(), ContentHash: hash},
		// Provenance comes off the RESOLVED overlay the run used, not this service's startup config,
		// which describes the deployment rather than the run. See
		// docsite/content/architecture/checks-contract.md#provenance-is-read-off-the-resolved-overlay.
		Run: RunConfigProto(ov.Provenance(RunProvenance{
			Params:      s.specs != nil,
			Profiles:    s.env.Profiles,
			Intent:      s.env.Intent,
			Conventions: req.GetOverlay().GetConfig().GetConventions().GetName(),
		}), req.GetRatifiedFloor()),
		// The catalog snapshot is the one composed for THIS run, overlay included, not the service's base.
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
	return ReviewOf(name, doc), nil
}

// ListChecklists answers which checklists the design's project declares, inherited ones included (agni
// issue 859). It reads the project through the resolver CreateReview's overlay uses and opens no file,
// so it serves a host with no filesystem as CreateReview does. See ProjectResolver.Checklists for what
// an absent project and an empty one each answer.
func (s *ReviewService) ListChecklists(ctx context.Context, req *webapi.ListChecklistsRequest) (*webapi.ListChecklistsResponse, error) {
	u, err := ParseArtifactURI(req.GetDesignUri())
	if err != nil {
		return nil, err
	}
	lists, project, err := s.projects.Checklists(ctx, u)
	if err != nil {
		return nil, err
	}
	return &webapi.ListChecklistsResponse{Project: project, Checklists: lists}, nil
}

// ReviewOf is the Review resource for a stored document, with its summary computed from the
// document's item outcomes (agni issue 734). Every rpc returning a review builds it here, so the
// summary is never stored and a document written before the field existed answers with one. The CLI
// uses it too, for a document it read from a file.
func ReviewOf(name string, doc *checkspb.CheckResults) *webapi.Review {
	var outcomes []review.Outcome
	for _, a := range doc.GetAreas() {
		for _, it := range a.GetItems() {
			outcomes = append(outcomes, review.Outcome(it.GetOutcome()))
		}
	}
	t := review.TallyOf(outcomes...)
	return &webapi.Review{Name: name, Results: doc, Summary: &webapi.ReviewSummary{
		Total: int32(t.Total), Covered: int32(t.Covered()), Answered: int32(t.Answered()),
		Pass: int32(t.Pass), Fail: int32(t.Fail), Provisional: int32(t.Provisional),
	}}
}

// GetReview returns a stored run. It reads only the store, so neither the design nor the checklist
// it was about needs to still exist.
func (s *ReviewService) GetReview(ctx context.Context, req *webapi.GetReviewRequest) (*webapi.Review, error) {
	store, err := s.reviewStore()
	if err != nil {
		return nil, err
	}
	doc, err := store.Get(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	return ReviewOf(req.GetName(), doc), nil
}

// ListReviews returns stored runs newest first, paginated, optionally narrowed to one project and to
// one design.
//
// An empty parent lists EVERY run, parented or not.
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
		resp.Reviews = append(resp.Reviews, ReviewOf(names[i], doc))
	}
	return resp, nil
}

// DeleteReview removes a stored run. Deleting an absent run is ErrNotFound, so a client acting on a
// stale listing is told.
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
// An unsupported filter is an ERROR rather than an ignored argument. A client that believed it had
// narrowed to its own board and got every board's runs would read another team's failures as its own.
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

// GetReviewManifest resolves a stored checklist into the value CreateReview takes. It is the one
// place in this service that reads a file, so a caller already holding a manifest never triggers it
// and a host with no filesystem need not serve it.
//
// It validates before returning, so a malformed checklist is reported here with the item that is
// wrong rather than at scoring time.
func (s *ReviewService) GetReviewManifest(ctx context.Context, req *webapi.GetReviewManifestRequest) (*webapi.GetReviewManifestResponse, error) {
	if req.GetUri() == "" {
		return nil, fmt.Errorf("%w: GetReviewManifest needs a uri", ErrInvalidArgument)
	}
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	man, err := s.loader.Manifest(ctx, u)
	if err != nil {
		return nil, ClassifyLoadErr(err)
	}
	// Structure only: a stored manifest is described with no design in view, so there is no run
	// vocabulary to compile its queries against. CreateReview compiles them against the one it runs with.
	if err := review.ValidateStructure(man); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	return &webapi.GetReviewManifestResponse{Manifest: ManifestProto(man)}, nil
}

// runOne builds one design's Model (netlist, optional separate board tier, the shared params tier)
// and runs the manifest over it. The board tier is read from board_ref, so a netlist entry can attach
// a separate confidential board export (WS3-089); an override that reads no board is an error.
//
// It returns the per-request catalog, overlay spliced on, because the stored document records the
// rules that ACTUALLY ran.
func (s *ReviewService) runOne(ctx context.Context, designURI, boardURI artifact.URI, man review.Manifest, floor float64, ov Overlay) (review.Report, *check.Catalog, error) {
	m, err := BuildModel(ctx, s.loader, designURI, boardURI, ov.SpecsOver(s.specs), ov.ReadOptions()...)
	if err != nil {
		return review.Report{}, nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return review.Report{}, nil, err
	}
	vocab, err := ov.Registry()
	if err != nil {
		return review.Report{}, nil, err
	}
	// The run's own profiles, so a project profile re-binding a built-in's signals is the one the
	// presence gate asks about (agni issue 833).
	present, scope, compScope := reviewClosures(m, ov.ProfileIndex(s.byName))
	rep, err := review.Run(ctx, review.RunParams{
		Model: m, Catalog: cat, Manifest: man, Design: designURI.String(),
		Present: present, Scope: scope, CompScope: compScope, RatifiedFloor: floor,
		// intent.Emits narrows the intent/ prefix to the compiler's name space, so a pre-bound intent
		// rule that has not shipped reads not-automated rather than needs-design-intent (WS3-098).
		// Injected to keep `review` decoupled from `intent`.
		IntentRuleKnown: intent.Emits,
		Vocabulary:      vocab,
	})
	if err != nil {
		return review.Report{}, nil, err
	}
	return rep, cat, nil
}

// reviewClosures builds the presence and scope closures a review run needs over a design's Model and
// the profile index. present marks an item bound to a known-but-absent interface not-applicable, and
// scope/compScope filter a scoped binding's findings to the interface's nets/parts (WS3-058/083).
// Keeping them here keeps `review` decoupled from `profiles` (WS9-048).
func reviewClosures(m check.Model, byName map[string][]profiles.Profile) (review.PresenceFunc, review.ScopeFunc, review.CompScopeFunc) {
	present := func(name string) (review.Presence, bool) {
		ps, ok := byName[name]
		if !ok {
			return review.IfaceAbsent, false // unknown interface, so leave the item running
		}
		// The interface evaluates when a component declares its host, or when its signal convention is
		// in use AND the completeness rule can anchor, the same preconditions the profile's rules apply
		// (WS3-090, WS3-099). The host path needs no anchor because hostIncompleteRule anchors on the
		// declared component.
		for _, p := range ps {
			if profiles.HostDeclared(m, p) || (profiles.InUse(m, p) && profiles.Anchored(m, p)) {
				return review.IfacePresent, true
			}
		}
		// In use but unanchored. The interface is named to the convention, yet the completeness rule has
		// nothing to hang on, so it is neither absent nor checkable under this naming (WS3-099).
		for _, p := range ps {
			if profiles.InUse(m, p) {
				return review.IfaceConventionUnmatched, true
			}
		}
		// Not strictly evaluable. A host-bound interface that IS named on the board but whose host is
		// annotated nowhere is host-unsatisfied, which reads not-automated. With no such evidence it is
		// absent (not-applicable); an absent host-bound interface must NOT read not-automated.
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

// reviewAreaProtos maps a review.Report's areas to their wire form. Outcome is the review.Outcome
// string as-is, since the CLI and a panel both key on it. The tally is not carried because a consumer
// derives it from the item outcomes, as review.Report.Tally() does.
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
// A malformed parent is an ERROR rather than a fallback to the unparented collection, for the same
// reason parseReviewFilter refuses a bad filter.
func reviewParent(parent string) (string, error) {
	if parent == "" {
		return "", nil
	}
	if _, ok := ProjectID(parent); !ok {
		return "", fmt.Errorf("%w: parent %q is not a project resource name (want \"projects/{project}\")", ErrInvalidArgument, parent)
	}
	return parent, nil
}

// unmetProtos carries a needs-data item's unmet dependencies onto the wire. It preserves the order
// UnseededSymbols sorted them into, which the results document's byte-for-byte re-render relies on.
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
