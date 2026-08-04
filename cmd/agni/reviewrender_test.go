package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestFindingSpecs: findings map to specs by kind (net->path, component->rect, pin), severity picks
// the color, and a finding that repeats (bound to several items) collapses to one spec.
func TestFindingSpecs(t *testing.T) {
	findings := []check.Finding{
		{Kind: check.KindNet, Subject: "CAN_H", NetID: "n1", Severity: "warning"},
		{Kind: check.KindNet, Subject: "CAN_H", NetID: "n1", Severity: "warning"}, // dup: same net, same id
		{Kind: check.KindComponent, Subject: "U1", Severity: "error"},
		{Kind: check.KindPin, Subject: "U2", Pin: "3", Severity: "info"},
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
