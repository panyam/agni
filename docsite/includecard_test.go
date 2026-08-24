package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A transcluded card does not pass through the template engine, so any site directive it carries
// reaches the browser verbatim. That broke four card images across two guide pages: the request went
// out for a literal `%7B%7B.Site.PathPrefix%7D%7D` path and 404'd. Nothing caught it because the
// prose around the image rendered perfectly and the page looked right.
//
// These tests read the same files the site generator reads, so a card gaining a new directive fails
// the gate rather than shipping a broken asset.

var includeCardRe = regexp.MustCompile(`\{\{-?\s*includeCard\s+"([^"]+)"`)

// transcludedCards returns every card path transcluded from content, keyed to the pages doing it.
func transcludedCards(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	err := filepath.Walk(contentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range includeCardRe.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = append(out[m[1]], path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", contentDir, err)
	}
	return out
}

// TestTranscludedCardsCarryNoUnrenderedDirective is the regression guard. It asserts on what
// includeCard actually RETURNS rather than on the card file, so it covers the substitution rather
// than restating the input.
func TestTranscludedCardsCarryNoUnrenderedDirective(t *testing.T) {
	cards := transcludedCards(t)
	if len(cards) == 0 {
		t.Fatal("no includeCard calls found in content; this test would pass vacuously")
	}
	var paths []string
	for c := range cards {
		paths = append(paths, c)
	}
	sort.Strings(paths)
	for _, card := range paths {
		body := IncludeCard(card)
		if body == "" {
			t.Errorf("includeCard(%q) returned nothing, so %s transcludes an empty block", card, cards[card][0])
			continue
		}
		if i := strings.Index(body, "{{"); i >= 0 {
			end := i + 60
			if end > len(body) {
				end = len(body)
			}
			t.Errorf("includeCard(%q) still carries an unrendered directive, which reaches the browser verbatim: %q\n  transcluded by %s",
				card, body[i:end], cards[card][0])
		}
	}
}

// TestTranscludedCardAssetsExist follows the substituted path to disk. The directive could be
// replaced with a WRONG prefix and the first test would still pass, since it only checks that the
// braces are gone.
func TestTranscludedCardAssetsExist(t *testing.T) {
	assetRe := regexp.MustCompile(`\]\(` + regexp.QuoteMeta(PathPrefix) + `(/static/[^)\s]+)`)
	checked := 0
	for card := range transcludedCards(t) {
		for _, m := range assetRe.FindAllStringSubmatch(IncludeCard(card), -1) {
			checked++
			if _, err := os.Stat(filepath.Join(".", m[1])); err != nil {
				t.Errorf("card %s references %s%s, which does not exist under docsite/", card, PathPrefix, m[1])
			}
		}
	}
	if checked == 0 {
		t.Fatal("no transcluded card referenced a static asset; the check proved nothing")
	}
	t.Logf("checked %d transcluded card assets", checked)
}
