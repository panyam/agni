package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const spec = "content/tutorials/runs/coverage-rollup.yaml"

// TestAgniRunCapturesAndCaches is the prototype's acceptance: it runs, it commits the capture, and a
// second call reuses it rather than re-running the engine.
func TestAgniRunCapturesAndCaches(t *testing.T) {
	_ = os.Remove(spec + outputSuffix)
	first := AgniRun(spec)
	if !strings.Contains(first, "13 of 15 covered") {
		t.Fatalf("expected the real rollup, got:\n%s", first)
	}
	out, stamp, err := readOutput(spec + outputSuffix)
	if err != nil {
		t.Fatalf("the capture should have been written: %v", err)
	}
	if stamp == "" || !strings.Contains(out, "13 of 15 covered") {
		t.Errorf("capture should carry a stamp and the body, got stamp=%q", stamp)
	}

	// A second call must be a CACHE HIT. Proven by corrupting the body: if the run happened again the
	// corruption would be overwritten, and it is the reuse that keeps a docs build hermetic.
	if err := os.WriteFile(spec+outputSuffix, []byte(stampPrefix+stamp+"\nSENTINEL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AgniRun(spec); !strings.Contains(got, "SENTINEL") {
		t.Errorf("a fresh capture must be reused, not regenerated:\n%s", got)
	}

	// A stamp mismatch — what an edited fixture or spec produces — regenerates.
	if err := os.WriteFile(spec+outputSuffix, []byte(stampPrefix+"stale\nSENTINEL\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AgniRun(spec); strings.Contains(got, "SENTINEL") {
		t.Error("a stale stamp must regenerate rather than serve the old capture")
	}
}

// TestAgniRunIsolatesTheFixture guards the hazard that bit by hand: rung 11 walks a reader through
// `mv params params-old`, and a run against the real fixture renamed the checked-in directory.
func TestAgniRunIsolatesTheFixture(t *testing.T) {
	tmp := t.TempDir() + "/destructive.yaml"
	if err := os.WriteFile(tmp, []byte("fixture: examples/tutorial-project\nscript: |\n  rm -rf params\n  ls params 2>/dev/null || echo GONE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Driven through the internals, since safeJoin confines AgniRun to the docsite tree.
	raw, _ := os.ReadFile(tmp)
	var s runSpec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	got, err := execute(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "GONE") {
		t.Errorf("the script should have run, got %q", got)
	}
	if _, err := os.Stat("../examples/tutorial-project/params"); err != nil {
		t.Fatalf("the CHECKED-IN fixture was mutated by a run: %v", err)
	}
}

// TestCaptureRulesAreDeclarative covers the three fields that replaced shell plumbing. They are worth
// testing directly because their whole justification is that the fragile mechanics live in Go, where
// they CAN be tested, instead of being re-derived in every spec.
func TestCaptureRulesAreDeclarative(t *testing.T) {
	base := runSpec{Script: "echo out; echo err 1>&2; exit 3"}
	for name, tc := range map[string]struct {
		spec runSpec
		want string
	}{
		"stdout is the default": {runSpec{Script: base.Script}, "out\n"},
		"stderr only":           {runSpec{Script: base.Script, Capture: "stderr"}, "err\n"},
		"both":                  {runSpec{Script: base.Script, Capture: "both"}, "out\nerr\n"},
		"none, for a lesson that is only the exit code": {
			runSpec{Script: base.Script, Capture: "none", Exit: true}, "exit 3\n"},
		"exit is appended to the captured stream": {
			runSpec{Script: base.Script, Exit: true}, "out\nexit 3\n"},
		"match keeps lines by shape": {
			runSpec{Script: "echo alpha; echo beta; echo alpaca", Match: "^alp"}, "alpha\nalpaca\n"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := execute(tc.spec)
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMatchingNothingIsAnError is the guard that makes a shape-based filter safe to rely on. A filter
// that stops matching has to say so: rendering an empty block is how a page ends up teaching from a
// blank space, which is the failure this whole mechanism exists to remove.
func TestMatchingNothingIsAnError(t *testing.T) {
	_, err := execute(runSpec{Script: "echo hello", Match: "^goodbye"})
	if err == nil {
		t.Fatal("a filter selecting no lines must be an error, not an empty block")
	}
	if !strings.Contains(err.Error(), "changed shape") {
		t.Errorf("the error should say what happened, got %v", err)
	}
}

// TestUnknownCaptureIsRejected: a typo must not silently fall back to stdout.
func TestUnknownCaptureIsRejected(t *testing.T) {
	if _, err := execute(runSpec{Script: "echo x", Capture: "stdrr"}); err == nil {
		t.Error("an unknown capture must be rejected")
	}
}

// The stamp has to be a function of COMMITTED content, or no committed value can be right for
// everyone (agni issue 357).
//
// inputHash walked the fixture on disk and hashed whatever it found. The tutorial's own Makefile has
// a `report` target that writes examples/tutorial-project/reports/, which its .gitignore covers, so
// anyone who had followed the tutorial hashed two extra files. Their every gate run rewrote the
// committed output with a different stamp, and a `git checkout --` on a file nobody edited became a
// step in the routine. Regenerating and committing would only have moved the staleness to whoever
// had NOT run the tutorial.
func TestInputHashIgnoresUntrackedFiles(t *testing.T) {
	const fixture = "examples/tutorial-project"
	before, err := inputHash([]byte("spec"), fixture)
	if err != nil {
		t.Fatalf("inputHash: %v", err)
	}

	// A generated file of exactly the shape the tutorial's own `make report` leaves behind.
	stray := filepath.Join("..", fixture, "reports", "gateway", "stray.json")
	if err := os.MkdirAll(filepath.Dir(stray), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stray, []byte(`{"generated":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(stray) })

	after, err := inputHash([]byte("spec"), fixture)
	if err != nil {
		t.Fatalf("inputHash after the stray file: %v", err)
	}
	if after != before {
		t.Errorf("an untracked file changed the stamp (%s -> %s); a committed output can then never be "+
			"current for a checkout that lacks it", before, after)
	}
}

// The positive control for the test above. A hash that ignored the fixture entirely, or returned a
// constant, would satisfy "untracked files do not move it" perfectly while measuring nothing.
func TestInputHashChangesWithTrackedContent(t *testing.T) {
	const fixture = "examples/tutorial-project"
	tracked := filepath.Join("..", fixture, "project.yaml")
	original, err := os.ReadFile(tracked)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.WriteFile(tracked, original, 0o644) })

	before, err := inputHash([]byte("spec"), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tracked, append(original, []byte("\n# edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := inputHash([]byte("spec"), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if after == before {
		t.Error("editing a TRACKED fixture file must move the stamp, or the output would never regenerate")
	}
}

// A fixture git cannot list is an error rather than a silent fall back to walking the directory.
// Falling back would restore the exact bug this file is about, and it would do it invisibly: the
// stamp would start depending on the working tree again with nothing to say so.
func TestInputHashRefusesAnUnlistableFixture(t *testing.T) {
	if _, err := inputHash([]byte("spec"), "no/such/fixture"); err == nil {
		t.Error("a fixture that does not exist must be an error")
	}

	// The case that actually distinguishes the two implementations: a fixture that EXISTS on disk and
	// is tracked by nothing. Walking it produces a confident hash over files no other checkout has;
	// listing it produces nothing, and answering with a hash of no content would be a stamp that looks
	// stable and means nothing. A fixture nobody committed is a mistake, so say so.
	dir := filepath.Join("..", "docsite", "testdata-untracked-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, "design.edn"), []byte("(edif X)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inputHash([]byte("spec"), "docsite/testdata-untracked-fixture"); err == nil {
		t.Error("a fixture with no tracked files must be an error, not a hash of whatever is on disk")
	}
}
