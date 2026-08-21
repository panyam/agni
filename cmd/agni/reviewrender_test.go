package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/review"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestFindingSpecs: findings map to specs by kind (net->path, component->rect, pin), severity picks
// the color, and a finding that repeats (bound to several items) collapses to one spec.
func TestFindingSpecs(t *testing.T) {
	findings := []check.Finding{
		{Subject: check.Entity{Kind: check.KindNet, Ref: "CAN_H", NetID: "n1"}, Severity: "warning"},
		{Subject: check.Entity{Kind: check.KindNet, Ref: "CAN_H", NetID: "n1"}, Severity: "warning"}, // dup: same net, same id
		{Subject: check.Entity{Kind: check.KindComponent, Ref: "U1"}, Severity: "error"},
		{Subject: check.Entity{Kind: check.KindPin, Ref: "U2", Pin: "3"}, Severity: "info"},
	}
	specs := findingSpecs(findings)
	if len(specs) != 3 {
		t.Fatalf("specs = %d, want 3 (the duplicate net collapses)", len(specs))
	}
	// Net: PATH shape, carries the net id, amber (warning).
	if specs[0].GetNets()[0] != "CAN_H" || specs[0].GetShape() != geom.HighlightShape_HIGHLIGHT_SHAPE_PATH ||
		len(specs[0].GetNetIds()) != 1 || specs[0].GetColor() != "#f59e0b" {
		t.Errorf("net spec = %+v", specs[0])
	}
	// Component: bounding rect, hot red (error).
	if specs[1].GetComponents()[0] != "U1" || specs[1].GetShape() != geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT ||
		specs[1].GetColor() != "#e11d48" {
		t.Errorf("component spec = %+v", specs[1])
	}
	// Pin: default color (info), the pin ref.
	if p := specs[2].GetPins(); len(p) != 1 || p[0].GetRefDes() != "U2" || p[0].GetPin() != "3" || specs[2].GetColor() != "" {
		t.Errorf("pin spec = %+v (color %q)", specs[2].GetPins(), specs[2].GetColor())
	}
}

// TestCompanionPath covers companion resolution: an explicit --companion is honored for one design
// and rejected for several; otherwise a sibling <stem>.eds next to a netlist is auto-detected, a
// design that already draws itself (.eds/.kicad_sch) auto-detects none, and a missing sibling is "".
func TestCompanionPath(t *testing.T) {
	dir := t.TempDir()
	edn := filepath.Join(dir, "d.edn")
	eds := filepath.Join(dir, "d.eds")
	os.WriteFile(edn, []byte("(edif X)"), 0o644)

	// No sibling yet.
	if got, err := companionPath(edn, "", 1); err != nil || got != "" {
		t.Errorf("no sibling: got %q, %v; want \"\"", got, err)
	}
	// Sibling present -> auto-detected.
	os.WriteFile(eds, []byte("(edif X)"), 0o644)
	if got, err := companionPath(edn, "", 1); err != nil || got != eds {
		t.Errorf("sibling: got %q, %v; want %q", got, err, eds)
	}
	// Explicit flag wins for a single design.
	other := filepath.Join(dir, "other.eds")
	os.WriteFile(other, []byte("(edif X)"), 0o644)
	if got, err := companionPath(edn, other, 1); err != nil || got != other {
		t.Errorf("flag: got %q, %v; want %q", got, err, other)
	}
	// Explicit flag with several designs is an error (one companion can't map to N).
	if _, err := companionPath(edn, other, 2); err == nil {
		t.Error("--companion with several designs should error")
	}
	// A missing explicit companion errors.
	if _, err := companionPath(edn, filepath.Join(dir, "nope.eds"), 1); err == nil {
		t.Error("a missing --companion file should error")
	}
	// A design that already carries geometry auto-detects no companion.
	sch := filepath.Join(dir, "d.kicad_sch")
	os.WriteFile(sch, []byte("x"), 0o644)
	if got, _ := companionPath(sch, "", 1); got != "" {
		t.Errorf("a faithful design should auto-detect no companion, got %q", got)
	}
}

// TestReviewRenderCompanion: a netlist design's findings are drawn on its auto-detected sibling .eds
// (companion-demo.eds), joined by net name — the WS1-047 join. The finding on SIGA highlights the
// SIGA wire on the .eds, the summary names the companion, and a matching pair raises no warning.
func TestReviewRenderCompanion(t *testing.T) {
	dir := t.TempDir()
	r := review.Report{Design: "testdata/review/companion-demo.edn", Areas: []review.AreaResult{{
		Items: []review.ItemResult{{Findings: []check.Finding{
			{Subject: check.Entity{Kind: check.KindNet, Ref: "SIGA"}, Severity: "warning"},
		}}},
	}}}
	summary, err := renderReviewImages([]review.Report{r}, sourcesOf([]review.Report{r}), dir, "")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(summary, "companion companion-demo.eds") {
		t.Errorf("summary should name the auto-detected companion:\n%s", summary)
	}
	if strings.Contains(summary, "overlap") {
		t.Errorf("a matching pair should raise no alignment warning:\n%s", summary)
	}
	svg := filepath.Join(dir, "companion-demo", "P1.svg")
	b, err := os.ReadFile(svg)
	if err != nil {
		t.Fatalf("expected companion render at %s: %v", svg, err)
	}
	if !strings.Contains(string(b), "stroke-opacity") {
		t.Error("the SIGA finding did not locate on the companion .eds")
	}
}

// TestReviewRenderCompanionMismatch: an explicit companion whose net names do not overlap the
// design's is flagged (likely a different-revision or wrong file), rather than silently drawing a
// wrong picture. can-broken's CAN_* nets share nothing with companion-demo.eds's SIGA/SIGB.
func TestReviewRenderCompanionMismatch(t *testing.T) {
	dir := t.TempDir()
	r := review.Report{Design: "testdata/review/can-broken.edn", Areas: []review.AreaResult{{
		Items: []review.ItemResult{{Findings: []check.Finding{
			{Subject: check.Entity{Kind: check.KindNet, Ref: "CAN_CANH"}, Severity: "warning"},
		}}},
	}}}
	summary, err := renderReviewImages([]review.Report{r}, sourcesOf([]review.Report{r}), dir, "testdata/review/companion-demo.eds")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(summary, "overlap") {
		t.Errorf("a mismatched companion should raise an alignment warning:\n%s", summary)
	}
}

// TestReviewCmdRender: --render writes an annotated SVG per design that has findings, with the
// findings baked in (a highlight stroke), while the report on stdout stays the normal markdown.
// can-broken is a netlist, so it renders on the auto-layout sheet ("graph"), proving findings locate
// there and not only on faithful geometry.
func TestReviewCmdRender(t *testing.T) {
	dir := t.TempDir()
	cmd := reviewCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml", "testdata/review/can-broken.edn", "--render", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review --render: %v", err)
	}
	// The report on stdout is unchanged (the summary went to stderr).
	if !strings.Contains(out.String(), "termination strategy") {
		t.Errorf("stdout should still be the report, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "rendered annotated") {
		t.Error("the render summary leaked into stdout; it must go to stderr")
	}
	if !strings.Contains(errBuf.String(), "rendered annotated review images") {
		t.Errorf("render summary missing from stderr:\n%s", errBuf.String())
	}
	// One SVG under the design's stem, carrying a baked highlight (the CAN findings).
	svg := filepath.Join(dir, "can-broken", "graph.svg")
	b, err := os.ReadFile(svg)
	if err != nil {
		t.Fatalf("expected annotated SVG at %s: %v", svg, err)
	}
	if !strings.Contains(string(b), "stroke-opacity") {
		t.Error("annotated SVG has no highlight overlay")
	}
}

// sourcesOf gives renderReviewImages the addressable source for each report. In production those
// come from the documents; a test that builds reports directly re-uses the reading name, which is a
// real path here.
func sourcesOf(reports []review.Report) []string {
	out := make([]string, 0, len(reports))
	for _, r := range reports {
		out = append(out, r.Design)
	}
	return out
}
