package datalog

import (
	"embed"
	"fmt"
)

// ruleDocs embeds docs/<rule>.md for each datalog rule plus the images those files reference, the
// same arrangement as stdlib/rules/builtin/docs.go. It embeds the whole docs/ directory rather than
// per-extension globs, so a doc can add an image of any type with no build change. docs_test.go
// enforces the 1:1 between "dl" rules and doc files, and that every referenced image exists.
//
//go:embed docs
var ruleDocs embed.FS

// ruleDoc returns the embedded markdown for a datalog rule, keyed by the bare name without the
// "dl/" source prefix. It panics on a missing file, which surfaces at package init when every rule
// loads its Detail.
func ruleDoc(name string) string {
	b, err := ruleDocs.ReadFile("docs/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("datalogrules: no doc for rule %q (want docs/%s.md): %v", name, name, err))
	}
	return string(b)
}
