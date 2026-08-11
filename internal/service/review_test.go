package service

import (
	"context"
	"github.com/panyam/agni/internal/artifact"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/stdlib/profiles"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // registers the built-in EE rules into DefaultCatalog
)

// fsReviewLoader is a filesystem-backed reviewLoader over the CLI's own review fixtures, so the
// service test runs the SAME inputs cmd/agni's TestReviewCmd asserts — served parity is by
// construction, not a re-encoded copy. It is the one os-touching test helper (production is os-free).
type fsReviewLoader struct{ base string }

// Design HONORS the read options rather than discarding them, because that is the whole point of the
// seam: a loader that dropped the lexicon would make a per-request convention silently no-op, and a
// test helper that dropped it would assert nothing.
func (l fsReviewLoader) Design(_ context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	return (&formats.Loader{Lexicon: ReadOpts(opts...).Lexicon}).ReadDesign(filepath.Join(l.base, uri.Path))
}

func (l fsReviewLoader) Board(_ context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	return (&formats.Loader{}).BoardGeometry(filepath.Join(l.base, uri.Path))
}

func (l fsReviewLoader) DesignHash(_ context.Context, uri artifact.URI) (string, error) {
	return "sha256:" + uri.Path, nil
}

func (l fsReviewLoader) Manifest(_ context.Context, uri artifact.URI) (review.Manifest, error) {
	f, err := os.Open(filepath.Join(l.base, uri.Path))
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

// testReviewEnv is the deployment provenance stamped into documents these tests create. The
// version is fixed so a document comparison never depends on the build.
var testReviewEnv = ReviewEnv{ProducerVersion: "test"}

func reviewByName() map[string][]profiles.Profile {
	m := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		m[p.Name] = append(m[p.Name], p)
	}
	return m
}

func outcomeOf(rv *webapi.Review, id string) (string, bool) {
	for _, a := range rv.GetResults().GetAreas() {
		for _, it := range a.GetItems() {
			if it.GetId() == id {
				return it.GetOutcome(), true
			}
		}
	}
	return "", false
}

func newReviewSvc() *ReviewService {
	base := filepath.Join("..", "..", "cmd", "agni", "testdata")
	return NewReviewService(fsReviewLoader{base: base}, NewMemReviewStore(), check.DefaultCatalog(), reviewByName(), nil, testReviewEnv, "")
}

// fixtureManifest reads a checklist fixture and returns its WIRE form, which is how a manifest now
// reaches CreateReview (WS9-050). The tests below go through this rather than naming a path in the
// request, so they exercise the conversion the CLI and the browser both depend on instead of a
// hand-built proto that could drift from what the YAML actually parses to.
func fixtureManifest(t *testing.T, name string) *checkspb.ReviewManifest {
	t.Helper()
	man, err := fsReviewLoader{base: filepath.Join("..", "..", "cmd", "agni", "testdata")}.Manifest(context.Background(), testURI(t, "m", name))
	if err != nil {
		t.Fatalf("load manifest fixture %s: %v", name, err)
	}
	return ManifestProto(man)
}

// TestCreateReviewOverFixtures runs the same manifest+design the CLI's TestReviewCmd runs, WITH a
// separate board attached (WS3-089), and checks each item's outcome end to end: the CAN profile
// fails termination + signals, the datasheet rule with no --params is not-applicable, the noted item
// is not-automated, the board item fails on the fires board, and the inline house-rule passes.
func TestCreateReviewOverFixtures(t *testing.T) {
	svc := newReviewSvc()
	resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		Manifest:  fixtureManifest(t, "review/mini.yaml"),
		DesignUri: "mount://m/review/can-broken.edn",
		BoardUri:  "conformance/drc.fires.kicad_pcb",
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if resp.GetName() == "" {
		t.Fatalf("a created review has no resource name: %+v", resp)
	}
	want := map[string]string{
		"202": "fail",           // CAN termination missing
		"198": "fail",           // CAN bus signal missing
		"196": "not-automated",  // manual note, no binding
		"197": "not-applicable", // datasheet rule, no --params
		"b1":  "fail",           // track-width fires on the attached fires board
		"h1":  "pass",           // inline house-rule query
	}
	for id, w := range want {
		got, ok := outcomeOf(resp, id)
		if !ok {
			t.Errorf("item %s missing from report", id)
			continue
		}
		if got != w {
			t.Errorf("item %s outcome = %q, want %q", id, got, w)
		}
	}
}

// TestCreateReviewBoardURIGate: with no board_ref the board item is not-applicable (mirroring the CLI);
// attaching the fires board flips it to fail. Pins the served side of WS3-089.
func TestCreateReviewBoardURIGate(t *testing.T) {
	svc := newReviewSvc()
	resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		Manifest:  fixtureManifest(t, "review/mini.yaml"),
		DesignUri: "mount://m/review/can-broken.edn",
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	if got, _ := outcomeOf(resp, "b1"); got != "not-applicable" {
		t.Errorf("no board_ref: b1 = %q, want not-applicable", got)
	}
}

// TestCreateReviewErrors: an empty design list, an absent manifest, an INVALID manifest, and a board_ref
// at a non-board file each error — the run is all-or-nothing, so a partial read never reports items
// clean.
//
// The invalid case is the one the value-carrying request made possible (WS9-050). A manifest that
// arrives as a value never passed a parser, so nothing but this service would have rejected an item
// declaring two mutually-exclusive bindings, and the run would have scored a design against whichever
// binding the resolver reached first.
func TestCreateReviewErrors(t *testing.T) {
	svc := newReviewSvc()
	ctx := context.Background()
	twoBindings := &checkspb.ReviewManifest{Name: "t", Areas: []*checkspb.ManifestArea{{
		Name:  "A",
		Items: []*checkspb.ManifestItem{{Id: "i", Binding: &checkspb.ItemBinding{Rule: "bulk-cap", Profile: "CAN"}}},
	}}}
	design := "review/can-broken.edn"
	cases := map[string]*webapi.CreateReviewRequest{
		"empty design_ref": {Manifest: fixtureManifest(t, "review/mini.yaml")},
		"absent manifest":  {DesignUri: design},
		"invalid manifest": {Manifest: twoBindings, DesignUri: design},
		"board at netlist": {Manifest: fixtureManifest(t, "review/mini.yaml"), DesignUri: design, BoardUri: "mount://m/review/can-broken.edn"},
	}
	for name, req := range cases {
		if _, err := svc.CreateReview(ctx, req); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// TestGetReviewManifest resolves a stored checklist into the value CreateReview takes, and the value it
// returns actually runs. Reading a manifest by ref did not disappear with WS9-050; it moved out of
// the run and into its own RPC, and this is the round trip a browser makes.
func TestGetReviewManifest(t *testing.T) {
	svc := newReviewSvc()
	ctx := context.Background()
	got, err := svc.GetReviewManifest(ctx, &webapi.GetReviewManifestRequest{Uri: "mount://m/review/mini.yaml"})
	if err != nil {
		t.Fatalf("GetReviewManifest: %v", err)
	}
	if got.GetManifest().GetName() == "" || len(got.GetManifest().GetAreas()) == 0 {
		t.Fatalf("resolved manifest is empty: %+v", got.GetManifest())
	}
	resp, err := svc.CreateReview(ctx, &webapi.CreateReviewRequest{
		Manifest:  got.GetManifest(),
		DesignUri: "mount://m/review/can-broken.edn",
	})
	if err != nil {
		t.Fatalf("CreateReview with the resolved manifest: %v", err)
	}
	if outcome, ok := outcomeOf(resp, "202"); !ok || outcome != "fail" {
		t.Errorf("item 202 = %q (found=%v), want fail: the resolved manifest did not score the same as the file", outcome, ok)
	}
}

// TestGetReviewManifestErrors: an empty ref and an unreadable one each error. The unreadable-file case
// lives here now rather than on the run, because that is where the read moved.
func TestGetReviewManifestErrors(t *testing.T) {
	svc := newReviewSvc()
	ctx := context.Background()
	cases := map[string]*webapi.GetReviewManifestRequest{
		"empty uri":      {},
		"absent file":    {Uri: "mount://m/review/does-not-exist.yaml"},
		"not a manifest": {Uri: "mount://m/review/can-broken.edn"},
	}
	for name, req := range cases {
		if _, err := svc.GetReviewManifest(ctx, req); err == nil {
			t.Errorf("%s: expected an error, got nil", name)
		}
	}
}

// TestReviewReportProtoMapsAllFields pins the one review.Report -> proto projection: every item's id,
// title, outcome, note, and findings must survive, so the served surface and the CLI (which renders
// the Go review.Report) cannot silently drift on a lost field.
func TestReviewReportProtoMapsAllFields(t *testing.T) {
	rep := review.Report{
		Manifest: "M", Design: "d.edn",
		Areas: []review.AreaResult{{
			Area: review.Area{Name: "A"},
			Items: []review.ItemResult{
				{
					Item:    review.Item{ID: "1", Title: "t1"},
					Outcome: review.Fail,
					Findings: []check.Finding{
						{Rule: "r", Severity: "error", Kind: check.KindNet, Subject: "N", Message: "boom"},
					},
				},
				{Item: review.Item{ID: "2", Title: "t2"}, Outcome: review.NotApplicable, Note: "no tier"},
			},
		}},
	}
	areas := reviewAreaProtos(rep)
	if len(areas) != 1 || areas[0].GetName() != "A" {
		t.Fatalf("areas = %+v", areas)
	}
	items := areas[0].GetItems()
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].GetId() != "1" || items[0].GetTitle() != "t1" || items[0].GetOutcome() != "fail" {
		t.Errorf("item 0 = %+v", items[0])
	}
	if len(items[0].GetFindings()) != 1 || items[0].GetFindings()[0].GetRule() != "r" || items[0].GetFindings()[0].GetMessage() != "boom" {
		t.Errorf("item 0 findings = %+v", items[0].GetFindings())
	}
	if items[1].GetOutcome() != "not-applicable" || items[1].GetNote() != "no tier" {
		t.Errorf("item 1 = %+v", items[1])
	}
}

// stubReviewLoader serves a canned design + manifest, so a WS3-090 test can inject a synthetic
// profile shape (prefix-discriminated, or host-bound) without fixture files.
type stubReviewLoader struct {
	design *ir.Design
	man    review.Manifest
}

func (l stubReviewLoader) Design(context.Context, artifact.URI, ...ReadOption) (*ir.Design, error) {
	return l.design, nil
}
func (l stubReviewLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return nil, nil
}
func (l stubReviewLoader) DesignHash(context.Context, artifact.URI) (string, error) {
	return "", nil
}
func (l stubReviewLoader) Manifest(context.Context, artifact.URI) (review.Manifest, error) {
	return l.man, nil
}

func runOneItem(t *testing.T, p profiles.Profile, d *ir.Design) string {
	t.Helper()
	man := review.Manifest{Name: "t", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "it", Title: "iface", Binding: review.Binding{Profile: p.Name}},
	}}}}
	cat := check.CatalogWith(profiles.Source("t", []profiles.Profile{p}))
	byName := map[string][]profiles.Profile{p.Name: {p}}
	svc := NewReviewService(stubReviewLoader{design: d, man: man}, NewMemReviewStore(), cat, byName, nil, testReviewEnv, "")
	resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{Manifest: ManifestProto(man), DesignUri: "d"})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	out, _ := outcomeOf(resp, "it")
	return out
}

// TestReviewAbsentPrefixProfileNotApplicable (WS3-090 case 1): a prefix-discriminated rule-bearing
// profile whose own nets are absent reads not-applicable, even though foreign nets share its bare
// suffix. Before the fix the loose suffix presence read it "present", the rules (which require the
// prefix) did not fire, and the item false-passed.
func TestReviewAbsentPrefixProfileNotApplicable(t *testing.T) {
	pcie := profiles.Profile{Name: "PXTEST", Signals: []profiles.Signal{
		{Name: "TX", Prefix: "PCIE_", Suffix: "_TX", Anchor: true},
		{Name: "RX", Prefix: "PCIE_", Suffix: "_RX"},
	}, Requirements: []profiles.Requirement{{Type: "signal-missing"}}}
	d := &ir.Design{Nets: []*ir.Net{{Name: "LIN_TX"}, {Name: "CAN_RX"}}}
	if got := runOneItem(t, pcie, d); got != "not-applicable" {
		t.Errorf("absent prefix-discriminated profile: want not-applicable, got %q", got)
	}
}

// TestReviewHostUnannotatedNotAutomated (WS3-090 case 2): a host-bound profile whose host is declared
// on no component and whose convention is not in use reads not-automated — the intended host check
// could not evaluate — not a hollow pass.
func TestReviewHostUnannotatedNotAutomated(t *testing.T) {
	// Prefix-discriminated host-bound profile: its nets are NAMED on the board (bare suffix) but not
	// strictly in use (wrong prefix), and no component declares the host -> the host check is blocked.
	lin := profiles.Profile{Name: "XLIN", HostAttrKey: "interface", HostAttrVal: "XLIN_HOST",
		Signals: []profiles.Signal{
			{Name: "TX", Prefix: "XLIN_", Suffix: "_TX", Anchor: true},
			{Name: "RX", Prefix: "XLIN_", Suffix: "_RX"},
		}, Requirements: []profiles.Requirement{{Type: "host-incomplete"}}}
	d := &ir.Design{Nets: []*ir.Net{{Name: "FOO_TX"}, {Name: "BAR_RX"}}, Components: []*ir.Component{{RefDes: "U1"}}}
	if got := runOneItem(t, lin, d); got != "not-automated" {
		t.Errorf("host-bound unannotated profile: want not-automated, got %q", got)
	}
}

// TestReviewAbsentHostProfileNotApplicable pins the regression the Named gate guards: a host-bound
// interface that is GENUINELY absent (not named at all, no host declared) reads not-applicable, NOT
// not-automated — only a named-but-unresolvable host interface is not-automated.
func TestReviewAbsentHostProfileNotApplicable(t *testing.T) {
	xcan := profiles.Profile{Name: "XCAN", HostAttrKey: "interface", HostAttrVal: "XCAN_HOST",
		Signals: []profiles.Signal{
			{Name: "H", Suffix: "_XCANH", Anchor: true},
			{Name: "L", Suffix: "_XCANL"},
		}, Requirements: []profiles.Requirement{{Type: "host-incomplete"}}}
	d := &ir.Design{Nets: []*ir.Net{{Name: "UNRELATED"}}, Components: []*ir.Component{{RefDes: "U1"}}}
	if got := runOneItem(t, xcan, d); got != "not-applicable" {
		t.Errorf("genuinely-absent host-bound profile: want not-applicable, got %q", got)
	}
}

// TestReviewInUseWithoutAnchorNotAutomated (WS3-099): a profile whose convention is in use through two
// NON-anchor signals, with the anchor net absent, must not read pass. The completeness rule hangs on the
// anchor, so it reports nothing, and zero findings scored a clean pass on an interface nothing checked.
// This is the fifth route to the same defect after WS3-090/096/097/098.
func TestReviewInUseWithoutAnchorNotAutomated(t *testing.T) {
	pcie := profiles.Profile{Name: "PXANCHOR", Signals: []profiles.Signal{
		{Name: "PETP", Suffix: "_PETP", Anchor: true},
		{Name: "REFCLKP", Suffix: "_REFCLKP"},
		{Name: "REFCLKN", Suffix: "_REFCLKN"},
	}, Requirements: []profiles.Requirement{{Type: "signal-missing"}}}
	// Both REFCLK nets are properly wired (two connections each), so signal-dangling has nothing to say
	// either: the item's only verdict comes from the completeness rule, which cannot evaluate.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}},
		Nets: []*ir.Net{
			{Name: "PCIE_NAD_REFCLKP", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}}},
			{Name: "PCIE_NAD_REFCLKN", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}, {ComponentRef: "U2", PinRef: "2"}}},
		},
	}
	if got := runOneItem(t, pcie, d); got != "not-automated" {
		t.Errorf("in-use-but-unanchored profile: want not-automated, got %q", got)
	}
	// The same profile with its anchor net present evaluates normally, so the gate does not swallow a
	// genuinely-checkable interface: PETN is not declared, nothing is missing, clean pass.
	d.Nets = append(d.Nets, &ir.Net{Name: "PCIE_NAD_PETP",
		Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "3"}, {ComponentRef: "U2", PinRef: "3"}}})
	if got := runOneItem(t, pcie, d); got != "pass" {
		t.Errorf("anchored profile with nothing missing: want pass, got %q", got)
	}
}

// TestReviewOverlayProfileResolvesUnmatched (WS3-099) pins the REMEDIATION path for the verdict above:
// a board whose bus is correctly designed but named to a convention the shipped profile cannot express
// reads not-automated, and authoring an overlay profile with WS3-057 matchers makes it evaluate for
// real. The two profiles share the interface Name and load as separate catalog sources, as a built-in
// and a --profile-path overlay do. The core profile stays SILENT rather than clashing: it cannot anchor
// on this naming, so the overlay's finding is the whole verdict.
func TestReviewOverlayProfileResolvesUnmatched(t *testing.T) {
	core := profiles.Profile{Name: "PXOVL", Signals: []profiles.Signal{
		{Name: "PETP", Suffix: "_PETP", Anchor: true},
		{Name: "REFCLKP", Suffix: "_REFCLKP"},
		{Name: "REFCLKN", Suffix: "_REFCLKN"},
	}, Requirements: []profiles.Requirement{{Type: "signal-missing"}}}
	// Same interface, this board's naming, via a glob. TXP0 is required and absent -> a genuine finding.
	overlay := profiles.Profile{Name: "PXOVL", Signals: []profiles.Signal{
		{Name: "OREFCLKP", Glob: "PCIE_*_REFCLKP", Anchor: true},
		{Name: "OREFCLKN", Glob: "PCIE_*_REFCLKN"},
		{Name: "OTXP", Glob: "PCIE_*_TXP0"},
	}, Requirements: []profiles.Requirement{{Type: "signal-missing"}}}
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}, {RefDes: "U2"}},
		Nets: []*ir.Net{
			{Name: "PCIE_NAD_REFCLKP", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}}},
			{Name: "PCIE_NAD_REFCLKN", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}, {ComponentRef: "U2", PinRef: "2"}}},
		},
	}
	if got := runProfiles(t, []profiles.Profile{core}, d); got != "not-automated" {
		t.Errorf("shipped profile alone cannot express this naming: want not-automated, got %q", got)
	}
	if got := runProfiles(t, []profiles.Profile{core, overlay}, d); got != "fail" {
		t.Errorf("overlay profile should make the interface evaluate: want fail, got %q", got)
	}
}

// runProfiles runs a one-item manifest bound to the interface every profile in ps names, loading the
// first as a built-in source and the rest as an overlay source (the production namespacing, which is
// also what keeps their generated rule names distinct).
func runProfiles(t *testing.T, ps []profiles.Profile, d *ir.Design) string {
	t.Helper()
	name := ps[0].Name
	man := review.Manifest{Name: "t", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "it", Title: "iface", Binding: review.Binding{Profile: name}},
	}}}}
	srcs := []check.RuleSource{profiles.Source("profile-core", ps[:1])}
	if len(ps) > 1 {
		srcs = append(srcs, profiles.Source("profile-overlay", ps[1:]))
	}
	svc := NewReviewService(stubReviewLoader{design: d, man: man}, NewMemReviewStore(), check.CatalogWith(srcs...),
		map[string][]profiles.Profile{name: ps}, nil, testReviewEnv, "")
	resp, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{Manifest: ManifestProto(man), DesignUri: "d"})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	out, _ := outcomeOf(resp, "it")
	return out
}

// TestCreateReviewOverlayIsPerRequest is the property the process-global vocabulary could not provide,
// and the reason the lexicon travels with the read (WS3-106): two CreateReview calls in ONE process,
// naming different conventions, must each see their own. Run concurrently and repeatedly so a shared
// mutable vocabulary would show up as a race or a flipped outcome rather than passing by luck.
func TestCreateReviewOverlayIsPerRequest(t *testing.T) {
	svc := NewReviewService(fsReviewLoader{base: "../../cmd/agni/testdata"}, NewMemReviewStore(), check.DefaultCatalog(), reviewByName(), nil, testReviewEnv, "")
	req := func(conventions string) *webapi.CreateReviewRequest {
		r := &webapi.CreateReviewRequest{
			Manifest:  fixtureManifest(t, "review/conv.yaml"),
			DesignUri: "mount://m/review/conv-demo.edn",
		}
		if conventions != "" {
			// The convention travels as a VALUE, so the caller decides where it came from; here that is
			// the same YAML the CLI test uses, read by the test rather than by the service.
			cfg, err := naming.Load(filepath.Join("../../cmd/agni/testdata", conventions))
			if err != nil {
				t.Fatalf("naming.Load: %v", err)
			}
			r.Overlay = &webapi.OverlayConfig{Conventions: ConventionProto(cfg)}
		}
		return r
	}
	// Item 64 is keyed on a pin's electrical type, which ONLY the ingestion stamp can set (no model-side
	// name fallback exists for a pin direction), so this asserts the READ each request performed — not
	// merely the model it built afterwards.
	cases := []struct{ conventions, want string }{
		{"", "pass"},
		{"review/conventions.yaml", "fail"},
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		for _, tc := range cases {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := svc.CreateReview(context.Background(), req(tc.conventions))
				if err != nil {
					t.Errorf("CreateReview(%q): %v", tc.conventions, err)
					return
				}
				got, ok := outcomeOf(resp, "64")
				if !ok {
					t.Errorf("CreateReview(%q): item 64 missing", tc.conventions)
					return
				}
				if got != tc.want {
					t.Errorf("CreateReview(%q): item 64 = %s, want %s (a request saw another's lexicon)", tc.conventions, got, tc.want)
				}
			}()
		}
	}
	wg.Wait()
}
