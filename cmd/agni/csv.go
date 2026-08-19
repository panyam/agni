package main

import (
	"encoding/csv"
	"io"
	"strings"
)

// multiValueSep joins several values inside one cell. It is deliberately not a comma: a reader
// splitting a cell would then be unable to tell an intra-cell separator from the field delimiter
// the parser already consumed.
const multiValueSep = "|"

// csvFormulaPrefixes are the leading characters that make a spreadsheet treat a cell as a formula
// rather than text. Excel, LibreOffice and Sheets all do it, and they do it on open, before anyone
// clicks anything.
const csvFormulaPrefixes = "=+-@"

// sanitizeCell makes a value safe to write into a spreadsheet cell.
//
// A cell whose text begins with one of csvFormulaPrefixes is EXECUTED on open, so a net named
// "-VBUS" or a rule message beginning with "=" becomes a formula in the reader's sheet rather than
// the string we emitted. Prefixing a single quote is the convention every major spreadsheet
// understands as "this is text", and it is why escaping is unconditional rather than a flag: the
// values reaching these cells are net names and rule prose, neither of which we control.
//
// Leading tab, carriage return and newline are stripped first, because a cell beginning with
// whitespace ahead of a formula character is still parsed as a formula by some readers.
func sanitizeCell(s string) string {
	trimmed := strings.TrimLeft(s, "\t\r\n ")
	if trimmed == "" {
		return s
	}
	if strings.ContainsRune(csvFormulaPrefixes, rune(trimmed[0])) {
		return "'" + s
	}
	return s
}

// joinCell renders several values into one cell, sanitizing the result rather than each part, since
// only the leading character of the finished cell can start a formula.
func joinCell(vals []string) string {
	return sanitizeCell(strings.Join(vals, multiValueSep))
}

// csvWriter wraps encoding/csv with the two behaviours every table this package emits needs: cells
// are sanitized on the way out, and the first error is retained so a caller checks once at the end
// instead of after every row.
type csvWriter struct {
	w   *csv.Writer
	err error
}

func newCSVWriter(w io.Writer) *csvWriter { return &csvWriter{w: csv.NewWriter(w)} }

// header writes the column names verbatim. They are ours, not design data, so they skip
// sanitizing; running them through it would be harmless and would also imply they were untrusted.
func (c *csvWriter) header(cols []string) {
	if c.err == nil {
		c.err = c.w.Write(cols)
	}
}

// row writes one record, sanitizing every cell.
func (c *csvWriter) row(cells []string) {
	if c.err != nil {
		return
	}
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = sanitizeCell(cell)
	}
	c.err = c.w.Write(out)
}

// finish flushes and returns the first error seen, from any row or from the flush itself.
func (c *csvWriter) finish() error {
	c.w.Flush()
	if c.err != nil {
		return c.err
	}
	return c.w.Error()
}
