package report

import (
	"bytes"
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
	// Total is how many rows the answer had before a limit cut it to Rows (agni issue 896), zero when
	// nothing was cut. The count line then says how many were left out, so a page is never read as the
	// whole answer.
	Total int
}

// Count is the line stating how many rows the view holds, and how many the answer had when a limit
// left some out.
func (t Table) Count() string {
	if t.Total > len(t.Rows) {
		return fmt.Sprintf("%d of %d result(s), %d left out by the limit", len(t.Rows), t.Total, t.Total-len(t.Rows))
	}
	return fmt.Sprintf("%d result(s)", len(t.Rows))
}

// TableRow is one answer: a cell per column, plus the provenance of the facts that produced it.
type TableRow struct {
	Cells []string
	// Cites are the fact citations behind this row, rendered as one trailing column rather than as
	// a footnote.
	Cites []string
	// Why aligns with Cells and says why an empty cell is absent, such as a sum over numbers in
	// different units. A renderer prints the reason in the cell, so an absent answer is never a blank.
	Why []string
	// Witnesses say how the row holds, when the query asked for them (agni issue 794). text and html
	// print the tree under the row, markdown in a section after the table, and csv never, since a
	// csv holds the table alone.
	Witnesses []Witness
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
	out := append(make([]string, 0, len(r.Cells)+1), r.Cells...)
	for i, why := range r.Why {
		if why != "" && i < len(out) && out[i] == "" {
			out[i] = "absent: " + why
		}
	}
	return append(out, strings.Join(r.Cites, " ; "))
}

// TableText writes the aligned terminal table: the projected columns plus provenance, then the
// count. Keep it byte-stable, since scripts parse `agni query` output.
func TableText(w io.Writer, t Table) error {
	if len(t.Rows) == 0 {
		_, err := fmt.Fprintln(w, "no results")
		return err
	}
	// The rows align through a buffer, so a row's witness tree can go under it without a line that
	// holds no cells breaking the alignment of the rows after it.
	var aligned bytes.Buffer
	tw := tabwriter.NewWriter(&aligned, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(t.header(), "\t"))
	for _, r := range t.Rows {
		fmt.Fprintln(tw, strings.Join(r.cells(), "\t"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	bw := &errWriter{w: w}
	lines := strings.SplitAfter(aligned.String(), "\n")
	// A cell holding a newline spans two lines, so a tree goes under its row only when every row is
	// one line, and otherwise after the table, numbered by row.
	if !t.witnessed() || len(lines) != len(t.Rows)+2 {
		bw.printf("%s", aligned.String())
		lines = nil
		for i, r := range t.Rows {
			if len(r.Witnesses) > 0 {
				bw.printf("\nrow %d:\n", i+1)
			}
			for _, l := range WitnessLines(r.Witnesses) {
				bw.printf("%s%s\n", witnessIndent+witnessIndent, l)
			}
		}
	}
	for i := range lines {
		bw.printf("%s", lines[i])
		if i >= 1 && i <= len(t.Rows) {
			for _, l := range WitnessLines(t.Rows[i-1].Witnesses) {
				bw.printf("%s%s\n", witnessIndent+witnessIndent, l)
			}
		}
	}
	bw.printf("\n%s\n", t.Count())
	return bw.err
}

// witnessed reports whether any row carries witnesses.
func (t Table) witnessed() bool {
	for _, r := range t.Rows {
		if len(r.Witnesses) > 0 {
			return true
		}
	}
	return false
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
		r.Why = nil // a csv cell travels raw, so an absent one stays empty
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
	bw.printf("\n%s\n", t.Count())
	// A markdown table has no row that spans it, so the trees follow the table, one per row, under
	// the row's cells.
	if t.witnessed() {
		bw.printf("\n**How each row holds**\n")
		for i, r := range t.Rows {
			if len(r.Witnesses) == 0 {
				continue
			}
			bw.printf("\nRow %d, %s:\n\n```\n%s\n```\n", i+1, mdEscape(strings.Join(r.Cells, ", ")), strings.Join(WitnessLines(r.Witnesses), "\n"))
		}
	}
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
	Body   []bodyRow
}

// bodyRow is one emitted row and, when it has witnesses, their tree as text, spanning the row.
type bodyRow struct {
	Cells []string
	How   string
	Span  int
}

func (t Table) bodyRows() []bodyRow {
	out := make([]bodyRow, 0, len(t.Rows))
	for _, r := range t.Rows {
		c := r.cells()
		out = append(out, bodyRow{Cells: c, How: strings.Join(WitnessLines(r.Witnesses), "\n"), Span: len(c)})
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
