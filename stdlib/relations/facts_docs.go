package relations

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// relationDocs embeds the per-relation reference markdown under facts/docs/ (WS14-005), one file
// per query relation plus the images it references, parallel to the per-rule docs that ruleDoc
// serves. The one-line Summary stays in query.Catalog, which ListRelations carries to both the CLI
// and the web panel; this is the longer Detail behind it. Embedding keeps the core free of file I/O
// (C1).
//
// facts_docs_test.go couples docs and relations in both directions, so every doc must name a
// registered relation and a `doc: facts/docs/<name>.md` back-link on a Rel* const must resolve.
//
// The image glob is SVG-only because Go's embed errors on a glob that matches nothing. Adding the
// first raster card means adding `facts/docs/images/*.png` here; the handler already serves .png.
//
//go:embed facts/docs/*.md facts/docs/images/*.svg
var relationDocs embed.FS

// RelationDoc returns the embedded reference markdown for a query relation, or "" if it has none.
// Unlike ruleDoc, a missing doc does not panic. Every built-in EDB relation has one
// (facts_docs_test.go requires it), but a computed predicate may not, so callers treat "" as "fall
// back to the catalog Summary".
func RelationDoc(name string) string {
	b, err := relationDocs.ReadFile("facts/docs/" + name + ".md")
	if err != nil {
		return ""
	}
	return string(b)
}

// RelationDocImageHandler serves the relation docs' embedded images (.svg and .png under
// facts/docs/images/) so the Query panel can resolve a Detail's relative image refs, the same way
// RuleDocImageHandler does for rules. Any other path, including the markdown, a directory or a
// traversal attempt, is 404. Mount it under a prefix; the handler sees the prefix-stripped path
// (e.g. "images/net.bus_like.svg"). The SVG content-type is set explicitly because Go's mime table
// resolves .svg only from the host's mime files, which CI and WASM may lack.
func RelationDocImageHandler() http.Handler {
	sub, err := fs.Sub(relationDocs, "facts/docs")
	if err != nil {
		panic("check: relation docs sub-FS: " + err.Error()) // embed path is a constant; unreachable
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
