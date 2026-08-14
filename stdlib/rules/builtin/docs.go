package builtin

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// ruleDocs embeds the per-rule documentation for the built-in rules: one markdown file per rule
// under docs/, plus the images those files reference (WS3-025). The convention is in
// docsite/content/build/check-rule.md. Embedding rather than reading files at runtime keeps the
// single-binary and WASM builds working and the core free of file I/O (C1). The embed patterns are
// per-extension globs, so a doc referencing a new image type needs a pattern added below. The 1:1
// between registered rules and doc files is harness-enforced (docs_test.go).
//
//go:embed docs/*.md docs/images/*.svg docs/images/*.png
var ruleDocs embed.FS

// ruleDoc returns the embedded markdown for a built-in rule, and panics if the file is missing.
// That is a programmer error caught at package init, when every Detail loads, and the harness
// reports it with the rule name before any panic would.
func ruleDoc(name string) string {
	b, err := ruleDocs.ReadFile("docs/" + name + ".md")
	if err != nil {
		panic(fmt.Sprintf("check: no doc for rule %q (want docs/%s.md): %v", name, name, err))
	}
	return string(b)
}

// RuleDocImageHandler serves the rule docs' embedded explainer images (the diagrams a
// docs/<rule>.md references, WS3-025) as a read-only static route, so the web rules/expectations
// panels can resolve the relative image refs in Rule.Detail (WS9-030). Only image files under
// docs/images/ are served (.svg and .png); any other path, including the markdown itself, a
// directory, or a traversal attempt, is 404. Mount it under a prefix (the handler sees the
// prefix-stripped, relative path, e.g. "images/single-pin-net.svg"). SVG's content-type is set
// explicitly because Go's mime table resolves .svg only from the host's mime files, which CI/WASM
// may lack.
func RuleDocImageHandler() http.Handler {
	sub, err := fs.Sub(ruleDocs, "docs")
	if err != nil {
		panic(fmt.Sprintf("check: rule docs sub-FS: %v", err)) // embed path is a constant; unreachable
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
