package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projectFixture writes a minimal PROJECT around a copy of a design fixture and returns the path to
// the design folder. The layout is the conventional one, so `project.yaml` declares only a name and
// the config is found beside it: conventions.yaml, params/, profiles/.
//
// It is built here rather than committed as testdata because the point is the config being RESOLVED
// through the descriptors, and a fixture tree makes it easy to add a file to one and forget the other.
func projectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copyIn := func(src, rel string) {
		t.Helper()
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		write(rel, string(b))
	}
	write("project.yaml", "name: prov\ntitle: Provenance fixture\n")
	copyIn("testdata/review/conventions.yaml", "conventions.yaml")
	copyIn("testdata/review/can-broken.edn", "designs/d/board.edn")
	write("designs/d/design.yaml", "name: d\ntitle: D\nentry: board.edn\n")
	// A params directory with one seeded spec, so the corpus is genuinely attached rather than merely
	// declared. Its contents do not matter: RunConfig.params reports that a corpus reached the run.
	copyIn("testdata/intent/params/"+firstEntry(t, "testdata/intent/params"), "params/seed.textproto")
	return filepath.Join(root, "designs", "d")
}

// firstEntry returns the first file name in dir, so the fixture can borrow a real seeded spec without
// this test hard-coding a name that belongs to another test's corpus.
func firstEntry(t *testing.T, dir string) string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if !e.IsDir() {
			return e.Name()
		}
	}
	t.Fatalf("no files in %s", dir)
	return ""
}

// runConfigOf reads the `run` block out of a written results document.
func runConfigOf(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Run     map[string]any `json:"run"`
		Catalog []struct {
			Name string `json:"name"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	// The catalog snapshot is the independent witness that the project's config actually reached the
	// run. Without this check a `run` block full of falses would be indistinguishable from a fixture
	// whose descriptors never resolved, and the test would pass for the wrong reason.
	var sawProjectRule bool
	for _, r := range doc.Catalog {
		if len(r.Name) > 6 && r.Name[:6] == "house/" {
			sawProjectRule = true
		}
	}
	if !sawProjectRule {
		t.Fatalf("the project's convention rules never reached the run; the fixture did not resolve")
	}
	if doc.Run == nil {
		return map[string]any{}
	}
	return doc.Run
}

// TestResultsRunConfigRecordsProjectTiers is the regression this closes, and it is reachable on the
// shipped tutorial project: a design whose PROJECT declares conventions.yaml and params/ was scored
// against both and wrote `"run": {}`.
//
// The failure direction is what makes it worth a test. RunConfig exists so a reader can tell a design
// with no datasheet violations from a run that had no datasheet corpus attached; recording false for a
// corpus that WAS attached makes a clean report look better-founded than it is, and nothing in the
// document contradicts it except the catalog snapshot nobody cross-reads.
func TestResultsRunConfigRecordsProjectTiers(t *testing.T) {
	design := projectFixture(t)
	out := filepath.Join(t.TempDir(), "check.results.json")

	cmd := checkCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	// No --conventions and no --params: every tier below comes from the project's descriptors alone,
	// which is the case the old code recorded as nothing.
	cmd.SetArgs([]string{"--results-out", out, design})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check: %v", err)
	}

	run := runConfigOf(t, out)
	if run["conventions"] != "house" {
		t.Errorf("run.conventions = %v, want \"house\" (the project's conventions.yaml)", run["conventions"])
	}
	if run["params"] != true {
		t.Errorf("run.params = %v, want true (the project declares params/)", run["params"])
	}
}

// TestReviewResultsRunConfigRecordsProjectTiers is the same regression on the review path, which builds
// its RunConfig inside the service rather than at the CLI edge. Both surfaces had it, so both are
// pinned: fixing one and leaving the other is how the two documents come to disagree about one run.
func TestReviewResultsRunConfigRecordsProjectTiers(t *testing.T) {
	design := projectFixture(t)
	out := filepath.Join(t.TempDir(), "review.results.json")

	cmd := reviewCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml", "--results-out", out, design})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("review: %v", err)
	}

	run := runConfigOf(t, out)
	if run["conventions"] != "house" {
		t.Errorf("run.conventions = %v, want \"house\"", run["conventions"])
	}
	if run["params"] != true {
		t.Errorf("run.params = %v, want true", run["params"])
	}
}

// TestCheckProvenanceIsMountRelative pins the SHAPE of provenance.sourceFile: the path a locator
// records is relative to the design's mount, never the absolute path the command was handed.
//
// The failure direction is the reason this is a test rather than a nicety. A results document is
// meant to be written on one machine and re-read on another (architecture/checks-contract), and
// `--results-out` STORES every locator. An absolute source path makes the stored document a record
// of the producer's filesystem, so a reader holding the design still cannot resolve a finding to a
// file, and nothing in the document says why. It also publishes a directory layout that a public
// artifact has no business carrying.
//
// Asserted on an absolute argument specifically, because that is the only case that can regress: a
// relative argument used to come out looking correct by accident.
func TestCheckProvenanceIsMountRelative(t *testing.T) {
	dir := projectFixture(t)
	design := filepath.Join(dir, "board.edn")
	results := filepath.Join(t.TempDir(), "results.json")

	cmd := checkCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--format", "json", "--results-out", results, design})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("check --format json: %v", err)
	}

	var got struct {
		Findings []struct {
			Provenance struct {
				SourceFile string `json:"sourceFile"`
			} `json:"provenance"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if len(got.Findings) == 0 {
		t.Fatalf("the fixture produced no findings, so nothing carries a locator to assert on")
	}
	for _, f := range got.Findings {
		if f.Provenance.SourceFile != "designs/d/board.edn" {
			t.Errorf("provenance.sourceFile = %q, want the mount-relative %q", f.Provenance.SourceFile, "designs/d/board.edn")
		}
	}

	// The stored document is checked separately from the printed one. They are written by different
	// code paths, and it is the stored one that outlives the machine.
	stored, err := os.ReadFile(results)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(filepath.Dir(dir))
	for what, body := range map[string]string{"printed output": out.String(), "results document": string(stored)} {
		if strings.Contains(body, root) {
			t.Errorf("%s carries the absolute fixture root %q", what, root)
		}
	}
}
