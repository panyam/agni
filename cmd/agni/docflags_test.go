package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// docFlagExceptions are flag-shaped strings a page may name without the CLI defining them, each with
// the reason. Keep it short: every entry is a place the docs and the tool are allowed to disagree.
// It holds ANOTHER TOOL'S flags, which is the one category syntax cannot separate from ours: a
// backticked `--cask` is a real flag and it is brew's. Every entry names the tool, so a reader can
// tell a deliberate exception from a stale one, and the list stays short because agni's own flags
// come from the command tree rather than from here.
var docFlagExceptions = map[string]string{
	"--cask":             "brew",
	"--pages":            "kicad-cli",
	"--mode-single":      "kicad-cli",
	"--plotfile":         "xschem",
	"--layers":           "kicad-cli",
	"--quit":             "kicad-cli",
	"--svg":              "kicad-cli",
	"--exclude-standard": "git ls-files",
	"--dump":             "hack/fixture_copies_check.sh",
	"--noEmit":           "tsc",
	"--no":               "a line-wrapped flag of another tool, not a flag in itself",
}

// TestDocsNameNoFlagTheCLILacks is issue 636's guard, and it covers the half TestDocumentedCommandsParse
// cannot reach.
//
// That test extracts hand-written `agni ...` commands from FENCES and runs them through cobra, so a
// removed flag inside a fence fails there. It cannot see a flag named in PROSE, and prose is where
// most of a stale flag lives: nine of the thirteen `--url-base` references that survived its removal
// were sentences, not commands.
//
// The flag set comes from walking the command tree, so nothing here is hand-maintained except the
// exception list. A flag defined on ANY command counts, because a page discussing `--verdicts` while
// documenting `review` is describing a real flag and the reader can find it.
func TestDocsNameNoFlagTheCLILacks(t *testing.T) {
	known := knownFlags(rootCmd())
	// Flags named in INLINE CODE, outside fenced blocks. Both halves of that narrowing matter.
	//
	// Fences are excluded because they are the other test's job when they hold an `agni` command, and
	// because they legitimately hold OTHER tools: `brew --cask`, `kicad-cli --pages`,
	// `git --exclude-standard`. A rule that read them would report a dozen flags this CLI has no
	// opinion about, and an exception list that long stops being read.
	//
	// Inline code because that is how prose names a flag it means. A bare `--o` in an ASCII diagram or
	// a `--no` at a line wrap is not a claim about the CLI, and requiring the backticks separates the
	// two without a list.
	// TWO passes, and the second one matters: a span is found first, then EVERY flag inside it. One
	// combined pattern reports only the first flag per span, so `agni check --mount … --url-base …`
	// matched on --mount, found it known, and cleared the span with the stale flag still in it. That
	// is the shape of a guard that reports clean because it stopped looking.
	spanRe := regexp.MustCompile("`[^`]+`")
	flagRe := regexp.MustCompile(`--[a-z][a-z0-9-]*`)

	type hit struct{ where, flag string }
	var hits []hit
	err := filepath.WalkDir(docContentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(docContentDir, path)
		for i, line := range stripFences(string(b)) {
			for _, span := range spanRe.FindAllString(line, -1) {
				for _, f := range flagRe.FindAllString(span, -1) {
					if known[f] || docFlagExceptions[f] != "" {
						continue
					}
					hits = append(hits, hit{where: rel + ":" + itoa(i+1), flag: f})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		return
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].where < hits[j].where })
	var lines []string
	for _, h := range hits {
		lines = append(lines, "  "+h.where+"  "+h.flag)
	}
	t.Errorf("these pages name a flag the CLI does not define:\n%s\n\n"+
		"A renamed or removed flag leaves the gate green everywhere except a fence, which is how "+
		"--url-base survived across five pages (agni issue 636). Either the page is stale, or the "+
		"string is not a flag and belongs in docFlagExceptions with its reason.", strings.Join(lines, "\n"))
}

// knownFlags collects every flag name the command tree defines, local and persistent, including
// hidden ones. A HIDDEN flag counts as known on purpose: hiding it is a decision about `--help`, and
// a page may still legitimately explain a flag that works.
func knownFlags(root *cobra.Command) map[string]bool {
	out := map[string]bool{}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		// cobra adds --help and --version lazily, on Execute, so a tree that has not run yet does not
		// declare them. Without this the guard reports `agni --version` as a flag the CLI lacks, which
		// is the guard being wrong about its own tool.
		c.InitDefaultHelpFlag()
		c.InitDefaultVersionFlag()
		c.Flags().VisitAll(func(f *pflag.Flag) { out["--"+f.Name] = true })
		c.PersistentFlags().VisitAll(func(f *pflag.Flag) { out["--"+f.Name] = true })
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// stripFences returns the document's lines with fenced blocks blanked, keeping the line numbering so
// a failure names the line a reader can open.
func stripFences(doc string) []string {
	lines := strings.Split(doc, "\n")
	inFence := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			lines[i] = ""
			continue
		}
		if inFence {
			lines[i] = ""
		}
	}
	return lines
}
