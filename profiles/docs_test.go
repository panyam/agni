package profiles

import (
	"regexp"
	"strings"
	"testing"
)

// requirementDocs are the per-requirement doc names the compiler references (ruleDoc). Docs are
// keyed by requirement type, not rule name, because the rule is per-profile but the contract is
// per-requirement.
var requirementDocs = []string{"signal-missing", "missing-pullup", "signal-dangling", "termination"}

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
