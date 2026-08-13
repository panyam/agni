package main

import (
	"os"
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
