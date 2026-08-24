package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Explainable inlines a glossary term as a hoverable link, so a page can USE a term without
// re-teaching it.
//
// The problem it solves is repetition with a short half-life. A chapter explains what a differential
// pair is, and three pages later the next chapter has to explain it again, because a reader arriving
// there does not carry the first explanation with them. Writing the gloss every time is what makes
// the prose long; writing it once and linking is what makes a reader bounce out of the page.
//
// So the gloss lives in ONE place, `content/reference/terms/<id>.md`, and every use site is a tag:
//
//	CAN needs a {{ explainable "differential-pair" }}, {{ explainable "termination" }}, and a
//	{{ explainable "transceiver" }} for anything leaving the board.
//
// The term files are REAL pages rather than a data blob, which is what makes the fallbacks work. The
// tag renders an ordinary anchor carrying the one-line summary in `title`, so with no JavaScript a
// reader still gets the summary on hover and a click still lands on the full page. terms.js upgrades
// that to an inline popover carrying the whole term page, diagram included. A term is therefore
// deep-linkable, indexed by pagefind, and has somewhere for a diagram to live at full size, none of
// which a JSON glossary would have given.
//
// The convention that goes with it: a `learn/` chapter still teaches a term in full the first time it
// introduces it, and every later mention anywhere in the docsite is a tag. The course keeps its
// teaching arc and the repetition goes.
//
// Adding a term is ONE file plus one line in the glossary index. It needs no nav wiring, because
// `nav_test.go`'s reachability check reads files directly under a section and skips subdirectories,
// the same reason the generated rule and relation catalogs need none. `terms_test.go` enforces the
// rest.
func Explainable(id string, label ...string) string {
	return renderTerm(id, label, false)
}

// ExplainableCap is Explainable with the label's first letter upper-cased, for a term that opens a
// sentence. It is a separate function rather than a heuristic because no rule can tell "the sentence
// started" from "this term is a proper noun": a term that is ALWAYS capitalised (EDIF, ERC) sets its
// `label` that way and both functions then agree, with nothing to detect.
func ExplainableCap(id string) string {
	return renderTerm(id, nil, true)
}

// Term is one glossary entry, as declared in its page's frontmatter.
type Term struct {
	ID string `yaml:"-"`
	// Title is the term page's own heading.
	Title string `yaml:"title"`
	// Label is the mid-sentence form the tag inlines, normally lower case.
	Label string `yaml:"label"`
	// Summary is the one-liner shown on hover with no JavaScript, and as the popover's lead.
	Summary string `yaml:"summary"`
	// Level is the EE1-EE7 tier the term operates at, the same vocabulary learn/levels.md uses.
	Level string `yaml:"level"`
}

const termsDir = "content/reference/terms"

var (
	termsOnce  sync.Once
	termsByID  map[string]*Term
	termsError error
)

// LoadTerms reads every term page's frontmatter once per build.
func LoadTerms() (map[string]*Term, error) {
	termsOnce.Do(func() {
		termsByID, termsError = loadTerms()
	})
	return termsByID, termsError
}

func loadTerms() (map[string]*Term, error) {
	dir := filepath.Join(projectRoot, termsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", termsDir, err)
	}
	out := map[string]*Term{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		if id == "index" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading term %s: %w", id, err)
		}
		t := &Term{ID: id}
		if err := yaml.Unmarshal([]byte(frontmatterOf(string(data))), t); err != nil {
			return nil, fmt.Errorf("term %s frontmatter: %w", id, err)
		}
		out[id] = t
	}
	return out, nil
}

// frontmatterOf returns the leading YAML block's body, or "" when there is none.
func frontmatterOf(s string) string {
	s = strings.TrimLeft(s, "\n")
	if !strings.HasPrefix(s, "---\n") {
		return ""
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return ""
	}
	return s[4 : 4+end]
}

// TermIDs returns every declared term id, sorted. Used by the glossary index test.
func TermIDs() []string {
	terms, err := LoadTerms()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(terms))
	for id := range terms {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// TermURL is the page a term id resolves to.
func TermURL(id string) string {
	return PathPrefix + "/reference/terms/" + id + "/"
}

// renderTerm builds the inline anchor.
//
// A missing term renders LOUDLY rather than silently degrading to plain text, for the same reason
// AgniRun renders its own failures: a tag that quietly becomes ordinary prose is indistinguishable
// from prose someone wrote, so the typo survives every review. terms_test.go fails the build on it,
// and this is what a preview shows in the meantime.
func renderTerm(id string, label []string, capitalize bool) string {
	terms, err := LoadTerms()
	if err != nil {
		return "`[terms unavailable: " + html.EscapeString(err.Error()) + "]`"
	}
	t, ok := terms[id]
	if !ok {
		return "`[unknown term: " + html.EscapeString(id) + "]`"
	}

	text := t.Label
	if len(label) > 0 && strings.TrimSpace(label[0]) != "" {
		text = label[0]
	}
	if capitalize {
		text = upperFirst(text)
	}

	return `<a class="xterm" href="` + TermURL(id) +
		`" data-term="` + html.EscapeString(id) +
		`" title="` + html.EscapeString(t.Summary) + `">` +
		html.EscapeString(text) + `</a>`
}

// upperFirst upper-cases the first rune, leaving the rest alone so an embedded acronym survives
// ("i2c bus" must not become "I2c bus", so a term like that declares its own label).
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
