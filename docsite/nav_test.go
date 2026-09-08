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

// headerEntry is one HeaderNavLinks.json record. A child carries the same fields, and "group": true
// marks a child that renders as a subheading inside the dropdown rather than an ordinary item.
type headerEntry struct {
	Name     string        `json:"name"`
	URL      string        `json:"url"`
	Group    bool          `json:"group"`
	Children []headerEntry `json:"children"`
}

func headerEntries(t *testing.T) []headerEntry {
	t.Helper()
	var links []headerEntry
	if err := json.Unmarshal([]byte(read(t, headerLinks)), &links); err != nil {
		t.Fatalf("parse %s: %v", headerLinks, err)
	}
	return links
}

// TestHeaderDropdownListsEveryPage fails when a section owns a top-level header dropdown but one of
// its pages is missing from it. That page still renders and the sidebar still links it, so nothing
// looks broken. It is absent from the most visible nav the site has, and you do not notice because
// you reach the page by typing the URL you just wrote.
//
// TestHeaderNavLinksResolve covers the json-to-content direction. This is content-to-json, which is
// the half nothing checked (agni issue 574).
//
// The exemption is READ FROM THE DATA rather than listed here. A section is required to be complete
// only when it has a top-level entry of its own carrying children, which is the header declaring an
// intent to itemise that section. `learn/` and `build/` appear instead as group headings inside
// another section's dropdown, and `learn/` lists two of its fifteen pages on purpose, since twelve
// chapters would swamp the menu and the course has its own index and level map. Give `learn/` a
// top-level dropdown later and every chapter becomes required with no edit here. What this costs is
// that a section itemised only as a group heading goes unchecked, which is why
// TestSectionIndexLinksEveryPage carries no exemption at all.
func TestHeaderDropdownListsEveryPage(t *testing.T) {
	for _, entry := range headerEntries(t) {
		section := strings.Trim(entry.URL, "/")
		if len(entry.Children) == 0 || section == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(contentDir, section)); err != nil || !fi.IsDir() {
			continue // a dropdown that is not a content section
		}
		listed := map[string]bool{}
		for _, c := range entry.Children {
			listed[strings.Trim(c.URL, "/")] = true
		}
		for _, slug := range sectionPages(t, section) {
			if !listed[section+"/"+slug] {
				t.Errorf("content/%s/%s.md is missing from the %q dropdown in %s, so it is absent from the header nav",
					section, slug, entry.Name, headerLinks)
			}
		}
	}
}

// sectionIndexLinks returns the page slugs content/<section>/index.md links with a relative link.
// The indexes write `[EE1](levels/#parts-ee1)` rather than a section-qualified path, so the match is
// anchored on the markdown link target: an optional "./", the slug, a slash, and an optional anchor.
// Matching the bare slug anywhere in the body would pass on any mention of the page's name in prose.
func sectionIndexLinks(t *testing.T, section string) map[string]bool {
	t.Helper()
	body := read(t, filepath.Join(contentDir, section, "index.md"))
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\((?:\./)?([a-z0-9-]+)/(?:#[^)]*)?\)`).FindAllStringSubmatch(body, -1) {
		out[m[1]] = true
	}
	return out
}

// TestSectionIndexLinksEveryPage fails when a section's index.md does not link one of the section's
// pages. The index is where a reader who arrived at the section rather than at the page starts, so a
// page missing from it is invisible to anyone browsing rather than searching.
//
// This one has no exemption. Every section indexes every page it holds today, `learn/` included,
// which is what makes it the check covering the sections the header rule above lets through.
func TestSectionIndexLinksEveryPage(t *testing.T) {
	for _, section := range dispatchedSections(t) {
		if _, err := os.Stat(filepath.Join(contentDir, section, "index.md")); err != nil {
			t.Errorf("section %q has no index.md, so it has no landing page", section)
			continue
		}
		linked := sectionIndexLinks(t, section)
		for _, slug := range sectionPages(t, section) {
			if !linked[slug] {
				t.Errorf("content/%s/%s.md is not linked from content/%s/index.md, so nothing on the section's landing page reaches it",
					section, slug, section)
			}
		}
	}
}
