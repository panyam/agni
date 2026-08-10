package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Adding a docs section takes five coordinated edits: the pages, a nav template, BOTH the include
// and the dispatch branch in Sidebar.html, HeaderNavLinks.json, and a link from a sibling index.
// Nothing enforced that, and it had already drifted twice before these tests existed: an orphan nav
// template pointing at a content directory that was never created, and an architecture page absent
// from every nav, reachable only by typing its URL.
//
// These tests read the same files the site generator reads, so they fail on the edit rather than on
// someone later noticing a section renders the wrong sidebar.

const (
	contentDir  = "content"
	navDir      = "templates/nav"
	sidebarPath = "templates/Sidebar.html"
	headerLinks = "content/HeaderNavLinks.json"
)

// navTemplates returns the section name each templates/nav/<Name>Nav.html defines, keyed by file.
func navTemplates(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(navDir)
	if err != nil {
		t.Fatalf("read %s: %v", navDir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		out[e.Name()] = strings.TrimSuffix(e.Name(), ".html")
	}
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestEveryNavTemplateIsWired fails when a nav template exists but Sidebar.html does not both
// include it and dispatch to it. A template that is included but never dispatched is dead weight;
// one that is dispatched but not included breaks the page render.
func TestEveryNavTemplateIsWired(t *testing.T) {
	sidebar := read(t, sidebarPath)
	for file, name := range navTemplates(t) {
		if !strings.Contains(sidebar, `include "nav/`+file+`"`) {
			t.Errorf("%s is never included by %s", file, sidebarPath)
		}
		if !strings.Contains(sidebar, `template "`+name+`"`) {
			t.Errorf("%s defines %q but %s never dispatches to it, so its section falls through to the generic nav",
				file, name, sidebarPath)
		}
	}
}

// TestEveryDispatchedSectionExists fails when Sidebar.html routes a URL prefix to a nav template
// but no such content directory exists, which is how an orphan template survives unnoticed.
func TestEveryDispatchedSectionExists(t *testing.T) {
	sidebar := read(t, sidebarPath)
	re := regexp.MustCompile(`Contains \$currentPath "/([a-z-]+)/"`)
	for _, m := range re.FindAllStringSubmatch(sidebar, -1) {
		section := m[1]
		if _, err := os.Stat(filepath.Join(contentDir, section)); err != nil {
			t.Errorf("%s dispatches on /%s/ but %s/%s does not exist", sidebarPath, section, contentDir, section)
		}
	}
}

// sectionPages returns the non-index page slugs directly under content/<section>.
func sectionPages(t *testing.T, section string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(contentDir, section))
	if err != nil {
		t.Fatalf("read section %s: %v", section, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		if slug == "index" {
			continue
		}
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

// dispatchedSections returns the sections Sidebar.html routes to a nav template.
func dispatchedSections(t *testing.T) []string {
	t.Helper()
	re := regexp.MustCompile(`Contains \$currentPath "/([a-z-]+)/"`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(read(t, sidebarPath), -1) {
		out = append(out, m[1])
	}
	return out
}

// TestEveryPageIsReachable fails when a page exists in a navigated section but no nav template
// links it. Such a page still renders and is still indexed; it is simply unreachable by clicking,
// which is indistinguishable from it not existing.
func TestEveryPageIsReachable(t *testing.T) {
	navBlob := ""
	for file := range navTemplates(t) {
		navBlob += read(t, filepath.Join(navDir, file))
	}
	for _, section := range dispatchedSections(t) {
		if _, err := os.Stat(filepath.Join(contentDir, section)); err != nil {
			continue // reported by TestEveryDispatchedSectionExists
		}
		for _, slug := range sectionPages(t, section) {
			if !strings.Contains(navBlob, section+"/"+slug+"/") {
				t.Errorf("content/%s/%s.md is in no nav template, so nothing links to it", section, slug)
			}
		}
	}
}

// TestHeaderNavLinksResolve fails when HeaderNavLinks.json points at content that is not there. It
// is the top-of-page nav, so a stale entry is a 404 on the most visible surface the site has.
func TestHeaderNavLinksResolve(t *testing.T) {
	var links []struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Children []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"children"`
	}
	if err := json.Unmarshal([]byte(read(t, headerLinks)), &links); err != nil {
		t.Fatalf("parse %s: %v", headerLinks, err)
	}
	check := func(name, url string) {
		if url == "" {
			return // Home
		}
		clean := strings.Trim(url, "/")
		if _, err := os.Stat(filepath.Join(contentDir, clean+".md")); err == nil {
			return
		}
		if _, err := os.Stat(filepath.Join(contentDir, clean, "index.md")); err == nil {
			return
		}
		if _, err := os.Stat(filepath.Join(contentDir, clean)); err == nil {
			return // a generated directory such as reference/rules
		}
		t.Errorf("HeaderNavLinks entry %q points at %q, which resolves to no content", name, url)
	}
	for _, l := range links {
		check(l.Name, l.URL)
		for _, c := range l.Children {
			check(c.Name, c.URL)
		}
	}
}

// TestSectionIndexesAreListedInHeader fails when a navigated section is missing from the header
// nav entirely, which is how a whole section becomes reachable only from a sibling page's prose.
func TestSectionIndexesAreListedInHeader(t *testing.T) {
	header := read(t, headerLinks)
	for _, section := range dispatchedSections(t) {
		if !strings.Contains(header, `"`+section+`/"`) {
			t.Errorf("section %q has a nav template and a dispatch branch but no HeaderNavLinks entry", section)
		}
	}
}

// TestBuiltSiteIsSelfContained guards the layering between `build` and `gh-pages`. The generator
// emits pages only, so `build` must copy static/ in. It did not for a long time, and nothing
// noticed because the sole consumer was `gh-pages`, which copied static/ itself on the way out. A
// dist without it has no CSS, no JS bundle, no images and no rendered designs, and an interactive
// component whose bundle never loads renders as nothing at all rather than as a broken box.
//
// This asserts the Makefile wiring rather than running a build, so it stays fast and does not need
// node. The build is exercised for real by `make build` in CI.
func TestBuiltSiteIsSelfContained(t *testing.T) {
	mk := read(t, "Makefile")
	_, after, ok := strings.Cut(mk, "\nbuild:")
	if !ok {
		t.Fatal("no build target in docsite/Makefile")
	}
	recipe, _, _ := strings.Cut(after, "\n\n") // a make target ends at the first blank line
	if !strings.Contains(recipe, "cp -r static dist/static") {
		t.Error("the build target does not copy static/ into dist, so a built site has no CSS, no JS bundle and no designs")
	}
}
