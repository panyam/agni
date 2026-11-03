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
// the SVG is inlined at BUILD time. Inlining keeps `currentColor` and `--accent-color` working, and
// it keeps several hundred lines of path data out of the prose. An `<img src>` cannot do that,
// because the image renders in its own document and inherits nothing from the host page.
//
// This test guards against a silent failure. `IncludeFile` returns an empty string when the path
// does not resolve, so a typo, a rename, or a moved file removes the figure from the page and the
// build still succeeds. We verified this by pointing a live include at a missing file. The page
// rendered with zero SVG in its body and no error anywhere.

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

// If nothing includes a figure, someone drew a diagram and the prose never adopted it. terms_test.go
// makes the same argument for a glossary entry with no caller.
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

// The figures directory exists so these figures are theme-aware. A literal colour reads
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

// The inlined SVG is a raw-HTML block, and a BLANK LINE inside a figure file ends that block. The
// failure is invisible in the source and in the build.
//
// `IncludeFile` splices the file into the page BEFORE the markdown renderer runs, so the SVG is
// ordinary raw HTML by the time CommonMark sees it, and CommonMark ends an HTML block at the first
// blank line. Whether the next chunk is passed through again depends on what it starts with. A line
// holding nothing but an open tag (`<g fill="...">`) opens a new HTML block and survives, while a
// line with content after the tag (`<text x="8" y="44">your machine</text>`) does not, and is parsed
// as a paragraph instead. The renderer then wraps that chunk in `<p>`, which closes the `<svg>`
// early, and every element after it ends up OUTSIDE the figure. The page still builds, the figure
// still renders, and the elements in the orphaned tail are simply absent.
//
// Four figures merged before this test shipped in that state, each missing its closing
// caption on the live site. We found them by screenshotting a figure and counting the text runs,
// not by any test, so this test now exists. `docsite/README.md` gave the opposite advice until then.
func TestFiguresCarryNoBlankLines(t *testing.T) {
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
		b, err := os.ReadFile(filepath.Join(projectRoot, figuresDir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == "" {
				t.Errorf("%s/%s has a blank line at line %d; it ends the raw-HTML block, so everything after it can fall outside the <svg>",
					figuresDir, e.Name(), i+1)
				break
			}
		}
	}
}
