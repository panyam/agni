package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The learn course is indexed two ways: by topic (the numbered chapters) and by LEVEL
// (content/learn/levels.md, which lists every section operating at each level). The second is
// hand-maintained, so it drifts the moment a chapter adds a section and nobody updates it, and the
// drift is silent: the levels page still renders, just short.
//
// That is the same failure the generated command captures exist to prevent one layer down, so it
// gets the same treatment. These tests are cheap because both sides are greppable: a section
// declares its level in its heading, and the index links to it by anchor.

var (
	chapterFile = regexp.MustCompile(`^\d\d-.*\.md$`)
	levelHeading = regexp.MustCompile(`(?m)^## (.+?) \((EE\d)\)$`)
)

// slug mirrors the renderer's heading-to-anchor rule closely enough for a link check: lowercase,
// non-alphanumerics collapsed to hyphens. Verified against the built site when levels.md landed.
func slug(s string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func learnDir() string { return filepath.Join(contentDir, "learn") }

// chapterSections returns every level-tagged section as (file, anchor, level).
func chapterSections(t *testing.T) [][3]string {
	t.Helper()
	entries, err := os.ReadDir(learnDir())
	if err != nil {
		t.Fatalf("read learn dir: %v", err)
	}
	var out [][3]string
	for _, e := range entries {
		if !chapterFile.MatchString(e.Name()) {
			continue
		}
		body := read(t, filepath.Join(learnDir(), e.Name()))
		for _, m := range levelHeading.FindAllStringSubmatch(body, -1) {
			page := strings.TrimSuffix(e.Name(), ".md")
			out = append(out, [3]string{page, slug(m[1] + " " + m[2]), m[2]})
		}
	}
	if len(out) == 0 {
		t.Fatal("no level-tagged sections found; the heading convention must have changed")
	}
	return out
}

// TestEveryLevelledSectionIsIndexed fails when a chapter gains a level-tagged section that
// levels.md does not list. Without it the level index quietly stops being a map of the course.
func TestEveryLevelledSectionIsIndexed(t *testing.T) {
	index := read(t, filepath.Join(learnDir(), "levels.md"))
	for _, s := range chapterSections(t) {
		link := "../" + s[0] + "/#" + s[1]
		if !strings.Contains(index, link) {
			t.Errorf("content/learn/levels.md does not list %s (%s), so the %s index is short a section\n  expected a link to %s",
				s[1], s[0], s[2], link)
		}
	}
}

// TestLevelIndexPointsAtRealSections is the other direction: an entry for a section that was renamed
// or removed becomes a dead anchor, which renders as a link that silently goes nowhere.
func TestLevelIndexPointsAtRealSections(t *testing.T) {
	have := map[string]bool{}
	for _, s := range chapterSections(t) {
		have["../"+s[0]+"/#"+s[1]] = true
	}
	index := read(t, filepath.Join(learnDir(), "levels.md"))
	// Anchored links only. A bare `../03-why-.../` is prose pointing at a whole chapter, not a claim
	// about a section, and matching those made the first version of this test fail on valid text.
	for _, m := range regexp.MustCompile(`\]\((\.\./\d\d-[^)#]+/#[^)]+)\)`).FindAllStringSubmatch(index, -1) {
		if !have[m[1]] {
			t.Errorf("content/learn/levels.md links %s, which is not a level-tagged section in any chapter", m[1])
		}
	}
}

// TestEveryChapterDeclaresItsLevels: a chapter without the pointer line leaves a reader who meets
// "(EE4)" in a heading with nowhere to find out what EE4 means, which is the gap levels.md closed.
func TestEveryChapterDeclaresItsLevels(t *testing.T) {
	for _, s := range chapterSections(t) {
		body := read(t, filepath.Join(learnDir(), s[0]+".md"))
		if !strings.Contains(body, "**Levels on this page:**") {
			t.Errorf("content/learn/%s.md tags sections with levels but carries no pointer to their definitions", s[0])
			break
		}
	}
}
