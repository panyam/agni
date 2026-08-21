package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// TestEveryContentPageTemplateParses is the guard for a failure mode that is invisible in review and
// silent at build time: content markdown is run through text/template BEFORE it is rendered, so a
// stray "{{" anywhere in a page makes the whole page fail to load and publish as BLANK. Not
// truncated at the offending line, blank. The build still exits 0 and the page still exists at its
// URL, so nothing downstream reports it.
//
// This is not hypothetical. build/check-rule.md shipped that way: two Go code samples contained
// `[]check.ContextSubject{{`, which the templater read as an action calling a function named "Kind",
// and the repo's entire rule-authoring guide rendered as an empty page while its twelve siblings
// were fine.
//
// A code fence does not protect you, because templating happens before any markdown parsing. The fix
// for a Go composite literal is to write the element brace on its own line.
//
// Parsing with the site's own CommonFuncMap is what makes this precise rather than a "{{" grep: a
// legitimate `{{ agniRun "..." }}` or `{{.Site.PathPrefix}}` parses clean and only an unknown
// function or malformed action fails, which is exactly the class that blanks a page.
func TestEveryContentPageTemplateParses(t *testing.T) {
	funcs := template.FuncMap{}
	for name, fn := range Site.CommonFuncMap {
		funcs[name] = fn
	}

	var checked int
	err := filepath.Walk("content", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		if _, err := template.New(filepath.Base(path)).Funcs(funcs).Parse(string(body)); err != nil {
			t.Errorf("%s would publish as a BLANK page: %v\n"+
				"a stray {{ is parsed as a template action; in a Go sample put the element brace on its own line", path, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk content: %v", err)
	}
	if checked == 0 {
		t.Fatal("no content pages found, so this guard asserted nothing")
	}
	t.Logf("template-parsed %d content pages", checked)
}
