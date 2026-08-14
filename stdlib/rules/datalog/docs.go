package datalog

import (
	"embed"
	"fmt"
)

// ruleDocs embeds the per-rule documentation for the datalog-authored rules: one markdown file per
// rule under docs/, plus any images it references. The convention is in
// docsite/content/build/check-rule.md, and stdlib/rules/builtin/docs is the other site. The whole
// docs/ directory is embedded, not a `*.md` glob, so a rule's doc can add a `.png` walkthrough
// image with no build change. The 1:1 between "dl" rules and doc files, and that every referenced
// image exists, is harness-enforced (docs_test.go).
//
//go:embed docs
var ruleDocs embed.FS

// ruleDoc returns the embedded markdown for a datalog rule (the bare name, without the "dl/"
// source prefix), and panics if the file is missing. That is a programmer error caught at package
// init, when every rule loads its Detail.
func ruleDoc(name string) string {
	b, err := ruleDocs.ReadFile("docs/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("datalogrules: no doc for rule %q (want docs/%s.md): %v", name, name, err))
	}
	return string(b)
}
