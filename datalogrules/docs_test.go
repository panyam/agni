package datalogrules

import (
	"regexp"
	"strings"
	"testing"
)

// TestRuleDocsOneToOne holds the "dl" source's rules and datalogrules/docs to each other, the same
// discipline check/docs_test enforces for the built-ins: every rule's Detail comes from its own
// docs/<name>.md, every doc names a registered rule, and every image a doc references is present.
// A datalog rule PR without its doc (or a doc orphaned by a rename) fails here, not in review.
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
