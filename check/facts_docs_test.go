package check

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// relRe extracts the built-in relation names from their Rel* const declarations in facts.go — the
// authoritative set of EDB relations (the ones with a Go projector, hence the ones a projector→doc
// back-link can couple to). Reading the source keeps the harness single-source with the code it
// guards, the same spirit as docs_test.go reading the registered Rules.
var relRe = regexp.MustCompile(`Rel\w+\s*=\s*"([^"]+)"`)

// backlinkRe extracts a `doc: facts/docs/<file>.md` back-link wherever it appears in facts.go.
var backlinkRe = regexp.MustCompile(`doc:\s*facts/docs/(\S+\.md)`)

func builtinRelationNames(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("facts.go")
	if err != nil {
		t.Fatalf("read facts.go: %v", err)
	}
	names := map[string]bool{}
	for _, m := range relRe.FindAllStringSubmatch(string(src), -1) {
		names[m[1]] = true
	}
	if len(names) == 0 {
		t.Fatal("no Rel* relation constants found in facts.go")
	}
	return names
}

// predicateDocs are non-EDB relation names that earn a doc but have no check/facts.go projector
// (they are computed query built-ins, so they carry no projector→doc back-link and are outside the
// EDB require-all set). reaches is the reach-walk predicate, the recursive counterpart to
// net.bus_like. The string predicates (contains/prefix/suffix) are deliberately not documented here
// (tracked in OUT_OF_SCOPE); add a name to this set when its doc lands.
var predicateDocs = map[string]bool{"reaches": true}

// TestRelationDocsBidirectional couples check/facts/docs to the relation set in both directions
// (WS14-005), the docs_test.go analogue for facts, now REQUIRE-ALL: every built-in EDB relation
// (every Rel* const) must have a doc, so a new relation added without one fails CI (the staged flip
// from PR 287, once the backfill landed). What is enforced: every EDB relation has a doc; every doc
// names a real relation (EDB or an allowlisted predicate) and opens with its own heading; every
// projector→doc back-link resolves; every referenced image exists.
func TestRelationDocsBidirectional(t *testing.T) {
	relations := builtinRelationNames(t)

	entries, err := relationDocs.ReadDir("facts/docs")
	if err != nil {
		t.Fatal(err)
	}

	// doc→relation: every facts/docs/<name>.md (skipping _-prefixed scaffolding like _TEMPLATE.md)
	// names a registered relation and opens with "## <name>".
	docStems := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue // the images/ subdir, and _TEMPLATE.md and friends, are not relation docs
		}
		stem := strings.TrimSuffix(name, ".md")
		docStems[stem] = true
		if !relations[stem] && !predicateDocs[stem] {
			t.Errorf("facts/docs/%s names no built-in relation (orphan doc)", name)
		}
		body := RelationDoc(stem)
		if body == "" {
			t.Errorf("facts/docs/%s did not load via RelationDoc(%q)", name, stem)
		}
		if !strings.HasPrefix(body, "## "+stem+"\n") {
			t.Errorf("facts/docs/%s must open with its own '## %s' heading", name, stem)
		}
	}

	// relation→doc: every `doc: facts/docs/X.md` back-link in facts.go names a real relation and
	// resolves to a present doc file. The back-link is a CHECKED invariant, not a hopeful comment.
	src, _ := os.ReadFile("facts.go")
	for _, m := range backlinkRe.FindAllStringSubmatch(string(src), -1) {
		file := m[1]
		stem := strings.TrimSuffix(file, ".md")
		if !relations[stem] {
			t.Errorf("facts.go back-link doc: facts/docs/%s names no built-in relation", file)
		}
		if !docStems[stem] {
			t.Errorf("facts.go back-links doc: facts/docs/%s but that file is absent", file)
		}
	}

	// images: every ![](images/<file>) a doc references exists under facts/docs/images/.
	images := map[string]bool{}
	imgEntries, err := relationDocs.ReadDir("facts/docs/images")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range imgEntries {
		if n := e.Name(); strings.HasSuffix(n, ".svg") || strings.HasSuffix(n, ".png") {
			images["images/"+n] = true
		}
	}
	imgRe := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), "_") {
			continue // _TEMPLATE.md's placeholder image ref is not a real reference
		}
		b, _ := relationDocs.ReadFile("facts/docs/" + e.Name())
		for _, m := range imgRe.FindAllStringSubmatch(string(b), -1) {
			if strings.HasPrefix(m[1], "images/") && !images[m[1]] {
				t.Errorf("facts/docs/%s references missing image %q", e.Name(), m[1])
			}
		}
	}

	// require-all (the staged flip): every built-in EDB relation must have a doc. A relation added
	// to facts.go without a facts/docs/<name>.md now fails here, the same discipline docs_test.go
	// holds rules to.
	for rel := range relations {
		if !docStems[rel] {
			t.Errorf("relation %q has no doc: write facts/docs/%s.md (require-all)", rel, rel)
		}
	}
	// The allowlisted predicate docs must also be present (they are documented on purpose).
	for pred := range predicateDocs {
		if !docStems[pred] {
			t.Errorf("predicate doc facts/docs/%s.md is missing", pred)
		}
	}
}

// TestRelationDocImageHandler: the read-only route serves an embedded schematic card (200) as PNG
// or SVG but nothing else — the markdown, a missing image, a top-level (non-images/) path, or a
// non-image path all 404, mirroring RuleDocImageHandler (WS14-005).
func TestRelationDocImageHandler(t *testing.T) {
	h := RelationDocImageHandler()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	ok := get("/images/net.bus_like.svg")
	if ok.Code != http.StatusOK {
		t.Fatalf("images/net.bus_like.svg status = %d, want 200", ok.Code)
	}
	if ct := ok.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("svg content-type = %q, want image/svg+xml", ct)
	}
	for _, p := range []string{"/net.bus_like.md", "/images/nope.svg", "/", "/images/net.bus_like.txt"} {
		if code := get(p).Code; code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, code)
		}
	}
}
