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

// Site is the s3gen configuration for the Agni documentation site.
var Site = &s3.Site{
	OutputDir:   "./dist",
	ContentRoot: "./content",

	// URL path prefix. The GitHub Pages site is served at
	// https://panyam.github.io/agni/, so pages live under /agni.
	PathPrefix: "/agni",

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

		// String/content helpers the templates use. Newer s3gen provides
		// these via its default func map; the pinned version does not, so we
		// supply them here to keep the site self-contained.
		"Contains":      strings.Contains,
		"HasPrefix":     strings.HasPrefix,
		"HasSuffix":     strings.HasSuffix,
		"BytesToString": func(b []byte) string { return string(b) },
		"HTML":          func(s string) template.HTML { return template.HTML(s) },
	},
}

func main() {
	flag.Parse()

	if *build || os.Getenv("AGNI_DOCS_ENV") != "production" {
		Site.Rebuild(nil)
		Site.Watch()
	}

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
