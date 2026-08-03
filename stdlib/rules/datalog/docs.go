package datalog

import (
	"embed"
	"fmt"
)

// ruleDocs embeds the per-rule documentation for the datalog-authored rules: one markdown file
// (plus any images it references) per rule under docs/, the single source of Rule.Detail — the same
// convention as check/docs, so prose and diagrams are authored as markdown beside the rule, not as
// Go strings inline. The whole docs/ directory is embedded (not a `*.md` glob), so a rule's doc can
// add a `.png` walkthrough image with no build change. The 1:1 between "dl" rules and doc files, and
// that every referenced image exists, is harness-enforced (docs_test.go).
//
//go:embed docs
var ruleDocs embed.FS

// ruleDoc returns the embedded markdown for a datalog rule (the bare name, without the "dl/"
// source prefix). A missing file is a programmer error caught at package init, when every rule
// loads its Detail.
func ruleDoc(name string) string {
	b, err := ruleDocs.ReadFile("docs/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("datalogrules: no doc for rule %q (want docs/%s.md): %v", name, name, err))
	}
	return string(b)
}
