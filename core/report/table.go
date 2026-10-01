package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Table is one query answer, ready to render in any format: the projected columns, the rows, and
// the question that produced them.
//
// It is the query result's report shape, as Report is a check run's and Checklist a review's, so one
// value renders as text, csv, markdown or html. It lives here rather than in cmd/ so the renderers
// are not copied and left to drift (agni issue 380).
type Table struct {
	// Title names the view, e.g. "Test point coverage". Empty for an ad-hoc query, the common CLI
	// case.
	Title string
	// Query is the datalog that produced the rows, carried so an exported view states the question
	// it answers and can be re-run.
	Query string
	// Bindings are the values the query's variables were bound to, each formatted `?n = "GND"`
	// (agni issue 793). A parameterized view prints them under the query, since the query text alone
	// no longer states the whole question.
	Bindings []string
	// Source names the design the query ran against. Rendered in the subtitle, not in a cell.
	Source string
	// Columns are the projected column names, in the query's own order. Provenance is NOT among
	// them; every renderer appends it as the last column.
	Columns []string
	Rows    []TableRow
}

// TableRow is one answer: a cell per column, plus the provenance of the facts that produced it.
type TableRow struct {
	Cells []string
	// Cites are the fact citations behind this row, rendered as one trailing column rather than as
	// a footnote.
	Cites []string
}

// ProvenanceColumn is the name of the trailing column every renderer appends. Exported so a caller
// binding to the csv header does not have to spell it a second time.
const ProvenanceColumn = "provenance"

// header returns the emitted column order: the projection, then provenance.
func (t Table) header() []string {
	return append(append(make([]string, 0, len(t.Columns)+1), t.Columns...), ProvenanceColumn)
}

// cells returns one row's emitted cells, provenance joined into the trailing one.
func (r TableRow) cells() []string {
	return append(append(make([]string, 0, len(r.Cells)+1), r.Cells...), strings.Join(r.Cites, " ; "))
}

// TableText writes the aligned terminal table: the projected columns plus provenance, then the
// count. Keep it byte-stable, since scripts parse `agni query` output.
func TableText(w io.Writer, t Table) error {
	if len(t.Rows) == 0 {
		_, err := fmt.Fprintln(w, "no results")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(t.header(), "\t"))
	for _, r := range t.Rows {
		fmt.Fprintln(tw, strings.Join(r.cells(), "\t"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\n%d result(s)\n", len(t.Rows))
	return err
}

// TableCSV writes the rows as csv, header first, with every cell sanitized against spreadsheet
// formula execution (see SanitizeCell).
//
// It emits THE TABLE AND NOTHING ELSE: no title, no query, no count, because whatever binds to a
// csv expects the header as its first row. A commented preamble is not portable across readers.
//
// An empty result still writes the header, so it reads as a query that matched nothing rather than
// a run that failed.
func TableCSV(w io.Writer, t Table) error {
	c := NewCSVWriter(w)
	c.Header(t.header())
	for _, r := range t.Rows {
		c.Row(r.cells())
	}
	return c.Finish()
}

// TableMarkdown writes the view as a GitHub-flavoured markdown section: a heading, the query in a
// fence, then the table.
//
// The empty case gets a SENTENCE, not an empty table, since a header with no rows reads as a broken
// generator.
func TableMarkdown(w io.Writer, t Table) error {
	bw := &errWriter{w: w}
	if t.Title != "" {
		bw.printf("## %s\n\n", t.Title)
	}
	if t.Source != "" {
		bw.printf("*%s*\n\n", mdEscape(t.Source))
	}
	writeQuery(bw, t.Query, t.Bindings)
	if len(t.Rows) == 0 {
		bw.printf("No rows matched.\n")
		return bw.err
	}
	hdr := t.header()
	bw.printf("| %s |\n", strings.Join(mdEscapeAll(hdr), " | "))
	bw.printf("|%s\n", strings.Repeat(" --- |", len(hdr)))
	for _, r := range t.Rows {
		bw.printf("| %s |\n", strings.Join(mdEscapeAll(r.cells()), " | "))
	}
	bw.printf("\n%d result(s)\n", len(t.Rows))
	return bw.err
}

// mdEscape makes a cell safe inside a markdown table row. A pipe would end the cell early, and a
// newline would end the ROW, shifting every following cell one column left. Net names can carry
// either.
func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\r", " ")
}

func mdEscapeAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = mdEscape(s)
	}
	return out
}

// TableHTML writes the view as one self-contained page on the stylesheet the check report and the
// checklist share. html/template for the reason stated on HTML.
func TableHTML(w io.Writer, t Table) error {
	tm, err := parse("table.html.tmpl")
	if err != nil {
		return err
	}
	return tm.Execute(w, tableView{Table: t, Header: t.header(), Body: t.bodyRows()})
}

// tableView is the template's view of a Table, with the emitted header and rows precomputed. The
// template does no assembly, so what it renders and what the csv writes cannot diverge.
type tableView struct {
	Table
	Header []string
	Body   [][]string
}

func (t Table) bodyRows() [][]string {
	out := make([][]string, 0, len(t.Rows))
	for _, r := range t.Rows {
		out = append(out, r.cells())
	}
	return out
}

// errWriter retains the first write error so a renderer can print a document without checking after
// every line, like CSVWriter.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// writeQuery prints a query in a fence and, under it, the values its variables were bound to.
func writeQuery(bw *errWriter, query string, bindings []string) {
	if query != "" {
		bw.printf("```\n%s\n```\n\n", query)
	}
	if len(bindings) > 0 {
		quoted := make([]string, len(bindings))
		for i, b := range bindings {
			quoted[i] = "`" + b + "`"
		}
		bw.printf("Bound: %s\n\n", strings.Join(quoted, ", "))
	}
}
