package relations

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// relationDocs embeds the per-relation reference documentation (WS14-005): one markdown file
// (plus any referenced images) per query relation under facts/docs/, parallel to the per-rule
// docs/ that ruleDoc serves. A relation's one-line Summary stays in query.Catalog (it rides
// ListRelations to BOTH the CLI and the web panel — the "one source, two surfaces" invariant);
// this is the richer Detail behind that summary, authored as markdown so the Go+datalog+HW/SW
// framing and the SVG schematic cards live beside the prose, not in Go strings. Embedding keeps
// the core free of file I/O (C1) and the single-binary/WASM stories intact, exactly as ruleDocs
// does for rules.
//
// The coupling between a doc file and its relation is harness-enforced in both directions
// (facts_docs_test.go): every doc names a registered relation, and a relation that declares a
// `doc: facts/docs/<name>.md` back-link on its Rel* const must resolve to a present file.
//
// The image glob is SVG-only until a PNG lands (Go's embed errors on a zero-match glob); a
// backfill that adds a raster card re-adds `facts/docs/images/*.png` here — a visible edit. The
// handler already serves .png from the FS, so only the embed set gates which files exist.
//go:embed facts/docs/*.md facts/docs/images/*.svg
var relationDocs embed.FS

// RelationDoc returns the embedded reference markdown for a query relation, or "" if the relation
// has no doc yet. Unlike ruleDoc (which panics on a missing built-in rule doc), a missing relation
// doc is a NORMAL state during the WS14-005 staged backfill: the two exemplars ship first and the
// remaining relations are documented in a follow-up, so callers treat "" as "no deep-dive yet" and
// fall back to the catalog Summary. The require-all flip that makes an undocumented relation a CI
// failure is a later stage, enforced in the harness, not here.
func RelationDoc(name string) string {
	b, err := relationDocs.ReadFile("facts/docs/" + name + ".md")
	if err != nil {
		return ""
	}
	return string(b)
}

// RelationDocImageHandler serves the relation docs' embedded schematic-card images (the SVG/PNG a
// facts/docs/<relation>.md references) as a read-only static route, so a future Query panel can
// resolve the relative image refs in a relation's Detail the same way RuleDocImageHandler does for
// rules (WS9 follow-up). Only image files under facts/docs/images/ are served (.svg and .png); any
// other path (the markdown, a directory, a traversal attempt) is 404, so the handler never exposes
// anything but the diagrams. The images come from the embed FS alone — no filesystem access (C1).
// Mount it under a prefix; the handler sees the prefix-stripped relative path (e.g.
// "images/net.bus_like.svg"). SVG's content-type is set explicitly because Go's mime table resolves
// .svg only from the host's mime files, which CI/WASM may lack.
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
