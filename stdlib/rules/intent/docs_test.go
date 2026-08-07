package intent

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// kitchenSink is a declaration that exercises EVERY intent rule kind, so compiling it emits at least
// one rule per doc key: module-missing + module-count (a module with a count), voltage-domain-mismatch,
// subsystem-<slug>, both protection kinds, and both net-property kinds. TestRuleDocsOneToOne compiles it to tie the RUNTIME
// rules to their docs (the stronger binding a fully-dynamic source needs over the profiles list-only
// harness: it catches a builder that forgets to set Detail at all, not only a missing file).
func kitchenSink() Declaration {
	return Declaration{
		Name:           "test intent",
		Modules:        []Module{{Name: "MCU", Class: "soc", Count: 2}},
		VoltageDomains: []VoltageDomain{{Name: "io_3v3", Nominal: 3.3, Rails: []string{"3V3"}}},
		Subsystems:     []Subsystem{{Name: "main clock", Nets: []string{"CLK"}}},
		Protections:    []Protection{{Rail: "12V_IN", Kind: ProtectionOVP}, {Rail: "HV_BULK", Kind: ProtectionDischarge}},
		NetProperties: []NetProperty{
			{Net: "SYS_RESET_N", Property: PropResetPolarity, Value: "low"},
			{Net: "PCIE_TX0_P", Property: PropACCoupled},
		},
	}
}

// TestRuleDocsOneToOne holds the intent rule KINDS, the rules Compile emits, and the docs/ directory to
// each other, the same discipline builtin/docs_test and profiles/docs_test enforce, adapted for a
// source whose rules are generated (and partly named) per design. It checks four things:
//   - every docKey has a docs/<key>.md that opens with its own '## <key>' heading;
//   - every rule a kitchen-sink declaration emits gets its Detail from intentDoc(docKey(name)) and is
//     non-empty (the runtime tie: a rule shipped without a doc, or with Detail left unset, fails here);
//   - the emitted docKey set equals docKeys (a new rule kind added to Compile without a doc key, or a
//     doc key never emitted, fails);
//   - docs/ has no orphan .md and every image a doc references exists.
//
// So an intent rule PR without its doc/card fails here, not in review — the WS3-093 harness gap closed.
func TestRuleDocsOneToOne(t *testing.T) {
	// docKeys each resolve to a doc that opens with its heading.
	known := map[string]bool{}
	for _, k := range docKeys {
		known[k] = true
		if d := intentDoc(k); !strings.HasPrefix(d, "## "+k+"\n") {
			t.Errorf("docs/%s.md must open with '## %s'", k, k)
		}
	}

	// Every emitted rule's Detail comes from its doc key, and every doc key is emitted.
	emitted := map[string]bool{}
	for _, r := range Compile(kitchenSink()) {
		key := docKey(r.Name)
		if !known[key] {
			t.Errorf("rule %q maps to doc key %q, which is not in docKeys", r.Name, key)
			continue
		}
		emitted[key] = true
		if r.Detail == "" {
			t.Errorf("rule %q has no Detail (want intentDoc(%q))", r.Name, key)
			continue
		}
		if r.Detail != intentDoc(key) {
			t.Errorf("rule %q: Detail does not come from docs/%s.md (single-source violated)", r.Name, key)
		}
	}
	for _, k := range docKeys {
		if !emitted[k] {
			t.Errorf("doc key %q is never emitted by the kitchen-sink declaration (extend kitchenSink or drop the key)", k)
		}
	}

	// No orphan .md, and every referenced image exists.
	entries, err := ruleDocs.ReadDir("docs")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue // the images/ subdir is not a rule doc
		}
		if key := strings.TrimSuffix(e.Name(), ".md"); !known[key] {
			t.Errorf("docs/%s names no intent rule kind (orphan doc)", e.Name())
		}
	}
	images := map[string]bool{}
	imgEntries, err := ruleDocs.ReadDir("docs/images")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range imgEntries {
		if name := e.Name(); strings.HasSuffix(name, ".svg") || strings.HasSuffix(name, ".png") {
			images["images/"+name] = true
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

// TestRuleDocImageHandler: the read-only route serves an embedded card (200) as SVG but nothing else —
// the markdown, a missing image, a top-level (non-images/) path, or a non-image path all 404, so the
// handler never leaks anything but the diagrams (mirrors builtin's handler test).
func TestRuleDocImageHandler(t *testing.T) {
	h := RuleDocImageHandler()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec
	}
	ok := get("/images/protection-ovp.svg")
	if ok.Code != http.StatusOK {
		t.Fatalf("images/protection-ovp.svg status = %d, want 200", ok.Code)
	}
	if ct := ok.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("images/protection-ovp.svg content-type = %q, want image/svg+xml", ct)
	}
	for _, p := range []string{"/subsystem.md", "/images/nope.svg", "/", "/images/protection-ovp.txt", "/protection-ovp.svg"} {
		if code := get(p).Code; code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", p, code)
		}
	}
}

// TestDocRules holds intent.DocRules to the doc-key set the docsite catalog generator projects: one
// entry per docKey, each with a non-empty caption and its Detail from intentDoc. A new docKey without
// a DocRules entry (or a caption) fails here, so the docsite catalog cannot silently drop an intent
// rule kind.
func TestDocRules(t *testing.T) {
	got := DocRules()
	if len(got) != len(docKeys) {
		t.Fatalf("DocRules returned %d rules, want %d (one per docKey)", len(got), len(docKeys))
	}
	byName := map[string]bool{}
	for _, r := range got {
		byName[r.Name] = true
		if r.Summary == "" {
			t.Errorf("DocRules[%q] has an empty caption", r.Name)
		}
		if r.Detail != intentDoc(r.Name) {
			t.Errorf("DocRules[%q] Detail does not come from intentDoc(%q)", r.Name, r.Name)
		}
	}
	for _, k := range docKeys {
		if !byName[k] {
			t.Errorf("DocRules has no entry for docKey %q", k)
		}
	}
}
