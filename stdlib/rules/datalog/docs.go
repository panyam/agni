package datalog

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
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

// RuleDocImageHandler serves this package's card diagrams, under docs/images/, read-only (agni issue
// 75). It follows builtin.RuleDocImageHandler's contract. It serves .svg and .png only and answers
// 404 for anything else, the markdown included. It is mounted under a prefix (e.g.
// "images/power-pin-mistyped.svg") and sets an explicit SVG content-type.
func RuleDocImageHandler() http.Handler {
	sub, err := fs.Sub(ruleDocs, "docs")
	if err != nil {
		panic(fmt.Sprintf("datalogrules: rule docs sub-FS: %v", err)) // embed path is a constant; unreachable
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		case strings.HasSuffix(r.URL.Path, ".png"):
		default:
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
