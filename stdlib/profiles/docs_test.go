package profiles

import (
	"regexp"
	"strings"
	"testing"
)

// TestRuleDocsOneToOne holds the requirement docs and docs/ to each other. Every requirement the
// compiler documents has a file that opens with its heading, and docs/ has no orphan .md or missing
// image (mirrors the docs_test in stdlib/rules/builtin and stdlib/rules/datalog).
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
	// Diagrams live under docs/images/ and are referenced as "images/<file>", as in
	// stdlib/rules/builtin (agni issue 75). A family with no diagram yet has no images/ directory.
	images := map[string]bool{}
	if imgEntries, err := ruleDocs.ReadDir("docs/images"); err == nil {
		for _, e := range imgEntries {
			if name := e.Name(); strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".svg") {
				images["images/"+name] = true
			}
		}
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue // the images/ subdir is not a card
		}
		if r := strings.TrimSuffix(e.Name(), ".md"); !want[r] {
			t.Errorf("docs/%s is not a known requirement (orphan doc)", e.Name())
		}
	}
	imgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	referenced := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, _ := ruleDocs.ReadFile("docs/" + e.Name())
		for _, m := range imgRe.FindAllStringSubmatch(string(b), -1) {
			referenced[m[1]] = true
			if !images[m[1]] {
				t.Errorf("docs/%s references missing image %q", e.Name(), m[1])
			}
		}
	}
	// An image no card references ships in the binary and the docsite and is never shown (agni issue
	// 1517).
	for img := range images {
		if !referenced[img] {
			t.Errorf("docs/%s is referenced by no card; delete it or reference it", img)
		}
	}
}

// TestDocRules holds profiles.DocRules to the requirement-type set the docsite catalog generator
// projects, one entry per requirement, each with a non-empty caption and its Detail from ruleDoc. A
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

// cardDiagramExempt lists each card in docs/ that may ship without a diagram, with the reason (agni
// issue 75). Every other card must reference an image. TestRequirementCardsHaveDiagram fails on an
// entry whose card now references one, and on an entry naming no card, so the list shrinks as
// drawings are added.
var cardDiagramExempt = map[string]string{}

// TestRequirementCardsHaveDiagram holds every card in docs/ to the diagram convention (agni issue
// 75). A card references an image, or cardDiagramExempt names it with a reason. TestRuleDocsOneToOne
// already checks that a referenced image exists. This test checks that a card references one at all.
func TestRequirementCardsHaveDiagram(t *testing.T) {
	entries, err := ruleDocs.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	imgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	cards := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		card := strings.TrimSuffix(e.Name(), ".md")
		cards[card] = true
		b, err := ruleDocs.ReadFile("docs/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		drawn := imgRe.Match(b)
		reason, exempt := cardDiagramExempt[card]
		switch {
		case drawn && exempt:
			t.Errorf("docs/%s references a diagram, so remove its stale cardDiagramExempt entry (%q)", e.Name(), reason)
		case !drawn && !exempt:
			t.Errorf("docs/%s references no diagram. Draw one and reference it as ![alt](images/<name>.svg), or add %q to cardDiagramExempt with a reason", e.Name(), card)
		case exempt && strings.TrimSpace(reason) == "":
			t.Errorf("cardDiagramExempt[%q] gives no reason", card)
		}
	}
	for card := range cardDiagramExempt {
		if !cards[card] {
			t.Errorf("cardDiagramExempt names %q, which has no docs/%s.md", card, card)
		}
	}
	if len(cards) == 0 {
		t.Fatal("no requirement cards found under docs/, so the diagram check asserted nothing")
	}
}
