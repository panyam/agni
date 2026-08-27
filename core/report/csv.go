package report

import (
	"encoding/csv"
	"io"
	"strings"
)

// MultiValueSep joins several values inside one cell. It is deliberately not a comma: a reader
// splitting a cell would then be unable to tell an intra-cell separator from the field delimiter
// the parser already consumed.
const MultiValueSep = "|"

// csvFormulaPrefixes are the leading characters that make a spreadsheet treat a cell as a formula
// rather than text. Excel, LibreOffice and Sheets all do it, and they do it on open, before anyone
// clicks anything.
const csvFormulaPrefixes = "=+-@"

// SanitizeCell makes a value safe to write into a spreadsheet cell.
//
// A cell whose text begins with one of csvFormulaPrefixes is EXECUTED on open, so a net named
// "-VBUS" or a rule message beginning with "=" becomes a formula in the reader's sheet rather than
// the string we emitted. Prefixing a single quote is the convention every major spreadsheet
// understands as "this is text", and it is why escaping is unconditional rather than a flag: the
// values reaching these cells are net names and rule prose, neither of which we control.
//
// Leading tab, carriage return and newline are stripped first, because a cell beginning with
// whitespace ahead of a formula character is still parsed as a formula by some readers.
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

// CSVWriter wraps encoding/csv with the two behaviours every table this repo emits needs: cells
// are sanitized on the way out, and the first error is retained so a caller checks once at the end
// instead of after every row.
//
// It lives here rather than in cmd/ because three CLI renderers and the query table all need the
// same escaping, and a second copy is how the check and diff renderers drifted (agni issue 380).
type CSVWriter struct {
	w   *csv.Writer
	err error
}

func NewCSVWriter(w io.Writer) *CSVWriter { return &CSVWriter{w: csv.NewWriter(w)} }

// Header writes the column names verbatim. They are ours, not design data, so they skip
// sanitizing; running them through it would be harmless and would also imply they were untrusted.
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
