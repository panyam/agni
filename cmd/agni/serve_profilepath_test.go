package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
)

const overlayProfileYAML = `
name: TESTBUS
signals:
  - {name: A, suffix: _TBA, anchor: true}
  - {name: B, suffix: _TBB}
requirements:
  - {type: signal-dangling}
`

func writeProfileDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "testbus.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ruleNames lists the rule names the served CheckService advertises, which is what the web check
// panel populates from.
func ruleNames(t *testing.T, dir string) []string {
	t.Helper()
	overlay, err := loadOverlayProfiles(dir)
	if err != nil {
		t.Fatalf("loadOverlayProfiles: %v", err)
	}
	svc := service.NewCheckService(nil, serveCheckCatalog(overlay), nil)
	resp, err := svc.ListRules(context.Background(), &webapi.ListRulesRequest{})
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	var out []string
	for _, r := range resp.GetRules() {
		out = append(out, r.GetName())
	}
	return out
}

// WS3-048: an overlay profile passed to `agni serve --profile-path` must reach the CHECK surface, not
// only the review one. Before this, serve handed the CheckService a bare DefaultCatalog, so a
// customer's profile fired on the CLI and was invisible in the viewer — which is exactly where the
// interface-aware view presents it.
func TestServeCheckCatalogIncludesOverlayProfiles(t *testing.T) {
	names := ruleNames(t, writeProfileDir(t, overlayProfileYAML))
	want := "profile-overlay/testbus-signal-dangling"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
	// The built-ins must still be there: composing the overlay ADDS a source, it does not replace one.
	if !slicesContains(names, "single-pin-net") {
		t.Errorf("overlay composition dropped the built-ins: %v", names)
	}
}

// With no --profile-path the served catalog is the built-ins alone, so a server started without the
// flag advertises nothing extra.
func TestServeCheckCatalogWithoutProfilePath(t *testing.T) {
	names := ruleNames(t, "")
	for _, n := range names {
		if strings.HasPrefix(n, "profile-overlay/") {
			t.Errorf("no --profile-path should mean no overlay rules, got %q", n)
		}
	}
	if !slicesContains(names, "single-pin-net") {
		t.Error("built-in rules should still be advertised")
	}
}

// A malformed overlay profile fails serve STARTUP with the teaching error, rather than being skipped
// into a server that silently checks less than the operator asked for.
func TestServeBadProfileFailsStartup(t *testing.T) {
	dir := writeProfileDir(t, "name: X\nsignals: [{name: A, suffix: _A, anchor: true}]\nrequirements: [{type: nope}]\n")
	_, err := loadOverlayProfiles(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown requirement type") {
		t.Fatalf("want the teaching error at startup, got: %v", err)
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
