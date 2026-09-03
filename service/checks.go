package service

import (
	"context"
	"fmt"
	"github.com/panyam/agni/artifact"
	"sort"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/datasheet/param"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
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
	if len(cfg.GetRules()) > 0 {
		if _, err := naming.Source(cfg); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidArgument, err)
		}
	}
	return &webapi.GetNamingConventionResponse{Convention: cfg}, nil
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
	// Partition before running. check.Available is the same gate review.Run consults per item, and it
	// is asked HERE with the model — where ListRules asks it with a nil one, because that question is
	// "can this rule ever run" rather than "did it run on this design". The two are different answers
	// and the panel needs the second: a rule that is available in principle and gated in practice is
	// exactly the case a findings list cannot express.
	runnable, skipped := partitionAvailable(rules, m)
	// Verdicts come from the SAME runnable set as the findings, so the considered set describes the
	// run that actually happened rather than the catalog that might have. A rule gated to
	// not-applicable is reported by `skipped` and contributes no verdicts, which is the honest
	// answer: it did not consider anything, because it did not run.
	resp := &webapi.CheckDesignResponse{
		Findings: FindingProtos(check.Run(m, runnable)),
		Verdicts: VerdictProtos(check.RunVerdicts(m, runnable)),
		Skipped:  skipped,
	}
	AnnotateSheets(resp.Findings, BuildGeometry(ctx, s.loader, u, ov.ReadOptions()...), m)
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
	return &checkspb.Finding{Subject: subjectProto(f.Subject), Rule: f.Rule, Severity: f.Severity, Inconclusive: f.Inconclusive, Message: f.Message, Provenance: f.Prov, Datasheets: datasheetCitationProtos(f.DatasheetProv), Context: contextSubjectProtos(f.Context)}
}

// contextSubjectProtos maps a finding's context entities to the wire form, PRESERVING ORDER, because
// the order is the rule author's and matches the order the message names them (agni issue 349).
//
// A bus context entity gets its bus_id set from its ref for the same reason a bus SUBJECT does: a bus
// carries no net, so its name is the only geometry join key it has.
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
// tuples and context entries. One conversion rather than three copies is what keeps the bus rule
// below from being remembered in two places and forgotten in the third.
//
// A bus entity gets its bus_id set from its ref: a bus carries no net, so its name is the only
// geometry join key it has (WS7-042b), and a bus with no drawn geometry resolves to nothing.
func subjectProto(e check.Entity) *checkspb.Subject {
	out := &checkspb.Subject{Kind: e.Kind, Ref: e.Ref, Pin: e.Pin, NetId: e.NetID}
	if e.Kind == check.KindBus {
		out.BusId = e.Ref
	}
	return out
}

// subjectFromProto is the inverse of subjectProto. bus_id is not read back: it is derived from the
// ref, so an inbound one that disagreed would let a producer rename a bus by asserting a second name.
func subjectFromProto(s *checkspb.Subject) check.Entity {
	return check.Entity{Kind: s.GetKind(), Ref: s.GetRef(), Pin: s.GetPin(), NetID: s.GetNetId()}
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
		Doc:              c.Doc,
		DocRef:           c.DocRef,
		Page:             c.Page,
		Section:          c.Section,
		Method:           c.Method,
		Confidence:       c.Confidence,
		Verification:     c.Verification,
		VerifiedRevision: c.VerifiedRevision,
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
// cannot, carrying each skipped rule's own reason.
//
// Running only the runnable half is not an optimisation. check.Run already skips a gated rule, so the
// findings are identical either way; the split exists so the response can REPORT the other half
// rather than leaving the caller to infer it from an absence.
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

// VerdictProto and VerdictFromProto are the conversion pair for the considered set, and they carry a
// C26 round-trip guard (TestVerdictProtoRoundTrip) because check.Verdict is a hand-written Go twin of
// a wire message. The guard is the whole point: a field the converter never learned is absent from
// both sides of any assertion made on the proto, which is how naming.Lexicon and Profile.HostClass
// each shipped a silently dropped field.
//
// Verdict.Finding is DELIBERATELY not on the wire and so not in the round trip. A failing verdict's
// finding travels in CheckDesignResponse.findings as it always has; putting it here too would send
// one defect twice and let the copies disagree. TestVerdictFieldCensus is what keeps that a decision
// rather than an omission: it fails when a field is added to check.Verdict, so the next person has to
// say whether it belongs on the wire instead of discovering later that it never arrived.
func VerdictProto(v check.Verdict) *checkspb.Verdict {
	subjects := make([]*checkspb.Subject, 0, len(v.Subjects))
	for _, e := range v.Subjects {
		subjects = append(subjects, subjectProto(e))
	}
	return &checkspb.Verdict{Subjects: subjects, Id: check.VerdictID(v), Rule: v.Rule, Outcome: outcomeProto(v.Outcome), Witness: witnessProto(v.Witness), Reason: v.Reason, Context: contextSubjectProtos(v.Context)}
}

// VerdictFromProto is the inverse. Id is not read back: it is derived from the other fields, so
// trusting an inbound one would let a producer rename a verdict by asserting a different name.
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
// UNSPECIFIED rather than silently to PASS, because a new outcome reaching a consumer as "fine" is
// the exact false-pass shape verdicts exist to remove.
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
// reason: the order is the rule author's and matches the order the proof names them.
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

// datasheetCitationsFromProto is the inverse of datasheetCitationProtos. It exists because a Verdict
// round-trips under C26 where a Finding never did: FindingProto has no inverse, and its field
// coverage rests on nothing but review of the one call site.
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
		})
	}
	return out
}
