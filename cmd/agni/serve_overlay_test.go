package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
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

const overlayIntentYAML = `
name: serve overlay intent
protections:
  - {rail: 5V0, kind: discharge}
`

const overlayConventionsYAML = `
name: house
rules:
  - name: signal-net-naming
    severity: warning
    why: "signal nets are UPPER_SNAKE"
    allow: ["^[A-Z][A-Z0-9_]*$"]
`

func writeProfileDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "testbus.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// servedRuleNames lists the rule names the served CheckService advertises, composed the way serve's
// startup composes them. That is what the web check panel and ListRules populate from.
//
// It deliberately skips naming.ApplyLexicon, which serve also does: the lexicon is a process global
// and already reached both surfaces before this change, so installing it here would leak across tests
// while testing nothing this file is about.
func servedRuleNames(t *testing.T, profileDir, intentPath, conventionsPath string) []string {
	t.Helper()
	overlay, err := loadOverlayProfiles(profileDir)
	if err != nil {
		t.Fatalf("loadOverlayProfiles: %v", err)
	}
	var extra []check.RuleSource
	if conventionsPath != "" {
		cfg, err := naming.Load(conventionsPath)
		if err != nil {
			t.Fatalf("naming.Load: %v", err)
		}
		if len(cfg.Rules) > 0 {
			src, err := naming.Source(cfg)
			if err != nil {
				t.Fatalf("naming.Source: %v", err)
			}
			extra = append(extra, src)
		}
	}
	catalog, _, err := composeReviewInputsFrom(overlay, intentPath, extra...)
	if err != nil {
		t.Fatalf("composeReviewInputsFrom: %v", err)
	}
	svc := service.NewCheckService(nil, catalog, nil)
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
	names := servedRuleNames(t, writeProfileDir(t, overlayProfileYAML), "", "")
	want := "profile-overlay/testbus-signal-dangling"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
	// The built-ins must still be there: composing the overlay ADDS a source, it does not replace one.
	if !slicesContains(names, "single-pin-net") {
		t.Errorf("overlay composition dropped the built-ins: %v", names)
	}
}

// WS3-109: --intent-path reached the ReviewService catalog alone, so an intent rule ran in a review
// and was absent from the check panel. Absent there is indistinguishable from ran-and-passed.
func TestServeCheckCatalogIncludesIntent(t *testing.T) {
	names := servedRuleNames(t, "", writeFile(t, "intent.yaml", overlayIntentYAML), "")
	want := "intent/protection-discharge"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
}

// WS3-109: a --conventions config carrying RULES had those rules composed into the review catalog
// only. Its lexicon already reached both surfaces through the startup ApplyLexicon, which is what made
// the rules half easy to miss.
func TestServeCheckCatalogIncludesConventionRules(t *testing.T) {
	names := servedRuleNames(t, "", "", writeFile(t, "conventions.yaml", overlayConventionsYAML))
	want := "house/signal-net-naming"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
}

// The regression that matters most: serve REBUILT its review catalog when --conventions carried rules
// (check.CatalogWith(src)), which silently dropped the profile and intent sources composed just above
// it. So the combination an operator is most likely to run — house conventions plus their own
// interface profiles — was the one that lost both tiers, with nothing in the output to say so. This is
// the startup-side twin of the service-layer bug WS3-107 fixed.
func TestServeCatalogKeepsEveryOverlaySourceTogether(t *testing.T) {
	names := servedRuleNames(t,
		writeProfileDir(t, overlayProfileYAML),
		writeFile(t, "intent.yaml", overlayIntentYAML),
		writeFile(t, "conventions.yaml", overlayConventionsYAML))
	for _, want := range []string{
		"profile-overlay/testbus-signal-dangling",
		"intent/protection-discharge",
		"house/signal-net-naming",
		"single-pin-net",
	} {
		if !slicesContains(names, want) {
			t.Errorf("composing all three overlay flags dropped %q; got %d rules: %v", want, len(names), names)
		}
	}
}

// With no overlay flags the served catalog is the built-ins alone, so a server started without them
// advertises nothing extra.
func TestServeCheckCatalogWithoutOverlayFlags(t *testing.T) {
	names := servedRuleNames(t, "", "", "")
	for _, n := range names {
		if strings.HasPrefix(n, "profile-overlay/") || strings.HasPrefix(n, "intent/") || strings.HasPrefix(n, "house/") {
			t.Errorf("no overlay flags should mean no overlay rules, got %q", n)
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

// The same holds for the other two: a bad declaration stops the server coming up rather than serving a
// catalog quietly missing a tier the operator asked for.
func TestServeBadIntentAndConventionsFailStartup(t *testing.T) {
	_, _, err := composeReviewInputsFrom(nil, writeFile(t, "intent.yaml", "name: x\nprotections:\n  - {rail: 5V0, kind: nope}\n"))
	if err == nil {
		t.Error("a bad --intent-path should fail startup")
	}
	// A convention's regexes are validated when it is compiled into a source, not when the YAML is
	// read, so this is the call serve has to reach for the operator to hear about it at startup.
	cfg, err := naming.Load(writeFile(t, "conventions.yaml", "name: house\nrules:\n  - {name: r, allow: [\"([\"]}\n"))
	if err != nil {
		return
	}
	if _, err := naming.Source(cfg); err == nil {
		t.Error("a bad --conventions should fail startup")
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
