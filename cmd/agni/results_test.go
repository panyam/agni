package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCLI executes one cobra command with args and returns its stdout.
func runCLI(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// isolatedDesign copies a fixture into a fresh temp dir and returns its path, so a test can DELETE
// the design and still be sure the tracked fixture survives.
func isolatedDesign(t *testing.T, fixture string) string {
	t.Helper()
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), filepath.Base(fixture))
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestResultsRoundTripWithoutTheDesign is the acceptance test for the results half of the checks
// contract (WS3-103): a written document must be SELF-CONTAINED, not a view over live state.
//
// It proves that by deleting the design before re-rendering. Every format the live command emits has
// to come back byte for byte from the document alone — no design file, no re-run of a single rule. A
// document that merely round-tripped its own fields would pass a weaker test while still being
// unreadable to anyone who does not have the design, which is the whole population this artifact
// exists for.
func TestResultsRoundTripWithoutTheDesign(t *testing.T) {
	design := isolatedDesign(t, "testdata/conformance/fires.edn")
	doc := filepath.Join(t.TempDir(), "run.results.json")

	formats := []string{"text", "json", "markdown", "report"}
	live := map[string]string{}
	for _, f := range formats {
		live[f] = runCLI(t, checkCmd(), "--format", f, design)
	}
	// One write, then every format re-rendered from it: the document is not per-format.
	written := runCLI(t, checkCmd(), "--format", "text", "--results-out", doc, design)
	if written != live["text"] {
		t.Errorf("--results-out changed the terminal output:\n got %q\nwant %q", written, live["text"])
	}

	if err := os.Remove(design); err != nil {
		t.Fatal(err)
	}
	for _, f := range formats {
		got := runCLI(t, resultsCmd(), "--format", f, doc)
		if got != live[f] {
			t.Errorf("results --format %s differs from check --format %s\n got:\n%s\nwant:\n%s", f, f, got, live[f])
		}
	}
}

// TestResultsDocumentRecordsTheRun checks the fields that make a document interpretable later: what
// produced it, which revision it is about, and which overlay tiers were in effect. Without those a
// reader cannot tell a design with no datasheet violations from a run that had no datasheet corpus,
// which is the same silence-reads-as-coverage failure the review outcomes exist to prevent.
func TestResultsDocumentRecordsTheRun(t *testing.T) {
	design := isolatedDesign(t, "testdata/conformance/fires.edn")
	path := filepath.Join(t.TempDir(), "run.results.json")
	runCLI(t, checkCmd(), "--format", "text", "--results-out", path, design)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Meta struct {
			Schema, Producer, ProducerVersion, CreatedAt string
		}
		Design struct {
			Source, ContentHash string
		}
		Run struct {
			Params, Profiles, Intent bool
			Conventions              string
		}
		Catalog []struct {
			Name, Severity, Summary string
			Tags                    map[string]string
		}
		Findings []struct{ Rule string }
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("document is not valid JSON: %v\n%s", err, b)
	}
	if got.Meta.Schema != "agni.checks.results/v1" {
		t.Errorf("meta.schema = %q", got.Meta.Schema)
	}
	if got.Meta.Producer != "agni" || got.Meta.ProducerVersion == "" || got.Meta.CreatedAt == "" {
		t.Errorf("meta = %+v, want a named producer with a build identity and a timestamp", got.Meta)
	}
	if got.Design.Source != design {
		t.Errorf("design.source = %q, want %q", got.Design.Source, design)
	}
	if !strings.HasPrefix(got.Design.ContentHash, "sha256:") || len(got.Design.ContentHash) != len("sha256:")+64 {
		t.Errorf("design.contentHash = %q, want a sha256 hex digest", got.Design.ContentHash)
	}
	if got.Run.Params || got.Run.Profiles || got.Run.Intent || got.Run.Conventions != "" {
		t.Errorf("run = %+v, want every overlay tier absent for a bare check", got.Run)
	}
	if len(got.Catalog) == 0 {
		t.Fatal("catalog snapshot is empty; a reader cannot tell a clean design from a run that checked nothing")
	}
	for _, r := range got.Catalog {
		if r.Name == "" || r.Severity == "" || r.Summary == "" {
			t.Errorf("catalog entry %+v is missing identity or prose", r)
		}
	}
	if len(got.Findings) == 0 {
		t.Error("fires.edn should produce findings")
	}
}

// TestResultsDocumentRecordsOverlayTiers pins the other half of the run record: with a convention
// attached, the document names it. The name matters more than a flag would, because it is also the
// namespace the convention's rules are reported under.
func TestResultsDocumentRecordsOverlayTiers(t *testing.T) {
	design := isolatedDesign(t, "testdata/review/conv-demo.edn")
	path := filepath.Join(t.TempDir(), "run.results.json")
	runCLI(t, checkCmd(), "--format", "text", "--conventions", "testdata/review/conventions.yaml", "--results-out", path, design)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Run struct{ Conventions string }
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Run.Conventions == "" {
		t.Error("run.conventions is empty with --conventions supplied; a reader cannot resolve the namespaced rule names in the findings")
	}
}

// TestReviewResultsRoundTripWithoutTheDesign is the review half of the self-containment proof. The
// outcome vocabulary is the part of the document with no incumbent equivalent, so it is the part most
// worth pinning: every item's verdict, note and findings must survive the write.
func TestReviewResultsRoundTripWithoutTheDesign(t *testing.T) {
	design := isolatedDesign(t, "testdata/review/can-broken.edn")
	doc := filepath.Join(t.TempDir(), "review.results.json")

	live := map[string]string{}
	for _, f := range []string{"markdown", "json"} {
		live[f] = runCLI(t, reviewCmd(), "--checklist", "testdata/review/mini.yaml", "--format", f, design)
	}
	liveCoverage := runCLI(t, reviewCmd(), "--checklist", "testdata/review/mini.yaml", "--coverage", design)
	runCLI(t, reviewCmd(), "--checklist", "testdata/review/mini.yaml", "--format", "markdown", "--results-out", doc, design)

	if err := os.Remove(design); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"markdown", "json"} {
		if got := runCLI(t, resultsCmd(), "--format", f, doc); got != live[f] {
			t.Errorf("results --format %s differs from review --format %s\n got:\n%s\nwant:\n%s", f, f, got, live[f])
		}
	}
	if got := runCLI(t, resultsCmd(), "--coverage", doc); got != liveCoverage {
		t.Errorf("results --coverage differs from review --coverage\n got:\n%s\nwant:\n%s", got, liveCoverage)
	}
}

// TestReviewResultsRejectsARollup pins that --results-out refuses a multi-design run rather than
// silently documenting the first. A document claims to be about ONE design (it carries one
// DesignRef), so a rollup has no honest encoding here.
func TestReviewResultsRejectsARollup(t *testing.T) {
	a := isolatedDesign(t, "testdata/review/can-broken.edn")
	b := isolatedDesign(t, "testdata/review/conv-demo.edn")
	cmd := reviewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--checklist", "testdata/review/mini.yaml", "--results-out", filepath.Join(t.TempDir(), "x.json"), a, b})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("--results-out with two designs should error, not document one of them")
	}
	if !strings.Contains(err.Error(), "one design's document") {
		t.Errorf("error = %v, want it to say the document is per-design", err)
	}
}

// TestResultsRejectsAForeignDocument pins the schema gate. Half-reading a document this build does
// not understand would produce a findings list shorter than the run that made it, with nothing to say
// so — the exact silence-as-coverage failure the contract exists to rule out, so an unknown schema is
// an error rather than a best-effort read.
func TestResultsRejectsAForeignDocument(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"future.json":  `{"meta":{"schema":"agni.checks.results/v9"},"findings":[]}`,
		"notadoc.json": `{"findings":[]}`,
	}
	for name, body := range cases {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := resultsCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{p})
		if err := cmd.Execute(); err == nil {
			t.Errorf("%s: rendering should fail, got %q", name, out.String())
		}
	}
}
