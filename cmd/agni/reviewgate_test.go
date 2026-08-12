package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// reviewGateErr runs `review` with the given args and returns the error, which is nil when the gate did
// not trip. It deliberately does NOT t.Fatal on error, which is the one thing runReview cannot do.
func reviewGateErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := reviewCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

// mustGate asserts the run tripped the gate and returns the message, so a caller can check what it
// blamed.
func mustGate(t *testing.T, args ...string) string {
	t.Helper()
	err := reviewGateErr(t, args...)
	if err == nil {
		t.Fatalf("review %v: expected the gate to trip, got exit 0", args)
	}
	if code := exitCode(err); code != gateExitCode {
		t.Fatalf("review %v: gate error mapped to exit %d, want %d (%v)", args, code, gateExitCode, err)
	}
	return err.Error()
}

// mustNotGate asserts the run completed cleanly.
func mustNotGate(t *testing.T, args ...string) {
	t.Helper()
	if err := reviewGateErr(t, args...); err != nil {
		t.Fatalf("review %v: expected exit 0, got %v", args, err)
	}
}

const (
	miniChecklist = "testdata/review/mini.yaml"
	brokenDesign  = "testdata/review/can-broken.edn"
	cleanDesign   = "testdata/profiles/overlay-bus.edn"
)

// answeredFromJSON reads the answered and fail counts out of `review --format json`'s summary block.
// Reading the rendered document rather than recomputing from the items is deliberate: it asserts that
// the number a consumer sees is the same one the gate reads.
func answeredFromJSON(t *testing.T, out string) (answered, fail int) {
	t.Helper()
	var doc struct {
		Summary struct {
			Answered int `json:"answered"`
			Fail     int `json:"fail"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("review --format json: %v\n%s", err, out)
	}
	return doc.Summary.Answered, doc.Summary.Fail
}

// wrapErr wraps an error the way the call chain between the gate and main may, so TestExitCode proves
// the mapping survives it.
func wrapErr(err error) error { return fmt.Errorf("review: %w", err) }

// TestReviewGateIsOptIn is the compatibility guard, and it is the reason both flags default to off.
// `agni review` has always exited 0, so a default-on gate would turn every existing pipeline, both
// tutorial rungs that run it, and the three targets in examples/tutorial-project/Makefile red on a
// tool upgrade — with no flag anyone could have set in advance to opt out.
//
// can-broken.edn genuinely fails two checklist items, so this is the case a default-on gate would trip.
func TestReviewGateIsOptIn(t *testing.T) {
	mustNotGate(t, "--checklist", miniChecklist, brokenDesign)
}

// TestReviewGateFailOnOutcome: the outcome gate trips on a design whose checklist items fail, and stays
// quiet on one whose do not.
func TestReviewGateFailOnOutcome(t *testing.T) {
	msg := mustGate(t, "--checklist", miniChecklist, "--fail-on-outcome", "fail", brokenDesign)
	if !strings.Contains(msg, "2 checklist items") {
		t.Errorf("message should name how many items tripped, got %q", msg)
	}
	mustNotGate(t, "--checklist", miniChecklist, "--fail-on-outcome", "fail", cleanDesign)
}

// TestReviewGateProvisionalIsOptIn pins the decision that a provisional does NOT trip the plain fail
// gate. A provisional is a fail whose findings all rest on mock or below-floor datasheet data, so
// gating on it by default fails CI on data quality rather than on design quality, which teaches a team
// to switch the gate off. It gates when a team names it.
//
// --ratified-floor 1.0 is what manufactures the provisional: it puts the confidence bar above every
// seeded value, so a real failing item reports as resting on untrusted data.
func TestReviewGateProvisionalIsOptIn(t *testing.T) {
	args := []string{"--checklist", miniChecklist, "--ratified-floor", "1.0", brokenDesign}
	// Sanity: this configuration really does produce a provisional, or the rest of the test proves
	// nothing about provisional and merely re-tests the fail gate.
	if out := runReview(t, append([]string{"--format", "json"}, args...)...); !strings.Contains(out, `"provisional"`) {
		t.Fatalf("fixture produced no provisional outcome; this test cannot say anything:\n%s", out)
	}
	mustNotGate(t, append([]string{"--fail-on-outcome", "provisional"}, args...)...)
	// The same run gates once the team asks for it. The comma form is the opt-in.
	msg := mustGate(t, append([]string{"--fail-on-outcome", "fail,provisional"}, args...)...)
	if !strings.Contains(msg, "fail-on-outcome") {
		t.Errorf("message should name the flag that tripped, got %q", msg)
	}
}

// TestReviewGateMinAnswered: the floor trips below and passes at the boundary. can-broken answers 3 of
// its 6 items (1 pass, 2 fail); the other three are not-applicable or not-automated.
func TestReviewGateMinAnswered(t *testing.T) {
	mustNotGate(t, "--checklist", miniChecklist, "--min-answered", "3", brokenDesign)
	msg := mustGate(t, "--checklist", miniChecklist, "--min-answered", "4", brokenDesign)
	for _, want := range []string{"answered 3 of 6", "--min-answered 4"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q, got %q", want, msg)
		}
	}
}

// TestReviewGateSeesWhatSeverityCannot is the acceptance case issue 199 was written for, end to end.
//
// The same design and the same checklist are run twice, differing only in whether the datasheet corpus
// is attached. The answered count DROPS while the failure count does not rise, so --min-answered trips
// and --fail-on-outcome fail does not. That gap is the whole reason review needs a gate of its own:
// `check --fail-on` pivots on severity and can only ever see the second number.
func TestReviewGateSeesWhatSeverityCannot(t *testing.T) {
	// The rails fixture is chosen because its two datasheet-backed items PASS when the corpus is
	// attached. Removing it therefore cannot raise the failure count, which is what makes the two axes
	// visibly disagree instead of merely differing in magnitude.
	const (
		checklist = "testdata/intent/rails-checklist.yaml"
		design    = "testdata/intent/rails.edn"
		params    = "testdata/intent/params"
		intentDoc = "testdata/intent/rails-ok.yaml"
	)
	answered := func(extra ...string) (int, int) {
		t.Helper()
		args := append([]string{"--checklist", checklist, "--intent-path", intentDoc, "--format", "json"}, extra...)
		return answeredFromJSON(t, runReview(t, append(args, design)...))
	}
	withParams, failsWith := answered("--params", params)
	withoutParams, failsWithout := answered()

	if withoutParams >= withParams {
		t.Fatalf("removing the corpus must lower the answered count, got %d -> %d", withParams, withoutParams)
	}
	if failsWithout > failsWith {
		t.Fatalf("this case is only interesting while failures do NOT rise, got %d -> %d", failsWith, failsWithout)
	}
	// The floor is set to what the seeded run answers, so it holds there and trips once the corpus goes.
	floor := strconv.Itoa(withParams)
	mustNotGate(t, "--checklist", checklist, "--intent-path", intentDoc, "--params", params, "--min-answered", floor, design)
	mustGate(t, "--checklist", checklist, "--intent-path", intentDoc, "--min-answered", floor, design)
	// And the severity-shaped gate stays quiet across both, which is the point.
	mustNotGate(t, "--checklist", checklist, "--intent-path", intentDoc, "--fail-on-outcome", "fail", "--min-answered", "0", design)
}

// TestReviewGateRollup: with several designs the gate reads every one of them, not just the first. A
// gate that only worked on the single-design path would be a trap for the case most likely to want it
// (issue 199, design question 4).
func TestReviewGateRollup(t *testing.T) {
	// Ordered clean-first so a gate that only inspected reports[0] would pass and this test would catch
	// it. The reverse order would pass either way.
	msg := mustGate(t, "--checklist", miniChecklist, "--fail-on-outcome", "fail", cleanDesign, brokenDesign)
	if !strings.Contains(msg, "can-broken") {
		t.Errorf("the rollup gate must name the design that tripped, got %q", msg)
	}
	mustNotGate(t, "--checklist", miniChecklist, "--fail-on-outcome", "fail", cleanDesign, cleanDesign)
}

// TestReviewGateAppliesToResultsOut guards the exit path that returned early before the gate existed.
// A pipeline that gates is also the one archiving its results document, so a gate that worked on two of
// three code paths would have missed exactly the caller that wanted it.
func TestReviewGateAppliesToResultsOut(t *testing.T) {
	out := filepath.Join(t.TempDir(), "run.results.json")
	mustGate(t, "--checklist", miniChecklist, "--fail-on-outcome", "fail", "--results-out", out, brokenDesign)
	// The document is still written. A tripped gate reports on a run that happened; suppressing its
	// artifact would leave a red pipeline with nothing to look at.
	if _, err := os.Stat(out); err != nil {
		t.Errorf("--results-out document missing after a tripped gate: %v", err)
	}
}

// TestReviewGateRejectsUnknownOutcome: a typo is an error naming the valid set, never a silently
// disabled gate. A CI config that quietly stopped gating would report a clean pipeline for as long as
// nobody looked, which is the same silence-reads-as-coverage failure the outcome vocabulary removes.
func TestReviewGateRejectsUnknownOutcome(t *testing.T) {
	err := reviewGateErr(t, "--checklist", miniChecklist, "--fail-on-outcome", "failed", brokenDesign)
	if err == nil {
		t.Fatal("an unknown outcome must be an error, not an ignored argument")
	}
	if code := exitCode(err); code != 1 {
		t.Errorf("a bad flag value is a run failure, want exit 1, got %d", code)
	}
	for _, want := range []string{"failed", "not-automated", "provisional"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should quote the bad value and list the valid set, missing %q: %v", want, err)
		}
	}
}

// TestExitCode is the mapping main() applies. It is tested here rather than by spawning a process
// because the decision, not the os.Exit call, is what can be wrong.
func TestExitCode(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("nil -> %d, want 0", got)
	}
	if got := exitCode(errors.New("design would not parse")); got != 1 {
		t.Errorf("plain error -> %d, want 1", got)
	}
	if got := exitCode(&gateError{msg: "tripped"}); got != gateExitCode {
		t.Errorf("gate error -> %d, want %d", got, gateExitCode)
	}
	// Wrapped, because cobra and the call chain may wrap before main sees it.
	if got := exitCode(wrapErr(&gateError{msg: "tripped"})); got != gateExitCode {
		t.Errorf("wrapped gate error -> %d, want %d", got, gateExitCode)
	}
}
