package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vendorFixture is a kicad-cli ERC report captured over the showcase schematic. It is committed rather
// than produced by shelling out to kicad-cli, so the test runs anywhere CI does; the capture itself is
// verified by hand against the live tool.
const vendorFixture = "../../core/results/foreign/testdata/erc.json"

// TestImportJoinsToTheDesignAndReportsTheRest is the end-to-end join, run against a REAL design through
// the reader rather than a hand-built model — the same reason TestConformance lives at the CLI edge.
//
// The assertion that matters is the second one: whatever the import cannot attach must be COUNTED. An
// importer that dropped its residue would report fewer problems than the tool found, with nothing to
// say so, which is the failure this whole contract exists to rule out.
func TestImportJoinsToTheDesignAndReportsTheRest(t *testing.T) {
	out := filepath.Join(t.TempDir(), "imported.json")
	runCLI(t, importResultsCmd(),
		"--design", "testdata/conformance/showcase.fires.kicad_pro", "-o", out, vendorFixture)

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Meta struct {
			Producer, ProducerVersion string
			CoverageAxis              bool
		}
		Findings []struct {
			Rule    string
			Subject struct{ Kind, Ref, Pin string }
		}
		ImportSummary struct {
			Findings, Joined int
			Unjoined         []struct {
				Reason   string
				Count    int
				Examples []string
			}
		}
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, b)
	}

	if got.Meta.CoverageAxis {
		t.Error("an imported vendor report must not claim a coverage axis")
	}
	if got.Meta.Producer == "" || got.Meta.ProducerVersion == "" {
		t.Errorf("meta = %+v, want the vendor tool and its version named", got.Meta)
	}
	total := got.ImportSummary.Findings
	unjoined := 0
	for _, u := range got.ImportSummary.Unjoined {
		unjoined += u.Count
		if u.Reason == "" || len(u.Examples) == 0 {
			t.Errorf("residue class %+v carries no reason or no example, so a reader cannot judge it", u)
		}
	}
	if total != len(got.Findings) {
		t.Errorf("summary counts %d findings, document carries %d", total, len(got.Findings))
	}
	if got.ImportSummary.Joined+unjoined != total {
		t.Errorf("joined %d + unjoined %d != %d; some findings are unaccounted for",
			got.ImportSummary.Joined, unjoined, total)
	}
	if got.ImportSummary.Joined == 0 {
		t.Fatal("nothing joined; the description parse is not reaching the model")
	}
	for _, f := range got.Findings {
		if !strings.HasPrefix(f.Rule, "kicad-") {
			t.Errorf("rule %q is not namespaced; a foreign finding must stay visibly foreign", f.Rule)
		}
	}
}

// TestImportNeverInventsASubject pins the wrong-join guard. A parsed ref-des that names no component in
// the loaded design must leave the finding unattached, because attaching a real violation to an
// innocent part is worse than not attaching it at all.
func TestImportNeverInventsASubject(t *testing.T) {
	out := filepath.Join(t.TempDir(), "imported.json")
	// Joined against a DIFFERENT design: the report's entities do not exist here, so every finding
	// that names one must land in the residue rather than in a subject.
	runCLI(t, importResultsCmd(),
		"--design", "testdata/conformance/fires.edn", "-o", out, vendorFixture)

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Findings      []struct{ Subject struct{ Ref string } }
		ImportSummary struct {
			Joined   int
			Unjoined []struct {
				Reason string
				Count  int
			}
		}
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ImportSummary.Joined != 0 {
		t.Errorf("joined %d findings against an unrelated design; a wrong join attaches real violations to innocent entities", got.ImportSummary.Joined)
	}
	for _, f := range got.Findings {
		if f.Subject.Ref != "" {
			t.Errorf("finding carries subject %q that is not in the design", f.Subject.Ref)
		}
	}
	var sawNotInDesign bool
	for _, u := range got.ImportSummary.Unjoined {
		if strings.Contains(u.Reason, "does not contain") {
			sawNotInDesign = true
		}
	}
	if !sawNotInDesign {
		t.Errorf("the residue should distinguish 'understood but absent' from 'shape not recognized': %+v", got.ImportSummary.Unjoined)
	}
}

// TestImportedDocumentRendersLikeAnyOther pins the payoff of having a document contract: an import is an
// ordinary results document, so the existing renderer handles it with no special case.
func TestImportedDocumentRendersLikeAnyOther(t *testing.T) {
	out := filepath.Join(t.TempDir(), "imported.json")
	runCLI(t, importResultsCmd(),
		"--design", "testdata/conformance/showcase.fires.kicad_pro", "-o", out, vendorFixture)
	text := runCLI(t, resultsCmd(), "--format", "text", out)
	if !strings.Contains(text, "kicad-erc/") {
		t.Errorf("rendered import does not show its namespaced rules:\n%s", text)
	}
	if md := runCLI(t, resultsCmd(), "--format", "markdown", out); !strings.Contains(md, "agni check") {
		t.Errorf("markdown render failed:\n%s", md)
	}
}

// TestCompareReportsTheThreeWaySplit is the differential harness end to end: a native run and an
// imported vendor report over the same design, split by entity.
func TestCompareReportsTheThreeWaySplit(t *testing.T) {
	dir := t.TempDir()
	ours := filepath.Join(dir, "ours.json")
	theirs := filepath.Join(dir, "theirs.json")
	design := "testdata/conformance/showcase.fires.kicad_pro"

	runCLI(t, checkCmd(), "--format", "text", "--results-out", ours, design)
	runCLI(t, importResultsCmd(), "--design", design, "-o", theirs, vendorFixture)

	out := runCLI(t, resultsCmd(), ours, "--compare", theirs)
	for _, want := range []string{"entities flagged:", "both", "ours only", "theirs only"} {
		if !strings.Contains(out, want) {
			t.Errorf("comparison is missing %q:\n%s", want, out)
		}
	}
	// Exactly one side lacks a coverage axis, and the report has to say which.
	if n := strings.Count(out, "no coverage axis"); n != 1 {
		t.Errorf("coverage-axis labelling appeared %d times, want 1:\n%s", n, out)
	}
}
