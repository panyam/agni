package service

import (
	"context"
	"github.com/panyam/agni/internal/artifact"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/check/naming"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// conventionProto builds a wire convention with one naming rule, under the given catalog namespace.
func conventionProto(name, ruleName string) *configpb.NamingConvention {
	return &configpb.NamingConvention{
		Name: name,
		Rules: []*configpb.NamingRule{{
			Name:     ruleName,
			Severity: "warning",
			Allow:    []string{"^OK_"},
		}},
	}
}

// startupCatalog is a server built with --conventions: the deployment's own convention spliced onto
// the built-ins, which is what serveRuleServices produces.
func startupCatalog(t *testing.T, name, ruleName string) *check.Catalog {
	t.Helper()
	src, err := naming.Source(&configpb.NamingConvention{
		Name:  name,
		Rules: []*configpb.NamingRule{{Name: ruleName, Severity: "warning", Allow: []string{"^HOUSE_"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := check.DefaultCatalog().With(src)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func ruleNames(cat *check.Catalog) map[string]bool {
	out := map[string]bool{}
	for _, r := range cat.Rules() {
		out[r.Name] = true
	}
	return out
}

// TestRequestConventionOverridesServerDefault is the WS3-124 decision, executable.
//
// A request that names its own convention gets ITS rules and not the server's. The previous
// behaviour stacked them, so a caller asking "what does this board look like under MY vocabulary"
// silently got their rules PLUS the deployment's, and could not turn the latter off.
//
// Stacking was not obviously wrong, but it disagreed with everything around it: the serve flag help
// says a request "may name its own instead", the serve.go comment says it "overrides", and the
// lexicon half of the very same config already overrode, because it travels with the design read.
// One config whose two halves compose differently is the shape that let WS3-102's bug hide.
func TestRequestConventionOverridesServerDefault(t *testing.T) {
	base := startupCatalog(t, "house", "house-nets")
	ov, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conventionProto("acme", "acme-nets")}}, "house")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ov.Catalog(base)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	names := ruleNames(got)
	if !names["acme/acme-nets"] {
		t.Error("the request's own convention rule is missing")
	}
	if names["house/house-nets"] {
		t.Error("the server's convention rule survived a request that named its own; that is stacking, not override")
	}
	// Only the convention is replaced. The built-ins the server composes are not the request's to
	// remove, and a request that dropped them would report a design clean by asking nothing.
	if !names["bulk-cap"] {
		t.Error("a built-in rule was dropped along with the server convention")
	}
}

// TestRequestConventionMayReuseTheServersName is the collision this issue was raised about.
//
// It used to fail outright with `duplicate rule source "house"`, because the request's source was
// ADDED to a catalog that already had one by that name. Under override there is nothing to collide
// with, so the natural case — a project refining the house convention it already uses, keeping the
// name — simply works.
func TestRequestConventionMayReuseTheServersName(t *testing.T) {
	base := startupCatalog(t, "house", "house-nets")
	ov, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conventionProto("house", "house-nets-v2")}}, "house")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ov.Catalog(base)
	if err != nil {
		t.Fatalf("reusing the server's convention name must not error under override: %v", err)
	}
	names := ruleNames(got)
	if !names["house/house-nets-v2"] {
		t.Error("the request's rule is missing")
	}
	if names["house/house-nets"] {
		t.Error("the server's rule of the same source name survived")
	}
}

// TestAbsentRequestConventionKeepsTheServerDefault: the common case is unchanged. A request that
// names no convention gets the deployment's, which is what makes --conventions a default at all.
func TestAbsentRequestConventionKeepsTheServerDefault(t *testing.T) {
	base := startupCatalog(t, "house", "house-nets")
	for name, cfg := range map[string]*webapi.OverlayConfig{
		"nil overlay":   nil,
		"empty overlay": {},
	} {
		ov, err := ComposeOverlay(cfg, "house")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := ov.Catalog(base)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !ruleNames(got)["house/house-nets"] {
			t.Errorf("%s: the server's convention was dropped by a request that named none", name)
		}
	}
}

// TestOverrideOnlyDropsTheNamedBaseConvention: a caller that passes "" gets the additive behaviour
// rather than a silent partial override. This is the CLI's case, whose catalog already has the user's
// own --conventions spliced in and has no separate startup default for a request to replace.
func TestOverrideOnlyDropsTheNamedBaseConvention(t *testing.T) {
	base := startupCatalog(t, "house", "house-nets")
	ov, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conventionProto("acme", "acme-nets")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	// No base convention named: nothing is dropped, so both apply (the pre-WS3-124 behaviour).
	got, err := ov.Catalog(base)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if !ruleNames(got)["house/house-nets"] {
		t.Error("an unnamed base convention was dropped; override must be explicit about what it replaces")
	}
}

// TestDuplicateSourceErrorNamesTheServerFlag: worth keeping regardless of the override decision. A
// collision that CAN still happen (a request convention colliding with an overlay profile source,
// say) must tell the caller where the other source came from. `duplicate rule source "house"` with
// no other context is a bad five minutes.
func TestDuplicateSourceErrorNamesTheServerFlag(t *testing.T) {
	src, err := naming.Source(&configpb.NamingConvention{
		Name:  "house",
		Rules: []*configpb.NamingRule{{Name: "r", Severity: "warning", Allow: []string{"^X"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	base, err := check.DefaultCatalog().With(src)
	if err != nil {
		t.Fatal(err)
	}
	ov, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conventionProto("house", "r2")}}, "")
	if err != nil {
		t.Fatal(err)
	}
	// No base convention named, so the request's "house" collides with the base's "house".
	_, err = ov.Catalog(base)
	if err == nil {
		t.Fatal("want a collision error when the base convention is not named for replacement")
	}
	for _, want := range []string{"house", "--conventions"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("collision error %q does not mention %q; the caller cannot tell where the other source came from", err, want)
		}
	}
}

// fsConventionLoader reads convention configs from a real directory, so the resolver test can point
// at a malformed one.
type fsConventionLoader struct{ dir string }

func (l fsConventionLoader) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	return naming.Load(filepath.Join(l.dir, uri.Path))
}

func writeConvention(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestGetNamingConvention resolves a stored config into the value an OverlayConfig carries, and the
// value it returns actually composes. This is the browser's half of C22: it holds a ref and no
// filesystem, so the read is a named rpc and the run still takes a value.
func TestGetNamingConvention(t *testing.T) {
	dir := t.TempDir()
	writeConvention(t, dir, "house.yaml", `
name: house
lexicon:
  net:
    rail:
      patterns: ["_[0-9]V[0-9]$"]
rules:
  - name: signal-net-naming
    severity: warning
    allow: ["^[A-Z][A-Z0-9_]*$"]
`)
	svc := NewCheckService(nil, check.DefaultCatalog(), nil, "", fsConventionLoader{dir: dir}, nil)
	got, err := svc.GetNamingConvention(context.Background(), &webapi.GetNamingConventionRequest{Uri: "mount://m/house.yaml"})
	if err != nil {
		t.Fatalf("GetNamingConvention: %v", err)
	}
	conv := got.GetConvention()
	if conv.GetName() != "house" {
		t.Errorf("name = %q, want house", conv.GetName())
	}
	if len(conv.GetRules()) != 1 || conv.GetRules()[0].GetName() != "signal-net-naming" {
		t.Errorf("rules = %+v", conv.GetRules())
	}
	// The LEXICON half has to survive too. It is the half that reaches the design read, and a
	// resolver that dropped it would hand back a convention that compiles rules and leaves every
	// other rule blind to the project's rail names.
	if pats := conv.GetLexicon().GetNet().GetRail().GetPatterns(); len(pats) != 1 || pats[0] != "_[0-9]V[0-9]$" {
		t.Errorf("lexicon rail patterns = %v, want the config's", pats)
	}
	// And it must be usable as-is: this is the exact round trip the browser performs.
	if _, err := ComposeOverlay(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conv}}, ""); err != nil {
		t.Errorf("the resolved convention does not compose: %v", err)
	}
}

// TestGetNamingConventionRejectsBadInput: an absent ref, an absent file, and a config whose patterns
// will not compile are each an error HERE, so a client learns once rather than on every run that
// carries it. The malformed case is the one that matters: naming.Load parses, but a bad regex only
// fails when the config is USED.
func TestGetNamingConventionRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	writeConvention(t, dir, "bad-regex.yaml", "name: x\nrules:\n  - name: r\n    allow: [\"^(unclosed\"]\n")
	writeConvention(t, dir, "bad-class.yaml", "name: x\nlexicon:\n  class:\n    not_a_real_class:\n      patterns: [\"^X\"]\n")
	svc := NewCheckService(nil, check.DefaultCatalog(), nil, "", fsConventionLoader{dir: dir}, nil)
	ctx := context.Background()
	for name, ref := range map[string]string{
		"empty ref":      "",
		"absent file":    "nope.yaml",
		"pattern to nil": "bad-regex.yaml",
		"unknown class":  "bad-class.yaml",
	} {
		if _, err := svc.GetNamingConvention(ctx, &webapi.GetNamingConventionRequest{Uri: "mount://m/" + ref}); err == nil {
			t.Errorf("%s: want an error, got nil", name)
		}
	}
}

// TestGetNamingConventionNeedsALoader: a service built without a convention loader says so rather
// than panicking. That is the CLI's construction, which reads its own config at the edge.
func TestGetNamingConventionNeedsALoader(t *testing.T) {
	svc := NewCheckService(nil, check.DefaultCatalog(), nil, "", nil, nil)
	if _, err := svc.GetNamingConvention(context.Background(), &webapi.GetNamingConventionRequest{Uri: "mount://m/x.yaml"}); err == nil {
		t.Error("want an error from a service with no convention loader")
	}
}
