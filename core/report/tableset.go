package report

import (
	"fmt"
	"io"
)

// TableSet is the answers to a named set of queries asked of one design, rendered as one document
// (agni issue 729).
//
// Each section's table renders through the single-table renderer for its format, so a table in a set
// and the same query asked alone come out the same.
type TableSet struct {
	// Title names the set. Empty renders as "Query set".
	Title string
	// Source names the design every section ran against, once for the document.
	Source string
	// Preamble is the rules the queries share, shown once above the sections so each query's text can
	// be read against the relations it uses.
	Preamble string
	Sections []TableSection
}

// TableSection is one named query's answer, or why it has none.
type TableSection struct {
	Name        string
	Description string
	// Table is the answer. Its Title and Source are ignored, since the section's name is its heading
	// and the set carries the source.
	Table Table
	// Error, when set, replaces the table, so a failure never reads as a question that matched
	// nothing. Markdown and html still show Table.Query above it; the text renderer does not.
	Error string
}

func (s TableSet) title() string {
	if s.Title == "" {
		return "Query set"
	}
	return s.Title
}

// body is the section's table stripped of what the set renders once.
func (sec TableSection) body() Table {
	t := sec.Table
	t.Title, t.Source = "", ""
	return t
}

// TableSetText writes each section as a headed block in the aligned terminal format.
func TableSetText(w io.Writer, s TableSet) error {
	bw := &errWriter{w: w}
	bw.printf("%s\n", s.title())
	if s.Source != "" {
		bw.printf("%s\n", s.Source)
	}
	for _, sec := range s.Sections {
		bw.printf("\n== %s ==\n", sec.Name)
		if sec.Description != "" {
			bw.printf("%s\n", sec.Description)
		}
		if bw.err != nil {
			return bw.err
		}
		if sec.Error != "" {
			bw.printf("error: %s\n", sec.Error)
			continue
		}
		if err := TableText(w, sec.body()); err != nil {
			return err
		}
	}
	return bw.err
}

// TableSetMarkdown writes the set as one markdown document: a title, the shared preamble, then a
// section per query carrying its question above its answer.
func TableSetMarkdown(w io.Writer, s TableSet) error {
	bw := &errWriter{w: w}
	bw.printf("# %s\n\n", s.title())
	if s.Source != "" {
		bw.printf("*%s*\n\n", mdEscape(s.Source))
	}
	if s.Preamble != "" {
		bw.printf("Shared rules:\n\n```\n%s\n```\n\n", s.Preamble)
	}
	for _, sec := range s.Sections {
		bw.printf("## %s\n\n", sec.Name)
		if sec.Description != "" {
			bw.printf("%s\n\n", sec.Description)
		}
		if bw.err != nil {
			return bw.err
		}
		if sec.Error != "" {
			if sec.Table.Query != "" {
				bw.printf("```\n%s\n```\n\n", sec.Table.Query)
			}
			bw.printf("**Could not answer:** %s\n\n", mdEscape(sec.Error))
			continue
		}
		if err := TableMarkdown(w, sec.body()); err != nil {
			return err
		}
		bw.printf("\n")
	}
	return bw.err
}

// TableSetHTML writes the set as one self-contained page on the shared stylesheet.
func TableSetHTML(w io.Writer, s TableSet) error {
	tm, err := parse("tableset.html.tmpl")
	if err != nil {
		return err
	}
	v := tableSetView{Title: s.title(), Source: s.Source, Preamble: s.Preamble}
	for _, sec := range s.Sections {
		t := sec.body()
		v.Sections = append(v.Sections, tableSetSectionView{
			Name: sec.Name, Description: sec.Description, Error: sec.Error,
			Query: t.Query, Header: t.header(), Body: t.bodyRows(),
		})
	}
	if err := tm.Execute(w, v); err != nil {
		return fmt.Errorf("render query set: %w", err)
	}
	return nil
}

// tableSetView is the template's view, with every section's emitted header and rows precomputed so
// the template does no assembly.
type tableSetView struct {
	Title, Source, Preamble string
	Sections                []tableSetSectionView
}

type tableSetSectionView struct {
	Name, Description, Error, Query string
	Header                          []string
	Body                            [][]string
}
