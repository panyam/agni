package builtin

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// TestRuleDocsOneToOne holds the built-in catalog and this package's docs/ to each other
// (WS3-025). Every registered rule's Detail comes from its own docs/<name>.md, every doc
// file names a registered rule, and every image a doc references is present. It is the same
// exhaustiveness discipline as the conformance sidecars. A rule PR without its doc fails here,
// not in review, and so does a doc left orphaned by a rename.
func TestRuleDocsOneToOne(t *testing.T) {
	entries, err := ruleDocs.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bool{}
	for _, r := range rules {
		byName[r.Name] = true
		want := ruleDoc(r.Name)
		if r.Detail != want {
			t.Errorf("rule %q: Detail does not come from docs/%s.md (single-source violated)", r.Name, r.Name)
		}
		if !strings.HasPrefix(r.Detail, "## "+r.Name+"\n") {
			t.Errorf("docs/%s.md must open with its own '## %s' heading", r.Name, r.Name)
		}
	}
	imgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	// Diagrams live under docs/images/ and are referenced by that relative path (WS3-025). The census
	// keys each by its referenced form ("images/<file>") so the reference check below is exact.
	images := map[string]bool{}
	imgEntries, err := ruleDocs.ReadDir("docs/images")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range imgEntries {
		if name := e.Name(); strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".svg") {
			images["images/"+name] = true
		}
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue // the images/ subdir and its contents are not rule docs
		}
		rule := strings.TrimSuffix(name, ".md")
		if !byName[rule] {
			t.Errorf("docs/%s names no registered rule (orphan doc)", name)
		}
	}
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
		t.Fatal("no rules registered")
	}
}

// TestRuleDocImageHandler checks that the read-only route serves an embedded diagram (200) as PNG
// or SVG but nothing else. The markdown, a missing image, a top-level (non-images/) path, or a
// non-image path all 404, so the handler never leaks anything but the diagrams (WS9-030).
func TestRuleDocImageHandler(t *testing.T) {
	h := RuleDocImageHandler()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	// A real embedded PNG diagram is served with the PNG magic header.
	ok := get("/images/reach-cases.png")
	if ok.Code != http.StatusOK {
		t.Fatalf("images/reach-cases.png status = %d, want 200", ok.Code)
	}
	if got := ok.Body.Bytes(); len(got) < 8 || string(got[1:4]) != "PNG" {
		t.Errorf("reach-cases.png body is not a PNG (first bytes %q)", firstBytes(ok.Body.Bytes()))
	}
	// An SVG diagram is served with the image/svg+xml content-type (whichever svg exists first).
	if svg := firstSVG(t); svg != "" {
		rec := get("/images/" + svg)
		if rec.Code != http.StatusOK {
			t.Errorf("images/%s status = %d, want 200", svg, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("images/%s content-type = %q, want image/svg+xml", svg, ct)
		}
	}
	// The markdown, a non-image, a missing image, and a top-level (pre-move) path are all 404.
	for _, p := range []string{"/input-protection.md", "/images/nope.png", "/", "/images/reach-cases.txt", "/reach-cases.png"} {
		if code := get(p).Code; code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, code)
		}
	}
}

// firstSVG returns the name of any embedded svg diagram, or "" if none exist yet.
func firstSVG(t *testing.T) string {
	t.Helper()
	entries, err := ruleDocs.ReadDir("docs/images")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".svg") {
			return e.Name()
		}
	}
	return ""
}

func firstBytes(b []byte) []byte {
	if len(b) > 8 {
		return b[:8]
	}
	return b
}

// cardDiagramExempt lists each card in docs/ that may ship without a diagram, with the reason (agni
// issue 75). Every other card must reference an image. TestRuleCardsHaveDiagram fails on an entry
// whose card now references one, and on an entry naming no card, so the list shrinks as drawings
// are added.
var cardDiagramExempt = map[string]string{}

// TestRuleCardsHaveDiagram holds every card in docs/ to the diagram convention (agni issue 75). A
// card references an image under docs/images/, or cardDiagramExempt names it with a reason.
// TestRuleDocsOneToOne already checks that a referenced image exists. This test checks that a card
// references one at all.
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
			t.Errorf("docs/%s references no diagram. Draw one under docs/images/ and reference it as ![alt](images/<name>.svg), or add %q to cardDiagramExempt with a reason", e.Name(), card)
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
