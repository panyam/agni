package main

import (
	"context"
	"github.com/panyam/agni/stdlib/rules/intent"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check/naming"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

const overlayProfileYAML = `
name: TESTBUS
signals:
  - {name: A, suffix: _TBA, anchor: true}
  - {name: B, suffix: _TBB}
requirements:
  - {type: signal-dangling}
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

// servedRuleNames lists the rule names the served CheckService advertises, built through the same
// serveRuleServices call serve's startup makes. That is what the web check panel and ListRules
// populate from.
//
// It goes through serveRuleServices rather than composing a catalog here because an earlier
// version of this helper built its own catalog and handed it to NewCheckService, which meant a
// mutation replacing serve's catalog with a bare DefaultCatalog survived every test in this file.
//
// It skips naming.ApplyLexicon, which serve also does at startup, because the lexicon is a process
// global and installing it here would leak one test's vocabulary into the next while testing
// nothing this file is about.
func servedRuleNames(t *testing.T, profileDir, conventionsPath string) []string {
	t.Helper()
	var cfg *configpb.NamingConvention
	if conventionsPath != "" {
		loaded, err := naming.Load(conventionsPath)
		if err != nil {
			t.Fatalf("naming.Load: %v", err)
		}
		cfg = loaded
	}
	svc, _, err := serveRuleServices(nil, service.NewMemReviewStore(), nil, profileDir, cfg, nil, nil)
	if err != nil {
		t.Fatalf("serveRuleServices: %v", err)
	}
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
// customer's profile fired on the CLI and was invisible in the viewer, where the
// interface-aware view presents it.
func TestServeCheckCatalogIncludesOverlayProfiles(t *testing.T) {
	names := servedRuleNames(t, writeProfileDir(t, overlayProfileYAML), "")
	want := "profile-overlay/testbus-signal-dangling"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
	// The built-ins must still be there, because composing the overlay ADDS a source and does not
	// replace one.
	if !slicesContains(names, "single-pin-net") {
		t.Errorf("overlay composition dropped the built-ins: %v", names)
	}
}

// WS3-109: a --conventions config carrying RULES had those rules composed into the review catalog
// only. Its lexicon already reached both surfaces through the startup ApplyLexicon, which is what made
// the rules half easy to miss.
func TestServeCheckCatalogIncludesConventionRules(t *testing.T) {
	names := servedRuleNames(t, "", writeFile(t, "conventions.yaml", overlayConventionsYAML))
	want := "house/signal-net-naming"
	if !slicesContains(names, want) {
		t.Fatalf("served check catalog should advertise %q, got %d rules: %v", want, len(names), names)
	}
}

// This pins the regression that matters most, where serve REBUILT its review catalog when
// --conventions carried rules (check.CatalogWith(src)), which silently dropped the profile sources
// composed just above it. So the combination an operator is most likely to run
// (house conventions plus their own interface profiles) was the one that lost both tiers, with
// nothing in the output to say so. This is the startup-side twin of the service-layer bug WS3-107
// fixed.
func TestServeCatalogKeepsEveryOverlaySourceTogether(t *testing.T) {
	names := servedRuleNames(t,
		writeProfileDir(t, overlayProfileYAML),
		writeFile(t, "conventions.yaml", overlayConventionsYAML))
	for _, want := range []string{
		"profile-overlay/testbus-signal-dangling",
		"house/signal-net-naming",
		"single-pin-net",
	} {
		if !slicesContains(names, want) {
			t.Errorf("composing both overlay flags dropped %q; got %d rules: %v", want, len(names), names)
		}
	}
}

// The REVIEW half of the same guarantee, end to end through the served ReviewService.
//
// The check-surface tests above all read ListRules, so they cannot see a catalog that reached one
// service and not the other, and mutation testing confirmed that starving only the ReviewService
// survived every one of them. This runs an actual review through the service serve hands to
// CreateReview, over the same fixtures as the CLI-side TestReviewOverlayTiersCoexist, and asserts
// all three overlay tiers arrive. Intent is per design, so it rides the request as `agni review
// --intent-path` sends it (agni issue 831). Both fixtures are authored to FAIL rather than pass, because a
// pass is also what a vanished tier would produce on a design with nothing wrong.
func TestServeReviewServiceGetsEveryOverlayTier(t *testing.T) {
	cfg, err := naming.Load("testdata/review/conventions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, reviewSvc, err := serveRuleServices(&localLoader{loader: newLoader()}, service.NewMemReviewStore(), nil,
		"testdata/review/profiles", cfg, nil, nil)
	if err != nil {
		t.Fatalf("serveRuleServices: %v", err)
	}
	man, err := loadManifest("testdata/review/conv.yaml")
	if err != nil {
		t.Fatal(err)
	}
	di, err := intent.LoadFileProto("testdata/review/intent.yaml")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reviewSvc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		Manifest:  service.ManifestProto(man),
		DesignUri: "mount://m/testdata/review/conv-demo.edn",
		Overlay:   &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Intent: di}},
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	outcomes := map[string]string{}
	for _, area := range resp.GetResults().GetAreas() {
		for _, item := range area.GetItems() {
			outcomes[item.GetId()] = item.GetOutcome()
		}
	}
	for id, tier := range map[string]string{
		"16": "the convention's own rule",
		"70": "the request's intent",
		"71": "--profile-path",
	} {
		if outcomes[id] != "fail" {
			t.Errorf("served review lost %s: item %s read %q, want fail (outcomes: %v)", tier, id, outcomes[id], outcomes)
		}
	}
}

// With no overlay flags the served catalog is the built-ins alone, so a server started without them
// advertises nothing extra.
func TestServeCheckCatalogWithoutOverlayFlags(t *testing.T) {
	names := servedRuleNames(t, "", "")
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

// The same holds for conventions: a bad declaration stops the server coming up rather than serving a
// catalog quietly missing a tier the operator asked for. A bad --intent-path fails the CLI command
// before any request is sent.
func TestServeBadIntentAndConventionsFailStartup(t *testing.T) {
	if _, err := intent.LoadFileProto(writeFile(t, "intent.yaml", "name: x\nintent:\n  nets:\n    5V0:\n      protect: [nope]\n")); err == nil {
		t.Error("a bad --intent-path should fail before the request")
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

// WS3-124: a request that carries its own naming convention REPLACES the server's startup one rather
// than stacking on it, end to end through the wiring serve actually builds.
//
// This goes through serveRuleServices for the same reason servedRuleNames does. The base
// convention's NAME has to reach the services, and a test that composed an Overlay by hand would
// pass while the production wiring forgot to thread it. That is the failure mode WS3-109 was
// written about.
//
// The stacking behaviour was defensible in isolation and wrong in context. The flag help says a
// request's own config "REPLACES this one for that request", the serve.go comment says it
// overrides, and the LEXICON half of the same config already overrode because it travels with the
// read. One config whose halves compose differently is what let the WS3-102 bug hide.
func TestServedRequestConventionReplacesTheStartupOne(t *testing.T) {
	house, err := naming.Load("testdata/review/conventions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	checkSvc, _, err := serveRuleServices(&localLoader{loader: newLoader()}, service.NewMemReviewStore(), nil, "", house, nil, nil)
	if err != nil {
		t.Fatalf("serveRuleServices: %v", err)
	}
	// The server's own convention is in the catalog it was built with.
	listed, err := checkSvc.ListRules(context.Background(), &webapi.ListRulesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !slicesContains(ruleNamesOf(listed.GetRules()), "house/signal-net-naming") {
		t.Fatal("the server's startup convention is not in the catalog; the fixture or the wiring changed")
	}

	// A request naming its own convention gets ITS rule and not the server's.
	resp, err := checkSvc.GetCheckReport(context.Background(), &webapi.GetCheckReportRequest{
		Uri: "mount://m/testdata/review/conv-demo.edn",
		Overlay: &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: &configpb.NamingConvention{
			Name: "acme",
			Rules: []*configpb.NamingRule{{
				Name: "signal-net-naming", Severity: "warning", Allow: []string{"^ACME_"},
			}},
		}}},
	})
	if err != nil {
		t.Fatalf("GetCheckReport: %v", err)
	}
	var sawAcme, sawHouse bool
	for _, sev := range resp.GetReport().GetSections() {
		for _, rule := range sev.GetRules() {
			switch rule.GetRule() {
			case "acme/signal-net-naming":
				sawAcme = true
			case "house/signal-net-naming":
				sawHouse = true
			}
		}
	}
	if !sawAcme {
		t.Error("the request's own convention rule did not run")
	}
	if sawHouse {
		t.Error("the server's convention rule still ran; a request convention must replace it, not stack on it")
	}
}

func ruleNamesOf(rules []*webapi.RuleInfo) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, r.GetName())
	}
	return out
}
