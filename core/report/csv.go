package report

import (
	"encoding/csv"
	"io"
	"strings"
)

// MultiValueSep joins several values inside one cell. Not a comma, so a reader splitting a cell can
// tell it from the field delimiter.
const MultiValueSep = "|"

// csvFormulaPrefixes are the leading characters that make a spreadsheet treat a cell as a formula
// rather than text. Excel, LibreOffice and Sheets all do it on open.
const csvFormulaPrefixes = "=+-@"

// SanitizeCell makes a value safe to write into a spreadsheet cell.
//
// A cell whose text begins with one of csvFormulaPrefixes is EXECUTED on open, so a net named
// "-VBUS" becomes a formula. A leading single quote marks it as text in every major spreadsheet.
// Escaping is unconditional because the values are net names and rule prose, which we do not control.
//
// Leading whitespace is skipped when looking for the prefix, since some readers still parse
// whitespace ahead of a formula character as a formula.
func SanitizeCell(s string) string {
	trimmed := strings.TrimLeft(s, "\t\r\n ")
	if trimmed == "" {
		return s
	}
	if strings.ContainsRune(csvFormulaPrefixes, rune(trimmed[0])) {
		return "'" + s
	}
	return s
}

// JoinCell renders several values into one cell, sanitizing the result rather than each part, since
// only the leading character of the finished cell can start a formula.
func JoinCell(vals []string) string {
	return SanitizeCell(strings.Join(vals, MultiValueSep))
}

// CSVWriter wraps encoding/csv so cells are sanitized on the way out and the first error is
// retained, letting a caller check once in Finish instead of after every row.
//
// It lives here rather than in cmd/ so the CLI renderers and the query table share one escaping
// (agni issue 380).
type CSVWriter struct {
	w   *csv.Writer
	err error
}

func NewCSVWriter(w io.Writer) *CSVWriter { return &CSVWriter{w: csv.NewWriter(w)} }

// Header writes the column names verbatim. They are ours, not design data, so they skip sanitizing.
func (c *CSVWriter) Header(cols []string) {
	if c.err == nil {
		c.err = c.w.Write(cols)
	}
}

// Row writes one record, sanitizing every cell.
func (c *CSVWriter) Row(cells []string) {
	if c.err != nil {
		return
	}
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = SanitizeCell(cell)
	}
	c.err = c.w.Write(out)
}

// Finish flushes and returns the first error seen, from any row or from the flush itself.
func (c *CSVWriter) Finish() error {
	c.w.Flush()
	if c.err != nil {
		return c.err
	}
	return c.w.Error()
}
