package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/readers/formats"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/review"
)

// fsReviewLoader is a filesystem-backed reviewLoader over the CLI's own review fixtures, so the
// service test runs the SAME inputs cmd/agni's TestReviewCmd asserts — served parity is by
// construction, not a re-encoded copy. It is the one os-touching test helper (production is os-free).
type fsReviewLoader struct{ base string }

func (l fsReviewLoader) Design(_ context.Context, _, path string) (*ir.Design, error) {
	return (&formats.Loader{}).ReadDesign(filepath.Join(l.base, path))
}

func (l fsReviewLoader) Board(_ context.Context, _, path string) (*geom.BoardGeometry, error) {
	return (&formats.Loader{}).BoardGeometry(filepath.Join(l.base, path))
}

func (l fsReviewLoader) Manifest(_ context.Context, _, path string) (review.Manifest, error) {
	f, err := os.Open(filepath.Join(l.base, path))
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

func reviewByName() map[string][]profiles.Profile {
	m := map[string][]profiles.Profile{}
	for _, p := range profiles.Profiles {
		m[p.Name] = append(m[p.Name], p)
	}
	return m
}

func outcomeOf(rep *webapi.ReviewReport, id string) (string, bool) {
	for _, a := range rep.GetAreas() {
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
	return NewReviewService(fsReviewLoader{base: base}, check.DefaultCatalog(), reviewByName(), nil)
}

// TestRunReviewOverFixtures runs the same manifest+design the CLI's TestReviewCmd runs, WITH a
// separate board attached (WS3-089), and checks each item's outcome end to end: the CAN profile
// fails termination + signals, the datasheet rule with no --params is not-applicable, the noted item
// is not-automated, the board item fails on the fires board, and the inline house-rule passes.
func TestRunReviewOverFixtures(t *testing.T) {
	svc := newReviewSvc()
	resp, err := svc.RunReview(context.Background(), &webapi.RunReviewRequest{
		ManifestPath: "review/mini.yaml",
		DesignPath:   []string{"review/can-broken.edn"},
		BoardPath:    "conformance/drc.fires.kicad_pcb",
	})
	if err != nil {
		t.Fatalf("RunReview: %v", err)
	}
	if len(resp.GetReports()) != 1 {
		t.Fatalf("reports = %d, want 1", len(resp.GetReports()))
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
		got, ok := outcomeOf(resp.GetReports()[0], id)
		if !ok {
			t.Errorf("item %s missing from report", id)
			continue
		}
		if got != w {
			t.Errorf("item %s outcome = %q, want %q", id, got, w)
		}
	}
}

// TestRunReviewBoardPathGate: with no board_path the board item is not-applicable (mirroring the CLI);
// attaching the fires board flips it to fail. Pins the served side of WS3-089.
func TestRunReviewBoardPathGate(t *testing.T) {
	svc := newReviewSvc()
	resp, err := svc.RunReview(context.Background(), &webapi.RunReviewRequest{
		ManifestPath: "review/mini.yaml",
		DesignPath:   []string{"review/can-broken.edn"},
	})
	if err != nil {
		t.Fatalf("RunReview: %v", err)
	}
	if got, _ := outcomeOf(resp.GetReports()[0], "b1"); got != "not-applicable" {
		t.Errorf("no board_path: b1 = %q, want not-applicable", got)
	}
}

// TestRunReviewErrors: an empty design list, an unreadable manifest, and a board_path at a non-board
// file each error — the run is all-or-nothing, so a partial read never reports items clean.
func TestRunReviewErrors(t *testing.T) {
	svc := newReviewSvc()
	ctx := context.Background()
	cases := map[string]*webapi.RunReviewRequest{
		"empty design_path": {ManifestPath: "review/mini.yaml"},
		"bad manifest":      {ManifestPath: "review/does-not-exist.yaml", DesignPath: []string{"review/can-broken.edn"}},
		"board at netlist":  {ManifestPath: "review/mini.yaml", DesignPath: []string{"review/can-broken.edn"}, BoardPath: "review/can-broken.edn"},
	}
	for name, req := range cases {
		if _, err := svc.RunReview(ctx, req); err == nil {
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
	p := reviewReportProto(rep)
	if p.GetManifest() != "M" || p.GetDesign() != "d.edn" {
		t.Fatalf("report header = %q/%q", p.GetManifest(), p.GetDesign())
	}
	if len(p.GetAreas()) != 1 || p.GetAreas()[0].GetName() != "A" {
		t.Fatalf("areas = %+v", p.GetAreas())
	}
	items := p.GetAreas()[0].GetItems()
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

func (l stubReviewLoader) Design(context.Context, string, string) (*ir.Design, error) {
	return l.design, nil
}
func (l stubReviewLoader) Board(context.Context, string, string) (*geom.BoardGeometry, error) {
	return nil, nil
}
func (l stubReviewLoader) Manifest(context.Context, string, string) (review.Manifest, error) {
	return l.man, nil
}

func runOneItem(t *testing.T, p profiles.Profile, d *ir.Design) string {
	t.Helper()
	man := review.Manifest{Name: "t", Areas: []review.Area{{Name: "A", Items: []review.Item{
		{ID: "it", Title: "iface", Binding: review.Binding{Profile: p.Name}},
	}}}}
	cat := check.CatalogWith(profiles.Source("t", []profiles.Profile{p}))
	byName := map[string][]profiles.Profile{p.Name: {p}}
	svc := NewReviewService(stubReviewLoader{design: d, man: man}, cat, byName, nil)
	resp, err := svc.RunReview(context.Background(), &webapi.RunReviewRequest{ManifestPath: "m", DesignPath: []string{"d"}})
	if err != nil {
		t.Fatalf("RunReview: %v", err)
	}
	out, _ := outcomeOf(resp.GetReports()[0], "it")
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
