package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckProfilePathFlag: --profile-path compiles an overlay YAML interface profile into the
// catalog for one run (the open-core "author a proprietary interface as config" path). The custom
// TESTBUS profile's signal-dangling requirement fires on the fixture's single-pin BUS_TBB net,
// proving a profile authored entirely in YAML — no Go, no recompile — checks a real design.
func TestCheckProfilePathFlag(t *testing.T) {
	dir := t.TempDir()
	yaml := `
name: TESTBUS
signals:
  - {name: A, suffix: _TBA, anchor: true}
  - {name: B, suffix: _TBB}
requirements:
  - {type: signal-dangling}
`
	if err := os.WriteFile(filepath.Join(dir, "testbus.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--rule", "profile-overlay/testbus-signal-dangling",
		"--profile-path", dir, "testdata/profiles/overlay-bus.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --profile-path: %v", err)
	}
	var got struct {
		Findings []struct {
			Rule    string `json:"rule"`
			Subject struct{ Ref string } `json:"subject"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(got.Findings) != 1 || got.Findings[0].Subject.Ref != "BUS_TBB" {
		t.Fatalf("want one testbus-signal-dangling on BUS_TBB, got %+v\n%s", got.Findings, out.String())
	}
}

// A malformed overlay profile fails the run with a teaching error, not a silent skip.
func TestCheckProfilePathBadYAMLErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"),
		[]byte("name: X\nsignals: [{name: A, suffix: _A, anchor: true}]\nrequirements: [{type: nope}]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := checkCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--profile-path", dir, "testdata/profiles/overlay-bus.edn"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown requirement type") {
		t.Fatalf("want teaching error for unknown requirement type, got: %v", err)
	}
}

// TestCheckConventionsComposeOncePerRun pins that `agni check --conventions` still runs after WS3-107
// made overlay composition EXTEND the service's catalog instead of rebuilding it.
//
// The CLI composes a convention twice for two different jobs: the service composes the authoritative
// catalog from the request, and the CLI needs a local name space to resolve `--rule <config>/<rule>`
// against before it calls. While composing rebuilt the catalog from scratch, doing both was harmless.
// Once it extends the base, handing the service a catalog that ALREADY carries the convention asks it
// to add a source that is present — a duplicate-source error that made the flag unusable. Nothing
// covered this path, so the naive fix broke a shipped flag silently until it was run by hand.
func TestCheckConventionsComposeOncePerRun(t *testing.T) {
	got := runCLI(t, checkCmd(),
		"--conventions", "testdata/review/conventions.yaml",
		"testdata/review/conv-demo.edn")
	if !strings.Contains(got, "house/signal-net-naming") {
		t.Errorf("the convention's rule did not run:\n%s", got)
	}
}

// TestCheckConventionsResolveFacets pins the reason the CLI composes locally at all: selecting a
// convention's rule by name has to resolve against a catalog that contains it.
func TestCheckConventionsResolveFacets(t *testing.T) {
	got := runCLI(t, checkCmd(),
		"--conventions", "testdata/review/conventions.yaml",
		"--rule", "house/signal-net-naming",
		"testdata/review/conv-demo.edn")
	if !strings.Contains(got, "house/signal-net-naming: lowercase_net") {
		t.Errorf("--rule did not resolve against the convention:\n%s", got)
	}
	if strings.Contains(got, "no rules selected") {
		t.Errorf("the facet resolved to nothing:\n%s", got)
	}
}

// TestCheckConventionsAndProfilesTogether pins the combination on the check surface, mirroring the
// review-side coexistence test: both overlay tiers must reach one run.
func TestCheckConventionsAndProfilesTogether(t *testing.T) {
	got := runCLI(t, checkCmd(),
		"--conventions", "testdata/review/conventions.yaml",
		"--profile-path", "testdata/review/profiles",
		"testdata/review/conv-demo.edn")
	for _, want := range []string{"house/signal-net-naming", "profile-overlay/sigbus-signal-missing"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q; an overlay tier was dropped\n%s", want, got)
		}
	}
}
