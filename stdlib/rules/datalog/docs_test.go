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
	images := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".png") {
			images[e.Name()] = true
			continue
		}
		if rule := strings.TrimSuffix(e.Name(), ".md"); !byName[rule] {
			t.Errorf("docs/%s names no registered rule (orphan doc)", e.Name())
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
	if len(byName) == 0 {
		t.Fatal("no dl rules registered")
	}
}

// cardDiagramExempt lists each card in docs/ that may ship without a diagram, with the reason (agni
// issue 75). Every other card must reference an image. TestRuleCardsHaveDiagram fails on an entry
// whose card now references one, and on an entry naming no card, so the list shrinks as drawings
// are added.
//
// This family has no docs/images/ yet, and tools/catalogdocs copies no images for it. The first
// diagram here needs that plumbing as well as the drawing.
var cardDiagramExempt = map[string]string{
	"power-pin-mistyped": "not drawn yet (agni issue 75)",
}

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
