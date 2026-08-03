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
