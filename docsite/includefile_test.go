package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A diagram lives in `figures/` and a page pulls it in with `{{ includeFile "figures/x.svg" }}`, so
// the SVG is inlined at BUILD time. That keeps `currentColor` and `--accent-color` working, which an
// `<img src>` cannot do because the image renders in its own document and inherits nothing from the
// host page, while keeping several hundred lines of path data out of the prose.
//
// The failure mode this guards is silence. `IncludeFile` returns an empty string when the path does
// not resolve, so a typo, a rename, or a moved file removes the figure from the page and the build
// still succeeds. Verified by pointing a live include at a missing file: the page rendered with zero
// SVG in its body and no error anywhere.

var includeFileRe = regexp.MustCompile(`\{\{-?\s*includeFile\s+"([^"]+)"`)

const figuresDir = "figures"

// includedFiles returns every path included from content, keyed to the pages doing it.
func includedFiles(t *testing.T) map[string][]string {
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
		for _, m := range includeFileRe.FindAllStringSubmatch(string(b), -1) {
			out[m[1]] = append(out[m[1]], path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", contentDir, err)
	}
	return out
}

func TestEveryIncludedFileResolves(t *testing.T) {
	for rel, pages := range includedFiles(t) {
		if _, err := os.Stat(filepath.Join(projectRoot, rel)); err != nil {
			sort.Strings(pages)
			t.Errorf("%s includes %q, which does not exist, so the page renders without it and nothing says so\n  included by: %s",
				filepath.Base(pages[0]), rel, strings.Join(pages, ", "))
		}
	}
}

// A figure nothing includes is a diagram someone drew and the prose never adopted, the same argument
// terms_test.go makes for a glossary entry with no caller.
func TestEveryFigureIsIncluded(t *testing.T) {
	included := includedFiles(t)
	entries, err := os.ReadDir(filepath.Join(projectRoot, figuresDir))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("reading %s: %v", figuresDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		rel := figuresDir + "/" + e.Name()
		if len(included[rel]) == 0 {
			t.Errorf("%s is not included by any page", rel)
		}
	}
}

// The whole point of the figures directory is that these stay theme-aware. A literal colour reads
// correctly in whichever theme it was authored in and badly in the other, and the docsite has both.
func TestFiguresCarryNoColourLiterals(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(projectRoot, figuresDir))
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("reading %s: %v", figuresDir, err)
	}
	literal := regexp.MustCompile(`(?:fill|stroke|stop-color)="\s*(#[0-9a-fA-F]{3,8}|rgb\(|hsl\()`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(projectRoot, figuresDir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range literal.FindAllString(string(b), -1) {
			t.Errorf("%s/%s carries the colour literal %q; use currentColor or var(--accent-color)",
				figuresDir, e.Name(), m)
		}
	}
}
