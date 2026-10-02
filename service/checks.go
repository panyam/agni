package service

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/stdlib/profiles"
	"sort"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/core/query"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/expect"
)

// CheckService runs the rule checks over a design's netlist IR and serves the rule catalog, over
// the same injected Loader the design service uses (C13). It was split out of DesignService
// (WS9-026) and knows no transport.
type CheckService struct {
	// projects resolves a design to its project's config, nil when the deployment declares none.
	// fallback is the default for a design with no project. See ProjectResolver.Overlay.
	projects *ProjectResolver
	fallback Overlay
	loader   Loader
	// catalog is the composed rule set the service lists and runs (WS3-006). It is injected so an
	// embedder can compose its own sources beside the built-ins, e.g.
	// check.NewCatalog(check.Builtins, check.NewSource("acme", suite)). The catalog rejects two
	// rules with one name.
	catalog *check.Catalog
	// specs is the datasheet provider the params join reads (WS10-003), nil when serve ran without
	// --params, in which case GetComponentParams returns an empty list.
	specs param.ParamProvider
	// baseConvention names the catalog source of the deployment's --conventions default, "" when
	// there is none. A request convention replaces that source (WS3-124). See ComposeOverlay.
	baseConvention string
	// conventions backs GetNamingConvention alone, so it is a narrow port rather than a Loader
	// method, like ReviewLoader. A host that cannot resolve a stored convention passes nil.
	conventions ConventionLoader
	// profiles is the deployment's interface-profile index (the built-ins and any --profile-path),
	// the one ReviewService holds, for GetInterfaceCoverage. Nil means the built-ins alone.
	profiles map[string][]profiles.Profile
}

// WithProfileIndex gives the service the deployment's interface-profile index, keyed by name, which
// the coverage panel walks with a design's own profiles layered on. Pass the same index the
// ReviewService was built with, so the panel and a review describe an interface by one definition. It
// returns s for chaining. Without it the panel walks the built-ins.
func (s *CheckService) WithProfileIndex(byName map[string][]profiles.Profile) *CheckService {
	s.profiles = byName
	return s
}

// ConventionLoader reads a stored naming-convention config, mount-scoped by the impl.
//
// It is NOT on the path that RUNS checks. A convention reaches a check run as a value on the
// request (C22), so CheckDesign needs no filesystem. This backs the resolver rpc a client with a
// ref and no filesystem calls first.
type ConventionLoader interface {
	Convention(ctx context.Context, uri artifact.URI) (*configpb.NamingConvention, error)
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
	u, err := ParseArtifactURI(req.GetUri())
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
		return nil, ClassifyLoadErr(err)
	}
	// Compile both halves now. naming.Load only parses, and a bad pattern or an unknown component
	// class would otherwise fail on every later request that sends this config.
	if _, err := naming.BuildLexicon(cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
	}
	if len(cfg.GetRules()) > 0 {
		if _, err := naming.Source(cfg); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
		}
	}
	return &webapi.GetNamingConventionResponse{Convention: cfg}, nil
}

// ListRules returns the catalog a run under the same uri and overlay would use, each rule in its
// wire form with its availability from check.Available. The uri is optional, and when given it
// picks the project whose catalog is listed. The design itself is never loaded, so ListRules works
// before a file is chosen.
func (s *CheckService) ListRules(ctx context.Context, req *webapi.ListRulesRequest) (*webapi.ListRulesResponse, error) {
	u, err := optionalArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// A request convention replaces the server's (WS3-124), so listing the service's own catalog
	// would show rules that will not run and hide the ones that will.
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
			Remedy:            r.Remedy,
			Detail:            r.Detail,
			Reads:             r.Reads,
			Tags:              r.Tags,
			Available:         ok,
			UnavailableReason: reason,
		})
	}
	return resp, nil
}

// CheckDesign runs the rule checks over a loaded design's netlist IR. A geometry-only file with no
// netlist is an invalid argument. request.rules selects the subset to run, empty meaning the whole
// catalog. Each finding's sheets locate its subject in the design's geometry (WS9-024), and a
// design with no resolvable geometry gets findings without sheets rather than an error.
func (s *CheckService) CheckDesign(ctx context.Context, req *webapi.CheckDesignRequest) (*webapi.CheckDesignResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// Per-request overlay config (WS3-102) resolves through the same ComposeOverlay a review uses.
	ov, err := s.projects.Overlay(ctx, u, req.GetOverlay(), s.fallback, s.baseConvention)
	if err != nil {
		return nil, err
	}
	board, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	// Tiers from the design's declaration, so a check addressed at a companion analyses the netlist
	// and annotates against the schematic (agni issue 656).
	nu, bu, gu, err := s.projects.TierURIs(ctx, u, board, req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	m, err := BuildModel(ctx, s.loader, nu, bu, ov.SpecsOver(s.specs), ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return nil, err
	}
	rules := cat.Filter(check.Facets{Names: req.GetRules()})
	// check.Available is asked HERE with the model, where ListRules asks it with nil. ListRules wants
	// "can this rule ever run" and the panel wants "did it run on this design", so a rule gated on
	// this design is reported in `skipped` rather than vanishing from the findings.
	runnable, skipped := partitionAvailable(rules, m)
	// Verdicts come from the SAME runnable set as the findings. A skipped rule contributes no
	// verdicts, since it considered nothing.
	// The request's context reaches every rule, so a check nobody is waiting for stops and says so
	// (agni issue 795) rather than finishing the catalog.
	// A deployment's budget, narrowed by the request's, reaches every query-backed rule through ctx
	// (agni issue 792); a rule past it reports itself inconclusive and the others still answer.
	ctx = query.NarrowBudget(ctx, req.GetWorkBudget())
	// One evaluation per rule for both contracts (agni issue 810). Calling Run and then RunVerdicts
	// ran the whole catalog twice, since a finding is a projection of a verdict.
	findings, verdicts, err := check.RunAll(ctx, m, runnable)
	if err != nil {
		return nil, err
	}
	resp := &webapi.CheckDesignResponse{
		Findings: FindingProtos(findings),
		Verdicts: VerdictProtos(verdicts),
		Skipped:  skipped,
	}
	AnnotateSheets(resp.Findings, BuildGeometry(ctx, s.loader, gu, ov.ReadOptions()...), m)
	return resp, nil
}

// GetExpectations returns a design's expected findings from its sidecar (WS6-006). A missing
// sidecar yields an empty list, and only an invalid uri or a malformed sidecar is an error. The
// `fires` entries come first, then the `pending` ones, which the client reconciles against
// CheckDesign.
func (s *CheckService) GetExpectations(ctx context.Context, req *webapi.GetExpectationsRequest) (*webapi.GetExpectationsResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	e, err := s.loader.Expectations(ctx, u)
	if err != nil {
		return nil, ClassifyLoadErr(err)
	}
	resp := &webapi.GetExpectationsResponse{HasSidecar: e != nil}
	if e != nil {
		resp.Expectations = append(expectationProtos(e.Fires, false), expectationProtos(e.Pending, true)...)
	}
	return resp, nil
}

// GetComponentParams returns the datasheet join read-only (WS9-035), listing every component whose
// MPN resolves to a seeded PartSpec with that spec's parameters. A nil provider or an unseeded design
// yields an empty list rather than an error. The join needs no board, so none is passed.
func (s *CheckService) GetComponentParams(ctx context.Context, req *webapi.GetComponentParamsRequest) (*webapi.GetComponentParamsResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	// Through the project's overlay, as CheckDesign is, so the panel shows the same specs a check
	// judges by. It used the server's corpus alone, which inside a project with its own params/
	// showed a different spec than the verdict rested on, or none.
	ov, err := s.projects.Overlay(ctx, u, nil, s.fallback, s.baseConvention)
	if err != nil {
		return nil, err
	}
	specs := ov.SpecsOver(s.specs)
	m, err := BuildModel(ctx, s.loader, u, artifact.URI{}, specs, ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	namer, _ := specs.(param.CorpusNamer)
	resp := &webapi.GetComponentParamsResponse{}
	for _, c := range m.Components() {
		spec := m.PartSpec(c.GetRefDes())
		if spec == nil {
			continue
		}
		cp := &webapi.ComponentParams{
			RefDes: c.GetRefDes(),
			Mpn:    m.ComponentMPN(c.GetRefDes()),
			Spec:   spec,
		}
		if namer != nil {
			cp.Corpus = namer.CorpusOf(spec)
		}
		resp.Components = append(resp.Components, cp)
	}
	return resp, nil
}

// expectationProtos flattens a rule->entry map to RuleExpectations sorted by rule, so the panel does
// not reshuffle between fetches. It stamps pending on each and carries the sidecar's optional why
// (WS6-008).
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

// FindingProto is the one place a check.Finding becomes its wire form, shared by the rpc and the
// CLI's `check --format json` (C31). Subject is the highlight join key, and Provenance is carried
// when present so a consumer can link back to the source. A bus subject's join key is handled in
// subjectProto.
func FindingProto(f check.Finding) *checkspb.Finding {
	return &checkspb.Finding{Subject: subjectProto(f.Subject), Rule: f.Rule, Severity: f.Severity, Inconclusive: f.Inconclusive, Message: f.Message, Provenance: f.Prov, Datasheets: datasheetCitationProtos(f.DatasheetProv), Context: contextSubjectProtos(f.Context)}
}

// contextSubjectProtos maps a finding's context entities to the wire form, PRESERVING ORDER, since
// the order is the rule author's and matches the order the message names them (agni issue 349).
func contextSubjectProtos(cs []check.ContextSubject) []*checkspb.ContextSubject {
	if len(cs) == 0 {
		return nil
	}
	out := make([]*checkspb.ContextSubject, 0, len(cs))
	for _, c := range cs {
		out = append(out, &checkspb.ContextSubject{Subject: subjectProto(c.Entity), Role: c.Role})
	}
	return out
}

// subjectProto is the ONE place a check.Entity becomes a wire Subject, shared by findings, verdict
// tuples and context entries, so the bus rule below lives in one place.
//
// A bus carries no net, so its name is its only geometry join key and rides as bus_id (WS7-042b).
// A bus with no drawn geometry, such as a bus_alias or an EDIF array, resolves to nothing
// (WS7-042c).
func subjectProto(e check.Entity) *checkspb.Subject {
	out := &checkspb.Subject{Kind: e.Kind, Ref: e.Ref, Pin: e.Pin, NetId: e.NetID}
	if e.Kind == check.KindBus {
		out.BusId = e.Ref
	}
	return out
}

// subjectFromProto is the inverse of subjectProto. bus_id is not read back, because it derives from
// the ref and trusting an inbound one would let a producer rename a bus.
func subjectFromProto(s *checkspb.Subject) check.Entity {
	return check.Entity{Kind: s.GetKind(), Ref: s.GetRef(), Pin: s.GetPin(), NetID: s.GetNetId()}
}

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

// datasheetCitationProto maps one citation to its wire form, nil for nil (WS9-048).
func datasheetCitationProto(c *check.DatasheetCitation) *checkspb.DatasheetCitation {
	if c == nil {
		return nil
	}
	return &checkspb.DatasheetCitation{
		Doc:              c.Doc,
		DocRef:           c.DocRef,
		Page:             c.Page,
		Section:          c.Section,
		Method:           c.Method,
		Confidence:       c.Confidence,
		Verification:     c.Verification,
		VerifiedRevision: c.VerifiedRevision,
		Corpus:           c.Corpus,
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

// partitionAvailable splits selected rules into those that can evaluate on this design and those that
// cannot, carrying each skipped rule's reason. check.Run already skips a gated rule, so the findings
// are the same either way; the split lets the response REPORT the skipped half.
func partitionAvailable(rules []*check.Rule, m check.Model) ([]*check.Rule, []*webapi.SkippedRule) {
	runnable := make([]*check.Rule, 0, len(rules))
	var skipped []*webapi.SkippedRule
	for _, r := range rules {
		ok, why := check.Available(r, m)
		if ok {
			runnable = append(runnable, r)
			continue
		}
		skipped = append(skipped, &webapi.SkippedRule{Name: r.Name, Reason: why})
	}
	return runnable, skipped
}

// VerdictProto and VerdictFromProto are the conversion pair for the considered set. check.Verdict is
// a hand-written twin of a wire message, so the pair carries a C26 round-trip guard
// (TestVerdictProtoRoundTrip); a field the converter never learned is otherwise invisible to any
// assertion on the proto, as naming.Lexicon and Profile.HostClass each found.
//
// Verdict.Finding is DELIBERATELY not on the wire. A failing verdict's finding travels in
// CheckDesignResponse.findings, and a second copy here could disagree with it.
// TestVerdictFieldCensus fails when a field is added to check.Verdict, so whoever adds one decides
// whether it goes on the wire.
func VerdictProto(v check.Verdict) *checkspb.Verdict {
	subjects := make([]*checkspb.Subject, 0, len(v.Subjects))
	for _, e := range v.Subjects {
		subjects = append(subjects, subjectProto(e))
	}
	return &checkspb.Verdict{Subjects: subjects, Id: check.VerdictID(v), Rule: v.Rule, Outcome: outcomeProto(v.Outcome), Witness: witnessProto(v.Witness), Reason: v.Reason, Context: contextSubjectProtos(v.Context)}
}

// VerdictFromProto is the inverse. Id is not read back, because it derives from the other fields and
// trusting an inbound one would let a producer rename a verdict.
func VerdictFromProto(p *checkspb.Verdict) check.Verdict {
	if p == nil {
		return check.Verdict{}
	}
	subjects := make([]check.Entity, 0, len(p.GetSubjects()))
	for _, s := range p.GetSubjects() {
		subjects = append(subjects, subjectFromProto(s))
	}
	return check.Verdict{Subjects: subjects, Rule: p.GetRule(), Outcome: outcomeFromProto(p.GetOutcome()), Reason: p.GetReason(), Witness: witnessFromProto(p.GetWitness()), Context: contextSubjectsFromProto(p.GetContext())}
}

// VerdictProtos maps a verdict list, the counterpart of FindingProtos.
func VerdictProtos(vs []check.Verdict) []*checkspb.Verdict {
	if len(vs) == 0 {
		return nil
	}
	out := make([]*checkspb.Verdict, 0, len(vs))
	for _, v := range vs {
		out = append(out, VerdictProto(v))
	}
	return out
}

// outcomeProto maps the Go outcome vocabulary to the enum. An unrecognised outcome maps to
// UNSPECIFIED rather than PASS, so a new outcome never reaches a consumer as a false pass.
func outcomeProto(o check.Outcome) checkspb.Outcome {
	switch o {
	case check.Pass:
		return checkspb.Outcome_OUTCOME_PASS
	case check.Fail:
		return checkspb.Outcome_OUTCOME_FAIL
	case check.NoLimit:
		return checkspb.Outcome_OUTCOME_NO_LIMIT
	case check.NotConsidered:
		return checkspb.Outcome_OUTCOME_NOT_CONSIDERED
	case check.Inconclusive:
		return checkspb.Outcome_OUTCOME_INCONCLUSIVE
	default:
		return checkspb.Outcome_OUTCOME_UNSPECIFIED
	}
}

func outcomeFromProto(o checkspb.Outcome) check.Outcome {
	switch o {
	case checkspb.Outcome_OUTCOME_PASS:
		return check.Pass
	case checkspb.Outcome_OUTCOME_FAIL:
		return check.Fail
	case checkspb.Outcome_OUTCOME_NO_LIMIT:
		return check.NoLimit
	case checkspb.Outcome_OUTCOME_NOT_CONSIDERED:
		return check.NotConsidered
	case checkspb.Outcome_OUTCOME_INCONCLUSIVE:
		return check.Inconclusive
	default:
		return ""
	}
}

func witnessProto(w *check.Witness) *checkspb.Witness {
	if w == nil {
		return nil
	}
	var terms []*checkspb.WitnessTerm
	if len(w.Terms) > 0 {
		terms = make([]*checkspb.WitnessTerm, 0, len(w.Terms))
		for _, t := range w.Terms {
			terms = append(terms, &checkspb.WitnessTerm{Label: t.Label, Value: t.Value})
		}
	}
	return &checkspb.Witness{
		Statement: w.Statement,
		Terms:     terms,
		Datasheet: datasheetCitationProtos(w.Datasheet),
	}
}

func witnessFromProto(p *checkspb.Witness) *check.Witness {
	if p == nil {
		return nil
	}
	var terms []check.WitnessTerm
	if len(p.GetTerms()) > 0 {
		terms = make([]check.WitnessTerm, 0, len(p.GetTerms()))
		for _, t := range p.GetTerms() {
			terms = append(terms, check.WitnessTerm{Label: t.GetLabel(), Value: t.GetValue()})
		}
	}
	return &check.Witness{
		Statement: p.GetStatement(),
		Terms:     terms,
		Datasheet: datasheetCitationsFromProto(p.GetDatasheet()),
	}
}

// contextSubjectsFromProto is the inverse of contextSubjectProtos, PRESERVING ORDER for the same
// reason.
func contextSubjectsFromProto(ps []*checkspb.ContextSubject) []check.ContextSubject {
	if len(ps) == 0 {
		return nil
	}
	out := make([]check.ContextSubject, 0, len(ps))
	for _, p := range ps {
		s := p.GetSubject()
		out = append(out, check.ContextSubject{Entity: check.Entity{Kind: s.GetKind(), Ref: s.GetRef(), Pin: s.GetPin(), NetID: s.GetNetId()}, Role: p.GetRole()})
	}
	return out
}

// datasheetCitationsFromProto is the inverse of datasheetCitationProtos, needed because a Verdict
// round-trips under C26. FindingProto has no inverse, so its field coverage is checked only by
// review.
func datasheetCitationsFromProto(ps []*checkspb.DatasheetCitation) []*check.DatasheetCitation {
	if len(ps) == 0 {
		return nil
	}
	out := make([]*check.DatasheetCitation, 0, len(ps))
	for _, p := range ps {
		if p == nil {
			continue
		}
		out = append(out, &check.DatasheetCitation{
			Doc:              p.GetDoc(),
			DocRef:           p.GetDocRef(),
			Page:             p.GetPage(),
			Section:          p.GetSection(),
			Method:           p.GetMethod(),
			Confidence:       p.GetConfidence(),
			Verification:     p.GetVerification(),
			VerifiedRevision: p.GetVerifiedRevision(),
			Corpus:           p.GetCorpus(),
		})
	}
	return out
}
