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

// A PROJECT's own profiles supersede built-ins too, and that went unreported: the note was taken from
// the catalog the FLAGS built, before withProjectRules composed the project's rules onto it (agni
// issue 450).
//
// Silence here is the expensive kind. Supersession works by REMOVING rules, and a removed rule
// produces no output, so a report whose built-in CAN rules were dropped read exactly like one where
// they ran and found nothing. Every team using the project model was in that state.
func TestCheckReportsAProjectsOwnSupersessions(t *testing.T) {
	proj := t.TempDir()
	writeTutorialLikeProject(t, proj)
	t.Chdir(proj)

	cmd := rootCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	// No --profile-path. The project names its own profiles/ and composes them.
	cmd.SetArgs([]string{"check", "designs/board/board.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check: %v\n%s", err, stderr.String())
	}
	note := stderr.String()
	if !strings.Contains(note, "supersedes") {
		t.Fatalf("a project superseding a built-in must say so:\n%s", note)
	}
	// The note has to name the PROJECT's source, not the flag namespace, or it describes a composition
	// that did not happen.
	if strings.Contains(note, "profile-overlay") {
		t.Errorf("the note names the flag's namespace for a run that passed no flag:\n%s", note)
	}
	if !strings.Contains(note, "profile/can-") {
		t.Errorf("the note should name the built-in CAN rules it dropped:\n%s", note)
	}
}

// TestReviewReportsAProjectsOwnSupersessions is the twin of the test above, for the command that kept
// the defect after PR 467 fixed check. `agni review` took its note from the catalog the FLAGS built,
// which is composed before any project is resolved, so a project whose own profiles dropped built-in
// rules reported nothing at all.
//
// It matters more on review than on check, because a checklist ITEM bound to a superseded rule still
// scores. The item reads pass or not-applicable on the strength of a rule that was removed from the
// run, and the one line that would have said so was the note.
func TestReviewReportsAProjectsOwnSupersessions(t *testing.T) {
	proj := t.TempDir()
	writeTutorialLikeProject(t, proj)
	if err := os.WriteFile(filepath.Join(proj, "review.yaml"),
		[]byte("name: Test checklist\nareas:\n  - name: Interfaces\n    items:\n      - {id: \"I1\", title: the CAN interface is complete, profile: CAN}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(proj)

	cmd := rootCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"review", "designs/board/board.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review: %v\n%s", err, stderr.String())
	}
	note := stderr.String()
	if !strings.Contains(note, "supersedes") {
		t.Fatalf("a project superseding a built-in must say so on review too:\n%s", note)
	}
	if strings.Contains(note, "profile-overlay") {
		t.Errorf("the note names the flag's namespace for a run that passed no flag:\n%s", note)
	}
	if !strings.Contains(note, "profile/can-") {
		t.Errorf("the note should name the built-in CAN rules it dropped:\n%s", note)
	}
}

// TestReviewSupersessionNoteIsNotRepeatedPerDesign: the note moved into the per-design loop, which is
// the only scope where a project has been resolved. Two designs in one project supersede identically,
// so an un-deduped loop says the same line twice and a rollup over a dozen designs is a wall.
func TestReviewSupersessionNoteIsNotRepeatedPerDesign(t *testing.T) {
	proj := t.TempDir()
	writeTutorialLikeProject(t, proj)
	if err := os.WriteFile(filepath.Join(proj, "review.yaml"),
		[]byte("name: Test checklist\nareas:\n  - name: Interfaces\n    items:\n      - {id: \"I1\", title: the CAN interface is complete, profile: CAN}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(proj, "designs", "board2")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"design.yaml": "name: board2\ntitle: Second board\nentry: board.edn\n",
		"board.edn":   minimalEDN,
	} {
		if err := os.WriteFile(filepath.Join(second, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(proj)

	cmd := rootCmd()
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"review", "designs/board/board.edn", "designs/board2/board.edn"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review over two designs: %v\n%s", err, stderr.String())
	}
	if n := strings.Count(stderr.String(), "supersedes"); n != 1 {
		t.Errorf("the identical note appeared %d times over two designs in one project, want 1:\n%s", n, stderr.String())
	}
}
