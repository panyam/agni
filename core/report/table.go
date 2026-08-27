package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Table is one query answer, ready to render in any format: the projected columns, the rows, and
// the question that produced them.
//
// It exists because a query result is the one artifact in this repo with no report shape. A check
// run has Report, a checklist has Checklist, and a query printed an aligned table straight out of
// cmd/ with no way to get csv, markdown or html out of it at all. Those are the formats a view
// leaves the tool in, so the type is the view rather than a rendering detail.
//
// It lives here rather than in cmd/ for the reason agni issue 380 records: the check and diff
// renderers were written in cmd/, the second was copied from the first, and the copies drifted. A
// third table renderer in the same place would be the same mistake a third time.
type Table struct {
	// Title names the view, e.g. "Test point coverage". Empty for an ad-hoc query, which is the
	// common CLI case and renders with the query as its own heading.
	Title string
	// Query is the datalog that produced the rows, carried so an exported view states the question
	// it answers. A view whose question is missing cannot be re-run, checked, or argued with, which
	// is most of what separates a view from a screenshot.
	Query string
	// Source names the design the query ran against. Rendered in the subtitle, not in a cell.
	Source string
	// Columns are the projected column names, in the query's own order. Provenance is NOT among
	// them; every renderer appends it, so the two cannot disagree about where it goes.
	Columns []string
	Rows    []TableRow
}

// TableRow is one answer: a cell per column, plus the provenance of the facts that produced it.
type TableRow struct {
	Cells []string
	// Cites are the fact citations behind this row. Rendered as one trailing column rather than as
	// a footnote, because a citation a reader has to go and find is one they will not check.
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
// count. This is the format `agni query` has always printed, kept byte-identical so adding the
// others changed nothing for anyone already parsing it.
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
// It emits THE TABLE AND NOTHING ELSE: no title, no query, no count. The other formats carry that
// preamble and this one deliberately does not, because a csv is read by a machine or bound to by a
// spreadsheet whose first row must be the header. A commented preamble is a convention some readers
// honour and others hand you as a row of garbage.
//
// An empty result still writes the header. A zero-row csv with columns is a table that ran and
// matched nothing; an empty FILE is indistinguishable from a run that failed.
func TableCSV(w io.Writer, t Table) error {
	c := NewCSVWriter(w)
	c.Header(t.header())
	for _, r := range t.Rows {
		c.Row(r.cells())
	}
	return c.Finish()
}

// tableJSON is the wire shape of TableJSON, named separately so the field tags are visible at the
// point the format is defined rather than inferred from a struct built for rendering.
type tableJSON struct {
	Title   string         `json:"title,omitempty"`
	Query   string         `json:"query,omitempty"`
	Source  string         `json:"source,omitempty"`
	Columns []string       `json:"columns"`
	Rows    []tableRowJSON `json:"rows"`
	Count   int            `json:"count"`
}

type tableRowJSON struct {
	Cells []string `json:"cells"`
	Cites []string `json:"cites,omitempty"`
}

// TableJSON writes the view as one json object for tooling. Provenance stays a per-row LIST here
// rather than being flattened into a trailing cell the way the text and csv forms need: a consumer
// that wants the citations wants them apart, and json is the one format with somewhere to put them.
func TableJSON(w io.Writer, t Table) error {
	out := tableJSON{Title: t.Title, Query: t.Query, Source: t.Source, Columns: t.Columns, Count: len(t.Rows)}
	out.Rows = make([]tableRowJSON, 0, len(t.Rows))
	for _, r := range t.Rows {
		out.Rows = append(out.Rows, tableRowJSON{Cells: r.Cells, Cites: r.Cites})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// TableMarkdown writes the view as a GitHub-flavoured markdown section: a heading, the query in a
// fence, then the table.
//
// The empty case gets a SENTENCE, not an empty table. A markdown table with a header and no rows
// reads as an omission or a broken generator, and a view that matched nothing has to say so in
// words for the same reason a check that did not run may not render as a pass.
func TableMarkdown(w io.Writer, t Table) error {
	bw := &errWriter{w: w}
	if t.Title != "" {
		bw.printf("## %s\n\n", t.Title)
	}
	if t.Source != "" {
		bw.printf("*%s*\n\n", mdEscape(t.Source))
	}
	if t.Query != "" {
		bw.printf("```\n%s\n```\n\n", t.Query)
	}
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
// newline would end the ROW, silently shifting every following cell one column left. Both occur in
// real data: provenance joins several citations, and a net name is whatever the design called it.
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

// TableHTML writes the view as one self-contained page, sharing the stylesheet the check report and
// the checklist use, so three artifacts out of one tool look like one tool.
//
// html/template rather than string building, for the reason stated on HTML: every cell here came out
// of a design file this engine did not author.
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
// every line. Same posture as CSVWriter, for the same reason.
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
