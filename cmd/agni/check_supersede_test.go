package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The regression WS3-056 exists for. A naming map that re-binds SOME roles and leaves others at core
// naming used to leave the CORE profile still anchored (the anchor suffix `_CS` was not re-bound) and
// still past its two-signal in-use gate, so it reported every re-bound role as a missing signal at
// severity error while the overlay read the same design clean. Six findings, four of them false, on a
// design that is correct under its own convention.
//
// The asymmetry is the part worth remembering: re-binding the ANCHOR role hides the effect entirely,
// because the core profile then fails to anchor and goes quiet. So the CLOSER a house convention sits
// to the core one, the more false failures augmenting produced.
func TestCheckNamingMapSupersedesCoreProfile(t *testing.T) {
	dir := t.TempDir()
	yaml := "override: SPI_NOR\nsuffixes: {IO0: _DQ0, IO1: _DQ1, IO2: _DQ2, IO3: _DQ3}\n"
	if err := os.WriteFile(filepath.Join(dir, "housespi.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "--profile-path", dir,
		"testdata/profiles/house-spi-partial.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --profile-path: %v", err)
	}
	var got struct {
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}

	// The design satisfies its own convention on all six signals, so no signal-missing is a defect.
	for _, f := range got.Findings {
		if strings.HasSuffix(f.Rule, "-signal-missing") {
			t.Errorf("false failure survived supersession: %s (%s)", f.Rule, f.Severity)
		}
		if strings.HasPrefix(f.Rule, "profile/") {
			t.Errorf("superseded core rule still ran: %s", f.Rule)
		}
	}
	// The one genuine defect (chip-select reaches no rail) is still reported, exactly once. Suppressing
	// the duplicate must not suppress the finding.
	var pullup int
	for _, f := range got.Findings {
		if f.Rule == "profile-overlay/spi_nor-missing-pullup" {
			pullup++
		}
	}
	if pullup != 1 {
		t.Errorf("missing-pullup reported %d times, want exactly 1\nfindings: %+v", pullup, got.Findings)
	}
}

// Supersession removes rules, and a removed rule produces no output at all. The run must SAY what it
// dropped, or a clean report is indistinguishable from one whose rules were silently taken away.
func TestCheckReportsSupersededRules(t *testing.T) {
	dir := t.TempDir()
	yaml := "override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"
	if err := os.WriteFile(filepath.Join(dir, "housespi.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := checkCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--format", "json", "--profile-path", dir,
		"testdata/profiles/house-spi-partial.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --profile-path: %v", err)
	}
	note := stderr.String()
	if !strings.Contains(note, "profile-overlay supersedes") || !strings.Contains(note, "profile/spi_nor-signal-missing") {
		t.Errorf("stderr does not name the superseded rules:\n%s", note)
	}
}

// The note is a run diagnostic, not a finding: it must stay out of the serialized findings stream that
// --format json produces, the same posture warnOverBroadProfiles has.
func TestSupersessionNoteIsNotAFinding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "housespi.yaml"),
		[]byte("override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json", "--profile-path", dir,
		"testdata/profiles/house-spi-partial.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --profile-path: %v", err)
	}
	if strings.Contains(out.String(), "supersedes") {
		t.Errorf("the supersession note leaked into the findings stream:\n%s", out.String())
	}
}
