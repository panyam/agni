package service

import (
	"context"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"sort"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/expect"
)

// CheckService runs the rule checks over a design's netlist IR and serves the rule catalog,
// over the same injected Loader the design service uses (CONSTRAINTS C13). Extracted from
// DesignService (WS9-026): checks are their own concern, run on demand and independent of
// rendering. It knows no transport.
type CheckService struct {
	// projects resolves a design to its project and loads that project's config; nil when this
	// deployment declares none. fallback is the deployment default used for a design with no project.
	projects *ProjectResolver
	fallback Overlay
	loader   Loader
	// catalog is the composed rule set the service lists and runs (WS3-006). It is injected
	// rather than read from package state so an embedder composes its own sources (a customer
	// suite, later the DSL compiler's output) alongside the built-ins:
	// check.NewCatalog(check.Builtins, check.NewSource("tesla", suite)). The catalog owns the
	// namespace/collision policy, so a wired service can never carry two rules with one name.
	catalog *check.Catalog
	// specs is the datasheet knowledge base the params join reads (WS10-003), nil when the serve
	// process was started without --params. GetComponentParams builds the model with it; a nil
	// provider yields no joined specs (PartSpec guards nil), so the panel simply shows nothing.
	specs param.ParamProvider
	// baseConvention is the catalog source name of the deployment's --conventions default, "" when
	// there is none. A request carrying its own convention REPLACES it (WS3-124), and this is how the
	// service says which of its catalog's sources is the one to replace.
	baseConvention string
	// conventions resolves a stored convention config into a value, backing GetNamingConvention. It is
	// a narrow port defined at the point of use rather than a method on the fat Loader, the same shape
	// ReviewLoader takes: only this one rpc reads a convention, and a host that cannot (or should not)
	// resolve one passes nil rather than stubbing a method it has no answer for.
	conventions ConventionLoader
}

// ConventionLoader reads a stored naming-convention config, mount-scoped by the impl.
//
// It is deliberately NOT on the path that RUNS checks. A convention reaches a check run as a value on
// the request (C22), so CheckDesign needs no filesystem; this backs the separate resolver rpc that a
// client with a ref and no filesystem calls first.
type ConventionLoader interface {
	Convention(ctx context.Context, uri artifact.URI) (naming.Config, error)
}

// NewCheckService returns a CheckService backed by the given loader, rule catalog, and (optional)
// datasheet provider. Pass check.DefaultCatalog() for the built-ins alone and a nil provider when no
// datasheet corpus is wired.
//
// baseConvention names the startup convention already composed into catalog, so a request that sends
// its own replaces it rather than stacking on it; pass "" when the catalog carries none.
func NewCheckService(loader Loader, catalog *check.Catalog, specs param.ParamProvider, baseConvention string, conventions ConventionLoader, projects *ProjectResolver) *CheckService {
	return &CheckService{loader: loader, catalog: catalog, specs: specs, baseConvention: baseConvention, conventions: conventions, projects: projects}
}

// GetNamingConvention resolves a stored convention config into the value an OverlayConfig carries.
// It validates before returning, so a malformed config is reported once, here, naming what is wrong,
// rather than on every run that sends it.
func (s *CheckService) GetNamingConvention(ctx context.Context, req *webapi.GetNamingConventionRequest) (*webapi.GetNamingConventionResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if s.conventions == nil {
		return nil, fmt.Errorf("%w: this server cannot resolve stored naming conventions", ErrInvalidArgument)
	}
	if req.GetUri() == "" {
		return nil, fmt.Errorf("%w: GetNamingConvention needs a uri", ErrInvalidArgument)
	}
	cfg, err := s.conventions.Convention(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	// Compile both halves now. naming.Load already parses, but a config whose patterns will not compile
	// or whose lexicon names an unknown component class only fails when it is USED, which would be on
	// every future request rather than on the one that chose it.
	if _, err := naming.BuildLexicon(cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	if len(cfg.Rules) > 0 {
		if _, err := naming.Source(cfg); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
		}
	}
	return &webapi.GetNamingConventionResponse{Convention: ConventionProto(cfg)}, nil
}

// ListRules returns the catalog the service runs, mapping each rule to its wire form: identity,
// the full prose (summary, impact, detail markdown), the facts it reads, its classification tags,
// and whether it can run now (from check.Available). The request's mount/path are advisory today
// since availability derives from each rule's Reads (design-independent); the design is not
// loaded, so ListRules works before a file is chosen and never fails on a bad path.
func (s *CheckService) ListRules(ctx context.Context, req *webapi.ListRulesRequest) (*webapi.ListRulesResponse, error) {
	// The uri is optional here (an empty one lists the whole catalog before a file is chosen), but
	// when it is given it decides WHOSE catalog is listed: a design in a project sees that project's
	// rules. Listing a different catalog from the one a run would use is the same invisible failure
	// as running the wrong one, just one step earlier.
	u, err := optionalArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// The listed catalog is the one a run under the SAME overlay would use. A request convention
	// replaces the server's (WS3-124), so listing the service's own catalog would advertise rules that
	// will not run and hide the ones that will.
	ov, err := s.projects.Overlay(ctx, u, req.GetOverlay(), s.fallback, s.baseConvention)
	if err != nil {
		return nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return nil, err
	}
	resp := &webapi.ListRulesResponse{}
	for _, r := range cat.Rules() {
		ok, reason := check.Available(r, nil)
		resp.Rules = append(resp.Rules, &webapi.RuleInfo{
			Name:              r.Name,
			Severity:          r.Severity,
			Summary:           r.Summary,
			Impact:            r.Impact,
			Detail:            r.Detail,
			Reads:             r.Reads,
			Tags:              r.Tags,
			Available:         ok,
			UnavailableReason: reason,
		})
	}
	return resp, nil
}

// CheckDesign runs the rule checks (the check/ library) over a loaded design's netlist IR. A
// geometry-only file with no netlist classifies as an invalid argument. request.rules selects the
// subset to run (empty = the whole catalog). Each finding's subject (kind + ref) is the join key
// the viewer uses to highlight the offending element, and its sheets locate that subject in the
// design's default-layout geometry (WS9-024); a design with no resolvable geometry degrades to
// findings without sheets rather than an error.
func (s *CheckService) CheckDesign(ctx context.Context, req *webapi.CheckDesignRequest) (*webapi.CheckDesignResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// Per-request overlay config (WS3-102) resolves the same way it does for a review, through the one
	// ComposeOverlay, so the two surfaces cannot read a convention file differently.
	ov, err := s.projects.Overlay(ctx, u, req.GetOverlay(), s.fallback, s.baseConvention)
	if err != nil {
		return nil, err
	}
	board, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	m, err := BuildModel(ctx, s.loader, u, board, ov.SpecsOr(s.specs), ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return nil, err
	}
	rules := cat.Filter(check.Facets{Names: req.GetRules()})
	resp := &webapi.CheckDesignResponse{Findings: FindingProtos(check.Run(m, rules))}
	AnnotateSheets(resp.Findings, BuildGeometry(ctx, s.loader, u), m)
	return resp, nil
}

// GetExpectations returns a design's expected findings from its sidecar (WS6-006). A missing sidecar
// yields an empty list, not an error, so the viewer's panel is empty rather than broken; only an
// invalid mount/path or a malformed sidecar is an error. The `fires` entries come first, then the
// `pending` ones, each flattened to a RuleExpectation the client reconciles against CheckDesign.
func (s *CheckService) GetExpectations(ctx context.Context, req *webapi.GetExpectationsRequest) (*webapi.GetExpectationsResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	e, err := s.loader.Expectations(ctx, u)
	if err != nil {
		return nil, classifyLoadErr(err)
	}
	resp := &webapi.GetExpectationsResponse{HasSidecar: e != nil}
	if e != nil {
		resp.Expectations = append(expectationProtos(e.Fires, false), expectationProtos(e.Pending, true)...)
	}
	return resp, nil
}

// GetComponentParams surfaces the datasheet join read-only (WS9-035): every component whose MPN
// resolves to a seeded PartSpec, with that spec's parameters for the panel tree. It is the only
// CheckService method that needs the datasheet provider, so it builds the model WITH it
// (NewModelWithParams); a nil provider (serve started without --params) or an unseeded design yields
// an empty list — never an error — so the panel degrades gracefully. Board geometry is irrelevant to
// the join, so nil is passed.
func (s *CheckService) GetComponentParams(ctx context.Context, req *webapi.GetComponentParamsRequest) (*webapi.GetComponentParamsResponse, error) {
	u, err := artifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	m, err := BuildModel(ctx, s.loader, u, artifact.URI{}, s.specs)
	if err != nil {
		return nil, err
	}
	resp := &webapi.GetComponentParamsResponse{}
	for _, c := range m.Components() {
		spec := m.PartSpec(c.GetRefDes())
		if spec == nil {
			continue
		}
		resp.Components = append(resp.Components, &webapi.ComponentParams{
			RefDes: c.GetRefDes(),
			Mpn:    m.ComponentMPN(c.GetRefDes()),
			Spec:   spec,
		})
	}
	return resp, nil
}

// expectationProtos flattens a rule->entry map to sorted RuleExpectations (rule order stable so
// the panel does not reshuffle between fetches), stamping pending on each and carrying the
// sidecar's optional why narration (WS6-008).
func expectationProtos(m map[string]expect.Entry, pending bool) []*webapi.RuleExpectation {
	rules := make([]string, 0, len(m))
	for r := range m {
		rules = append(rules, r)
	}
	sort.Strings(rules)
	out := make([]*webapi.RuleExpectation, 0, len(rules))
	for _, r := range rules {
		out = append(out, &webapi.RuleExpectation{Rule: r, Subjects: m[r].Subjects, Pending: pending, Why: m[r].Why})
	}
	return out
}

// FindingProto is the one place a check.Finding becomes its webapi wire form, so the RPC and any
// other surface (e.g. the CLI's `check --format json`) share a single finding shape instead of two
// that can drift. Subject (kind + ref + pin) is the highlight join key; Provenance is carried
// through when the subject has it (nil-safe) so a consumer can link back to the source.
//
// A KindBus subject carries no net, so its geometry join key is the bus NAME (its range-label
// identity under WS1-034): it rides Subject.bus_id from the finding subject (the same name the
// geometry reader stamped on the bus WireGeometry.Net), so a bus-not-modeled finding highlights its
// own drawn bus (WS7-042b). A bus with no drawn geometry (a bus_alias, an EDIF array) simply
// resolves to nothing — its "bus not drawn" note is WS7-042c.
func FindingProto(f check.Finding) *checkspb.Finding {
	subject := &checkspb.Subject{Kind: f.Kind, Ref: f.Subject, Pin: f.Pin, NetId: f.NetID}
	if f.Kind == check.KindBus {
		subject.BusId = f.Subject
	}
	return &checkspb.Finding{
		Rule:         f.Rule,
		Severity:     f.Severity,
		Inconclusive: f.Inconclusive,
		Subject:      subject,
		Message:      f.Message,
		Provenance:   f.Prov,
		Datasheets:   datasheetCitationProtos(f.DatasheetProv),
	}
}

// datasheetCitationProto maps a check.DatasheetCitation to its wire form, nil for a finding not
// backed by a seeded datasheet value (WS9-048). One conversion site, shared by every Finding
// consumer (the review/check JSON surfaces and the web check panel).
// datasheetCitationProtos maps a finding's citations to the wire form, preserving order. A
// connection-aware rule contributes one per part its conclusion rests on (WS3-028).
func datasheetCitationProtos(cs []*check.DatasheetCitation) []*checkspb.DatasheetCitation {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*checkspb.DatasheetCitation, 0, len(cs))
	for _, c := range cs {
		if pc := datasheetCitationProto(c); pc != nil {
			out = append(out, pc)
		}
	}
	return out
}

func datasheetCitationProto(c *check.DatasheetCitation) *checkspb.DatasheetCitation {
	if c == nil {
		return nil
	}
	return &checkspb.DatasheetCitation{
		Doc:        c.Doc,
		DocRef:     c.DocRef,
		Page:       c.Page,
		Section:    c.Section,
		Method:     c.Method,
		Confidence: c.Confidence,
	}
}

// FindingProtos maps a slice of findings through FindingProto, preserving order (check.Run sorts by
// rule then subject).
func FindingProtos(fs []check.Finding) []*checkspb.Finding {
	out := make([]*checkspb.Finding, 0, len(fs))
	for _, f := range fs {
		out = append(out, FindingProto(f))
	}
	return out
}
