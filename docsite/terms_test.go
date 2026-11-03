package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A glossary is only useful while it is complete and every tag resolves, and discipline alone keeps
// neither true. A tag naming a missing term renders as a visible marker rather than as prose, but
// only on the page nobody previewed. A term that nothing references is dead weight, and nobody
// notices it. An index can also go short in silence. learn_levels_test.go exists to prevent that
// same failure one section over.
//
// These tests read the same files as the site generator, so a missing edit fails the gate rather
// than shipping.

const termsIndex = "content/reference/terms/index.md"

// tagRe matches {{ explainable "id" }} and {{ explainableCap "id" }}, with or without a label.
var tagRe = regexp.MustCompile(`\{\{-?\s*explainable(?:Cap)?\s+"([^"]+)"`)

// contentTags returns every term id referenced from content, keyed by id to the files using it.
// The glossary index itself is excluded, since it documents the syntax and its example is not a use.
func contentTags(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		if filepath.ToSlash(path) == termsIndex {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range tagRe.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = append(out[m[1]], path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", contentDir, err)
	}
	return out
}

// TestEveryTagResolvesToATerm fails when a page tags a term that has no page. Without this the tag
// renders its "[unknown term: x]" marker on a page nobody happens to open.
func TestEveryTagResolvesToATerm(t *testing.T) {
	terms, err := LoadTerms()
	if err != nil {
		t.Fatalf("loading terms: %v", err)
	}
	for id, files := range contentTags(t) {
		if _, ok := terms[id]; !ok {
			t.Errorf("%s tags term %q, but %s/%s.md does not exist", files[0], id, termsDir, id)
		}
	}
}

// TestEveryTermIsUsed fails on a term that nothing references. If a reader can reach a glossary
// entry only by browsing the glossary, the page has no caller. The useful signal is whether the
// prose it was written for ever adopted it.
func TestEveryTermIsUsed(t *testing.T) {
	used := contentTags(t)
	for _, id := range TermIDs() {
		if len(used[id]) == 0 {
			t.Errorf("%s/%s.md is referenced by no page; either tag it somewhere or delete it", termsDir, id)
		}
	}
}

// TestATermIsTaggedAtMostOncePerPage fails when one page tags the same term more than once.
//
// The tag is a reading aid, and a page that tags every occurrence wastes it. `pull-up` appears
// twenty times in prose outside `learn/`, and twenty dotted underlines in one tutorial reads as
// damage rather than as help. A reader needs one tag per term per page. That one tag opens the
// popover, and after it the glossary is a click away.
//
// This checks the COUNT, which is reliable. The convention also says the tagged mention should be
// the FIRST one. docsite/README.md documents that half, and it is deliberately not tested. Deciding
// whether an earlier plain-text occurrence "counts" means matching an inflected label through code
// spans, headings and link text. A check that fires wrongly on an author costs more than the
// convention is worth.
func TestATermIsTaggedAtMostOncePerPage(t *testing.T) {
	perFile := map[string]map[string]int{}
	for id, files := range contentTags(t) {
		for _, f := range files {
			if perFile[f] == nil {
				perFile[f] = map[string]int{}
			}
			perFile[f][id]++
		}
	}
	var paths []string
	for f := range perFile {
		paths = append(paths, f)
	}
	sort.Strings(paths)
	for _, f := range paths {
		var ids []string
		for id := range perFile[f] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if n := perFile[f][id]; n > 1 {
				t.Errorf("%s tags %q %d times; tag the FIRST mention only and leave the rest as plain text", f, id, n)
			}
		}
	}
}

// TestEveryTermDeclaresItsFields fails when a term lacks a field that the tag or the fallback
// needs. A term with no label renders an empty anchor. A term with no summary loses the
// no-JavaScript hover entirely. Both look like working links.
func TestEveryTermDeclaresItsFields(t *testing.T) {
	terms, err := LoadTerms()
	if err != nil {
		t.Fatalf("loading terms: %v", err)
	}
	for id, term := range terms {
		if strings.TrimSpace(term.Title) == "" {
			t.Errorf("term %q has no title, so its page renders headless", id)
		}
		if strings.TrimSpace(term.Label) == "" {
			t.Errorf("term %q has no label, so {{ explainable %q }} renders an empty link", id, id)
		}
		if strings.TrimSpace(term.Summary) == "" {
			t.Errorf("term %q has no summary, so it has no hover text without JavaScript", id)
		}
	}
}

// TestGlossaryIndexListsEveryTerm fails when a term exists but the glossary does not link it. The
// index is hand-maintained on purpose, as learn/levels.md is. Generating it would mean a second
// build step for a file that changes once per term, and this test costs less.
func TestGlossaryIndexListsEveryTerm(t *testing.T) {
	index := read(t, termsIndex)
	var missing []string
	for _, id := range TermIDs() {
		if !strings.Contains(index, "./"+id+"/") {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	for _, id := range missing {
		t.Errorf("%s does not link term %q, so it is absent from the glossary", termsIndex, id)
	}
}

// TestTermPagesAreExemptFromNavWiring pins the reason adding a term is ONE file rather than four.
// nav_test.go's reachability check reads files directly under a section and skips subdirectories.
// So the generated catalogs and this glossary need no nav entry each. If that ever changes, every
// term page becomes unreachable-by-nav, and this test says so first.
func TestTermPagesAreExemptFromNavWiring(t *testing.T) {
	for _, slug := range sectionPages(t, "reference") {
		if slug == "terms" {
			t.Fatal("sectionPages now returns the terms subdirectory, so every term page needs a nav entry")
		}
	}
	if len(TermIDs()) == 0 {
		t.Fatalf("no terms found under %s; this suite would pass vacuously", termsDir)
	}
}
