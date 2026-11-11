package datalog

import (
	"regexp"
	"strings"
	"testing"
)

// TestRuleDocsOneToOne holds the "dl" source's rules and stdlib/rules/datalog/docs to each other,
// the same discipline stdlib/rules/builtin/docs_test.go enforces for the built-ins. Every rule's
// Detail comes from its own docs/<name>.md, every doc names a registered rule, and every image a
// doc references is present. A datalog rule PR without its doc (or a doc orphaned by a rename)
// fails here, not in review.
func TestRuleDocsOneToOne(t *testing.T) {
	entries, err := ruleDocs.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bool{}
	for _, r := range dlRules {
		byName[r.Name] = true
		if r.Detail != ruleDoc(r.Name) {
			t.Errorf("rule %q: Detail does not come from docs/%s.md (single-source violated)", r.Name, r.Name)
		}
		if !strings.HasPrefix(r.Detail, "## "+r.Name+"\n") {
			t.Errorf("docs/%s.md must open with its own '## %s' heading", r.Name, r.Name)
		}
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
		if r := strings.TrimSuffix(e.Name(), ".md"); !byName[r] {
			t.Errorf("docs/%s names no registered rule (orphan doc)", e.Name())
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
	if len(byName) == 0 {
		t.Fatal("no dl rules registered")
	}
}

// cardDiagramExempt lists each card in docs/ that may ship without a diagram, with the reason (agni
// issue 75). Every other card must reference an image. TestRuleCardsHaveDiagram fails on an entry
// whose card now references one, and on an entry naming no card, so the list shrinks as drawings
// are added.
var cardDiagramExempt = map[string]string{}

// TestRuleCardsHaveDiagram holds every card in docs/ to the diagram convention (agni issue 75). A
// card references an image, or cardDiagramExempt names it with a reason. TestRuleDocsOneToOne
// already checks that a referenced image exists. This test checks that a card references one at all.
func TestRuleCardsHaveDiagram(t *testing.T) {
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
		t.Fatal("no rule cards found under docs/, so the diagram check asserted nothing")
	}
}
