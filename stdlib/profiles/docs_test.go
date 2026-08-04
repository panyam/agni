package profiles

import (
	"regexp"
	"strings"
	"testing"
)

// TestRuleDocsOneToOne holds the requirement docs and docs/ to each other: every requirement the
// compiler documents has a file that opens with its heading, and docs/ has no orphan .md or missing
// image (mirrors check/docs_test and datalogrules/docs_test).
func TestRuleDocsOneToOne(t *testing.T) {
	want := map[string]bool{}
	for _, r := range requirementDocs {
		want[r] = true
		if d := ruleDoc(r); !strings.HasPrefix(d, "## "+r+"\n") {
			t.Errorf("docs/%s.md must open with '## %s'", r, r)
		}
	}
	entries, err := ruleDocs.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	images := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			images[e.Name()] = true
			continue
		}
		if r := strings.TrimSuffix(e.Name(), ".md"); !want[r] {
			t.Errorf("docs/%s is not a known requirement (orphan doc)", e.Name())
		}
	}
	imgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, _ := ruleDocs.ReadFile("docs/" + e.Name())
		for _, m := range imgRe.FindAllStringSubmatch(string(b), -1) {
			if !images[m[1]] {
				t.Errorf("docs/%s references missing image %q", e.Name(), m[1])
			}
		}
	}
}

// TestDocRules holds profiles.DocRules to the requirement-type set the docsite catalog generator
// projects: one entry per requirement, each with a non-empty caption and its Detail from ruleDoc. A
// new requirement doc without a DocRules caption fails here.
func TestDocRules(t *testing.T) {
	got := DocRules()
	if len(got) != len(requirementDocs) {
		t.Fatalf("DocRules returned %d rules, want %d (one per requirement)", len(got), len(requirementDocs))
	}
	byName := map[string]bool{}
	for _, r := range got {
		byName[r.Name] = true
		if r.Summary == "" {
			t.Errorf("DocRules[%q] has an empty caption", r.Name)
		}
		if r.Detail != ruleDoc(r.Name) {
			t.Errorf("DocRules[%q] Detail does not come from ruleDoc(%q)", r.Name, r.Name)
		}
	}
	for _, req := range requirementDocs {
		if !byName[req] {
			t.Errorf("DocRules has no entry for requirement %q", req)
		}
	}
}
