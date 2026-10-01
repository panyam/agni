package main

import (
	"flag"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	s3 "github.com/panyam/s3gen"
)

var (
	addr  = flag.String("addr", DefaultAddress(), "Address where the http server is running")
	build = flag.Bool("build", false, "Build the site once and quit, instead of serving it")
)

// projectRoot is the absolute path to the docsite directory.
var projectRoot string

func init() {
	var err error
	projectRoot, err = filepath.Abs(".")
	if err != nil {
		log.Fatalf("Failed to get project root: %v", err)
	}
}

// IncludeFile returns a file's contents as raw (unescaped) HTML. The path is
// relative to the docsite directory and cannot escape it.
func IncludeFile(relativePath string) template.HTML {
	absPath, ok := safeJoin(relativePath)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	return template.HTML(data)
}

// IncludeFileText returns a file's contents as plain (escaped) text. Useful for
// showing source snippets.
func IncludeFileText(relativePath string) string {
	absPath, ok := safeJoin(relativePath)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// IncludeCard returns a GENERATED catalog card's markdown body, frontmatter stripped, for
// transcluding a rule or relation reference into the guide page that teaches it, so the card keeps
// ONE source in `make catalog-docs` (WS14-006). The card's own frontmatter would render as a stray
// heading in the host page. Cards open at h3 so they nest under the host page's h2.
func IncludeCard(relativePath string) string {
	absPath, ok := safeJoin(relativePath)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		log.Printf("includeCard: %v", err)
		return ""
	}
	return resolveSiteRefs(stripFrontmatter(string(data)))
}

// resolveSiteRefs substitutes the site directives a GENERATED card carries, because a transcluded
// card's return value is spliced in after the host page's template pass and nothing re-scans it
// (#449). Without this, `{{.Site.PathPrefix}}` in an image path reached the browser verbatim and
// 404'd. Substituting the const avoids re-running the template engine, and its recursion risk, inside
// a host render. See docsite/README.md for the transclusion trap.
func resolveSiteRefs(s string) string {
	for _, form := range []string{"{{.Site.PathPrefix}}", "{{ .Site.PathPrefix }}"} {
		s = strings.ReplaceAll(s, form, PathPrefix)
	}
	return s
}

// stripFrontmatter removes a leading YAML frontmatter block. A file without one is returned
// unchanged, so this is safe on a hand-written include as well as a generated card.
func stripFrontmatter(s string) string {
	s = strings.TrimLeft(s, "\n")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	if end := strings.Index(s[4:], "\n---"); end >= 0 {
		rest := s[4+end+4:]
		return strings.TrimLeft(rest, "\n")
	}
	return s // unterminated frontmatter is left alone rather than eating the file
}

// safeJoin resolves a project-relative path and rejects traversal.
func safeJoin(relativePath string) (string, bool) {
	cleanPath := filepath.Clean(relativePath)
	if filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "..") {
		log.Printf("include: rejected path %q (absolute or escaping)", relativePath)
		return "", false
	}
	fullPath := filepath.Join(projectRoot, cleanPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil || !strings.HasPrefix(absPath, projectRoot) {
		log.Printf("include: rejected path %q (escapes project root)", relativePath)
		return "", false
	}
	return absPath, true
}

// PathPrefix is the URL prefix every page and asset is served under. A const rather than a read of
// Site, because Explainable needs it and Site's literal references Explainable, which Go rejects as
// an initialization cycle.
const PathPrefix = "/agni"

// Site is the s3gen configuration for the Agni documentation site.
var Site = &s3.Site{
	OutputDir:   "./dist",
	ContentRoot: "./content",

	// GitHub Pages serves the site at https://panyam.github.io/agni/.
	PathPrefix: PathPrefix,

	TemplateFolders: []string{
		"./templates",
	},

	StaticFolders: []string{
		"/static/",
		"./static",
	},

	DefaultBaseTemplate: s3.BaseTemplate{
		Name: "BasePage.html",
		Params: map[any]any{
			"BodyTemplateName": "Content",
		},
	},

	CommonFuncMap: map[string]any{
		"includeFile":     IncludeFile,
		"includeFileText": IncludeFileText,
		"includeCard":     IncludeCard,
		"agniRun":         AgniRun,
		"explainable":     Explainable,
		"explainableCap":  ExplainableCap,

		// Helpers newer s3gen supplies in its default func map and the pinned version does not.
		"Contains":      strings.Contains,
		"HasPrefix":     strings.HasPrefix,
		"HasSuffix":     strings.HasSuffix,
		"BytesToString": func(b []byte) string { return string(b) },
		"HTML":          func(s string) template.HTML { return template.HTML(s) },
	},
}

func main() {
	flag.Parse()

	// Build once, then serve. No file watcher, so `-build` terminates as a build STEP must.
	Site.Rebuild(nil)

	if !*build {
		Site.Serve(*addr)
	}
}

func DefaultAddress() string {
	if a := os.Getenv("AGNI_DOCS_PORT"); a != "" {
		return a
	}
	return ":8080"
}
