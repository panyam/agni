package constraints

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// claudeMDCap is C36's budget in characters, the count `wc -m` prints.
const claudeMDCap = 10000

// TestC36EveryClaudeMDFitsItsBudget holds every CLAUDE.md in the tree to claudeMDCap. Every agent
// session loads the root file, and one in a directory loads that directory's file too, so a fact
// past the budget goes in a subdirectory's file, the docsite or CONTRIBUTING.md.
func TestC36EveryClaudeMDFitsItsBudget(t *testing.T) {
	root := repoRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			// tools/samples holds fetched boards from other repos, and it is gitignored.
			if path == filepath.Join(root, "tools", "samples") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "CLAUDE.md" {
			return nil
		}
		found = append(found, path)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if n := utf8.RuneCount(b); n > claudeMDCap {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s is %d characters, over C36's cap of %d. Move a fact out to a subdirectory's "+
				"CLAUDE.md, the docsite or CONTRIBUTING.md, and cut any history down to the rule "+
				"and one ticket link.", rel, n, claudeMDCap)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if !slices.Contains(found, filepath.Join(root, "CLAUDE.md")) {
		t.Fatalf("found no root CLAUDE.md among %d files, so this check proves nothing", len(found))
	}
}
